package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"vcode/internal/agent"
	"vcode/internal/config"
	"vcode/internal/event"
	"vcode/internal/taskgraph"
	"vcode/internal/verify"
	"vcode/internal/worktree"
)

func stderr() *os.File { return os.Stderr }

func taskCommand(args []string) int {
	root := mustCurrentDir()
	store := taskgraph.NewStore(root)
	global := taskgraph.NewIndex(config.VcodeHomeDir())
	if len(args) == 0 || args[0] == "list" {
		return listTasks(store)
	}
	switch args[0] {
	case "global":
		return listGlobalTasks(global)
	case "show":
		if len(args) < 2 {
			fmt.Fprintln(stderr(), "usage: vcode task show <task-id>")
			return 2
		}
		return showTask(store, args[1], len(args) > 2 && args[2] == "--json")
	case "logs":
		if len(args) < 2 {
			fmt.Fprintln(stderr(), "usage: vcode task logs <task-id>")
			return 2
		}
		return showTaskLogs(store, args[1])
	case "create":
		goal := strings.TrimSpace(strings.Join(args[1:], " "))
		if goal == "" {
			fmt.Fprintln(stderr(), "usage: vcode task create <goal>")
			return 2
		}
		t, err := store.Create(goal, root, nil)
		if err != nil {
			fmt.Fprintln(stderr(), "error:", err)
			return 1
		}
		fmt.Printf("created task %s\n", t.ID)
		_ = global.Upsert(t)
		return 0
	case "plan":
		if len(args) < 2 {
			fmt.Fprintln(stderr(), "usage: vcode task plan <task-id>")
			return 2
		}
		return planTask(store, global, args[1])
	case "approve":
		if len(args) < 2 {
			fmt.Fprintln(stderr(), "usage: vcode task approve <task-id>")
			return 2
		}
		return approveTask(store, global, args[1])
	case "resume", "retry", "pause", "cancel":
		if len(args) < 2 {
			fmt.Fprintln(stderr(), "usage: vcode task resume|retry|pause|cancel <task-id> [node-id]")
			return 2
		}
		return changeTaskState(store, global, args[0], args[1], args[2:])
	case "run":
		if len(args) < 2 {
			fmt.Fprintln(stderr(), "usage: vcode task run <task-id>")
			return 2
		}
		return runTaskGraph(store, args[1])
	case "merge", "integrate":
		if len(args) < 2 {
			fmt.Fprintln(stderr(), "usage: vcode task merge <task-id> [node-id]")
			return 2
		}
		return mergeTaskNode(store, global, args[1], args[2:])
	default:
		fmt.Fprintln(stderr(), "usage: vcode task [list|show|create|plan|approve|resume|retry|pause|cancel|run|merge]")
		return 2
	}
}

func runTaskGraph(store *taskgraph.Store, id string) int {
	task, err := store.Get(id)
	if err != nil {
		fmt.Fprintln(stderr(), "error:", err)
		return 1
	}
	if len(task.Nodes) == 0 {
		fmt.Fprintln(stderr(), "error: task has no nodes; create a plan before running it")
		return 1
	}
	if task.Status == taskgraph.Blocked {
		fmt.Fprintln(stderr(), "task is awaiting approval; run `vcode task approve", id+"` first")
		return 1
	}
	ctx := context.Background()
	var sink event.Sink = agent.NewTextSink(os.Stdout, nil, 100)
	if cfg, loadErr := config.Load(); loadErr == nil {
		sink = withNotifications(sink, cfg)
	}
	scheduler := taskgraph.Scheduler{Store: store, MaxParallel: 4, DefaultRetry: 2, OnEvent: func(e taskgraph.Event) {
		level := event.LevelInfo
		if e.Type == "node_failed" || e.Type == "node_retrying" {
			level = event.LevelWarn
		}
		sink.Emit(event.Event{Kind: event.Phase, Text: fmt.Sprintf("task %s · %s · %s", e.NodeID, e.Type, e.Message), Level: level})
	}}
	worktrees := worktree.NewManager(task.ProjectRoot)
	err = scheduler.Run(ctx, &task, func(ctx context.Context, node taskgraph.Node) taskgraph.NodeResult {
		role := string(node.Role)
		if role == "" {
			role = string(taskgraph.Build)
		}
		prompt := fmt.Sprintf("You are the %s role in a durable Vcode task.\nTask goal: %s\nNode: %s\n\n%s\n\nReturn changed files, commands, verification evidence, blockers, and next action.", role, task.Goal, node.Title, node.Prompt)
		workspace := node.Workspace
		if node.Role == taskgraph.Build {
			created, createErr := worktrees.Create(ctx, task.ID, node.ID)
			if createErr != nil {
				return taskgraph.NodeResult{Err: createErr}
			}
			workspace = created
		}
		if workspace == "" {
			workspace = task.ProjectRoot
		}
		ctrl, setupErr := setupWithWorkspaceRole(ctx, node.Model, node.MaxAttempts, true, sink, workspace, role)
		if setupErr != nil {
			return taskgraph.NodeResult{Err: setupErr}
		}
		defer ctrl.Close()
		readOnly := node.Role == taskgraph.Plan || node.Role == taskgraph.Explore || node.Role == taskgraph.Review
		ctrl.SetPlanMode(readOnly)
		if !readOnly {
			ctrl.SetAutoApproveTools(true)
		}
		if err := ctrl.Run(ctx, prompt); err != nil {
			return taskgraph.NodeResult{Workspace: workspace, Err: err}
		}
		result := verify.Run(ctx, workspace)
		v := &taskgraph.Verification{Status: string(result.Status), Passed: append([]string(nil), result.Passed...), Failed: append([]string(nil), result.Failed...), Skipped: result.Skipped}
		if len(result.Failed) > 0 {
			return taskgraph.NodeResult{Workspace: workspace, Verification: v, Err: fmt.Errorf("verification failed: %s", strings.Join(result.Failed, "; "))}
		}
		changed, changedErr := worktrees.ChangedFiles(ctx, task.ID, node.ID)
		if changedErr != nil {
			return taskgraph.NodeResult{Workspace: workspace, Verification: v, Err: changedErr}
		}
		commit := ""
		if node.Role == taskgraph.Build && len(changed) > 0 {
			commit, changedErr = worktrees.Commit(ctx, task.ID, node.ID, fmt.Sprintf("vcode task %s build %s", task.ID, node.ID))
			if changedErr != nil {
				return taskgraph.NodeResult{Workspace: workspace, ChangedFiles: changed, Verification: v, Err: changedErr}
			}
		}
		return taskgraph.NodeResult{Workspace: workspace, Commit: commit, ChangedFiles: changed, Verification: v, Message: "agent completed and project verification passed"}
	})
	_ = taskgraph.NewIndex(config.VcodeHomeDir()).Upsert(task)
	if err != nil {
		fmt.Fprintln(stderr(), "task failed:", err)
		return 1
	}
	fmt.Printf("task %s completed\n", id)
	return 0
}

func planTask(store *taskgraph.Store, global *taskgraph.Index, id string) int {
	task, err := store.Get(id)
	if err != nil {
		fmt.Fprintln(stderr(), "error:", err)
		return 1
	}
	if len(task.Nodes) == 0 {
		task.Nodes = defaultTaskNodes(task.Goal)
	}
	task.Status = taskgraph.Blocked
	if err := store.Save(task); err != nil {
		fmt.Fprintln(stderr(), "error:", err)
		return 1
	}
	_ = store.AppendEvent(&task, taskgraph.Event{Type: "plan_ready", Message: "中文执行计划已生成，等待批准"})
	_ = global.Upsert(task)
	fmt.Printf("任务 %s 规划完成（中文、只读阶段），批准后执行：\n", id)
	for i, node := range task.Nodes {
		fmt.Printf("%d. %s：%s\n", i+1, node.Title, node.Prompt)
	}
	fmt.Printf("使用 `vcode task approve %s` 开始执行。\n", id)
	return 0
}

func approveTask(store *taskgraph.Store, global *taskgraph.Index, id string) int {
	task, err := store.Get(id)
	if err != nil {
		fmt.Fprintln(stderr(), "error:", err)
		return 1
	}
	if len(task.Nodes) == 0 {
		fmt.Fprintln(stderr(), "error: task has no plan; run `vcode task plan", id+"` first")
		return 1
	}
	if err := store.SetStatus(&task, taskgraph.Ready, "operator approved execution plan"); err != nil {
		fmt.Fprintln(stderr(), "error:", err)
		return 1
	}
	_ = global.Upsert(task)
	fmt.Printf("任务 %s 已批准，可以运行。\n", id)
	return 0
}

func defaultTaskNodes(goal string) []taskgraph.Node {
	return []taskgraph.Node{
		{ID: "explore", Title: "探索项目现状", Role: taskgraph.Explore, Prompt: fmt.Sprintf("用只读方式分析项目结构、相关文件、现有约束和当前实现，围绕目标“%s”列出事实、风险与需要修改的范围。不得写入文件。", goal)},
		{ID: "plan", Title: "制定中文实施计划", Role: taskgraph.Plan, DependsOn: []string{"explore"}, Prompt: fmt.Sprintf("根据探索结果，用中文说明要解决的问题、最终目标、涉及模块和文件；拆成 2—6 个可验证步骤，并为每步写明动作、原因和验证方式。不得写入文件。目标：%s", goal)},
		{ID: "build", Title: "实现目标并保持最小改动", Role: taskgraph.Build, DependsOn: []string{"plan"}, Prompt: fmt.Sprintf("按已确认的中文计划实现目标“%s”。只修改必要文件，保留兼容性，完成后列出变更文件、风险和可复现验证命令。", goal)},
		{ID: "verify", Title: "验证实现并汇总证据", Role: taskgraph.Test, DependsOn: []string{"build"}, Prompt: fmt.Sprintf("针对目标“%s”执行项目已有的测试、构建、静态检查和必要 smoke test；不要声称未执行的检查成功，按 VERIFIED、PARTIAL 或 UNVERIFIED 汇总证据。", goal)},
	}
}

func mergeTaskNode(store *taskgraph.Store, global *taskgraph.Index, id string, args []string) int {
	task, err := store.Get(id)
	if err != nil {
		fmt.Fprintln(stderr(), "error:", err)
		return 1
	}
	manager := worktree.NewManager(task.ProjectRoot)
	merged := 0
	for i := range task.Nodes {
		node := &task.Nodes[i]
		if len(args) > 0 && node.ID != args[0] {
			continue
		}
		if strings.TrimSpace(node.Commit) == "" {
			continue
		}
		if err := manager.MergeCommit(context.Background(), node.Commit); err != nil {
			_ = store.SetStatus(&task, taskgraph.Blocked, fmt.Sprintf("integration conflict in %s: %v", node.ID, err))
			_ = global.Upsert(task)
			fmt.Fprintln(stderr(), "integration blocked:", err)
			return 1
		}
		merged++
		_ = store.AppendEvent(&task, taskgraph.Event{Type: "node_integrated", NodeID: node.ID, Message: "commit merged into project"})
	}
	if len(args) > 0 && merged == 0 {
		fmt.Fprintln(stderr(), "error: node has no committed changes", args[0])
		return 1
	}
	_ = global.Upsert(task)
	fmt.Printf("integrated %d node commit(s) for task %s\n", merged, id)
	return 0
}

func changeTaskState(store *taskgraph.Store, global *taskgraph.Index, action, id string, rest []string) int {
	t, err := store.Get(id)
	if err != nil {
		fmt.Fprintln(stderr(), "error:", err)
		return 1
	}
	if action == "pause" || action == "cancel" {
		status := taskgraph.Blocked
		if action == "cancel" {
			status = taskgraph.Cancelled
		}
		if err := store.SetStatus(&t, status, "changed by operator"); err != nil {
			fmt.Fprintln(stderr(), "error:", err)
			return 1
		}
		_ = global.Upsert(t)
		fmt.Printf("%s %s\n", id, status)
		return 0
	}
	if err := store.RecoverInterrupted(&t); err != nil {
		fmt.Fprintln(stderr(), "error:", err)
		return 1
	}
	if len(rest) > 0 {
		status := taskgraph.Ready
		if action == "retry" {
			status = taskgraph.Pending
		}
		if err := store.UpdateNode(&t, rest[0], status, "scheduled by operator"); err != nil {
			fmt.Fprintln(stderr(), "error:", err)
			return 1
		}
	}
	if err := store.SetStatus(&t, taskgraph.Ready, "scheduled by operator"); err != nil {
		fmt.Fprintln(stderr(), "error:", err)
		return 1
	}
	_ = global.Upsert(t)
	fmt.Printf("%s ready\n", id)
	return 0
}

func listGlobalTasks(index *taskgraph.Index) int {
	entries, err := index.List()
	if err != nil {
		fmt.Fprintln(stderr(), "error:", err)
		return 1
	}
	if len(entries) == 0 {
		fmt.Println("no global tasks")
		return 0
	}
	for _, entry := range entries {
		fmt.Printf("%-28s %-12s %-36s %s\n", entry.ID, entry.Status, entry.ProjectRoot, entry.Goal)
	}
	return 0
}

func listTasks(store *taskgraph.Store) int {
	tasks, err := store.List()
	if err != nil {
		fmt.Fprintln(stderr(), "error:", err)
		return 1
	}
	if len(tasks) == 0 {
		fmt.Println("no tasks")
		return 0
	}
	for _, t := range tasks {
		fmt.Printf("%-28s %-12s %s\n", t.ID, t.Status, t.Goal)
	}
	return 0
}

func showTask(store *taskgraph.Store, id string, asJSON bool) int {
	t, err := store.Get(id)
	if err != nil {
		fmt.Fprintln(stderr(), "error:", err)
		return 1
	}
	if asJSON {
		data, marshalErr := json.MarshalIndent(t, "", "  ")
		if marshalErr != nil {
			fmt.Fprintln(stderr(), "error:", marshalErr)
			return 1
		}
		fmt.Println(string(data))
		return 0
	}
	fmt.Printf("%s  %s  outcome=%s\n%s\n", t.ID, t.Status, t.Outcome, t.Goal)
	for _, n := range t.Nodes {
		verification := ""
		if n.Verification != nil {
			verification = n.Verification.Status
		}
		fmt.Printf("  %-12s %-10s %-12s attempt=%d/%d verify=%-10s %s\n", n.ID, n.Role, n.Status, n.Attempt, n.MaxAttempts, verification, n.Title)
		if n.Workspace != "" {
			fmt.Printf("    workspace: %s\n", n.Workspace)
		}
		if n.Error != "" {
			fmt.Printf("    error: %s\n", n.Error)
		}
	}
	return 0
}

func showTaskLogs(store *taskgraph.Store, id string) int {
	t, err := store.Get(id)
	if err != nil {
		fmt.Fprintln(stderr(), "error:", err)
		return 1
	}
	for _, e := range t.Events {
		fmt.Printf("%s %-18s %-12s %-8s %s\n", e.Timestamp.Local().Format("2006-01-02 15:04:05"), e.Type, e.NodeID, e.Role, e.Message)
	}
	return 0
}
