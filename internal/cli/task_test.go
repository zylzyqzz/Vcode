package cli

import (
	"encoding/json"
	"strings"
	"testing"

	"vcode/internal/compat"
	"vcode/internal/provider"
	"vcode/internal/taskgraph"
)

func TestDefaultTaskNodesAreChineseAndOrdered(t *testing.T) {
	nodes := defaultTaskNodes("实现登录功能")
	if len(nodes) != 5 {
		t.Fatalf("nodes=%d, want 5", len(nodes))
	}
	wantRoles := []taskgraph.Role{taskgraph.Explore, taskgraph.Review, taskgraph.Plan, taskgraph.Build, taskgraph.Test}
	for i, node := range nodes {
		if node.Role != wantRoles[i] || strings.TrimSpace(node.Title) == "" || strings.TrimSpace(node.Prompt) == "" {
			t.Fatalf("node %d is incomplete: %+v", i, node)
		}
		if i == 2 && len(node.DependsOn) != 2 {
			t.Fatalf("plan dependencies=%v, want two research roles", node.DependsOn)
		}
		if node.MaxSteps <= 0 {
			t.Fatalf("node %s has no long-task step budget", node.ID)
		}
		if i >= 3 && (len(node.DependsOn) != 1 || node.DependsOn[0] != nodes[i-1].ID) {
			t.Fatalf("node %d dependency=%v, want %s", i, node.DependsOn, nodes[i-1].ID)
		}
	}
}

func TestCompatibleAgentForRoleUsesExternalAliases(t *testing.T) {
	agents := []compat.AgentSpec{
		{Name: "architect", Model: "planner-model"},
		{Name: "debugger", Model: "debug-model"},
	}
	if got := compatibleAgentForRole(agents, "plan"); got == nil || got.Model != "planner-model" {
		t.Fatalf("plan agent=%+v", got)
	}
	if got := compatibleAgentForRole(agents, "debug"); got == nil || got.Model != "debug-model" {
		t.Fatalf("debug agent=%+v", got)
	}
	if got := compatibleAgentForRole(agents, "build"); got != nil {
		t.Fatalf("unexpected build agent=%+v", got)
	}
}

func TestApplyNodeBudgetsDoesNotOverwriteExplicitLimit(t *testing.T) {
	nodes := []taskgraph.Node{{ID: "build", Role: taskgraph.Build, MaxSteps: 7}, {ID: "plan", Role: taskgraph.Plan}}
	applyNodeBudgets(nodes)
	if nodes[0].MaxSteps != 7 || nodes[1].MaxSteps <= 0 {
		t.Fatalf("budgets=%+v", nodes)
	}
}

func TestDependencySummariesFlowIntoNextRole(t *testing.T) {
	task := taskgraph.Task{Nodes: []taskgraph.Node{
		{ID: "explore", Summary: "发现配置位于 vcode.toml", ChangedFiles: []string{"vcode.toml"}, Verification: &taskgraph.Verification{Status: "PARTIAL", Failed: []string{"go test ./..."}}},
		{ID: "build", DependsOn: []string{"explore"}},
	}}
	got := dependencySummaries(task, task.Nodes[1])
	if !strings.Contains(got, "发现配置位于") {
		t.Fatalf("dependency summary missing: %q", got)
	}
	for _, want := range []string{"changed files: vcode.toml", "verification: PARTIAL", "failed=go test ./..."} {
		if !strings.Contains(got, want) {
			t.Fatalf("dependency evidence %q missing: %q", want, got)
		}
	}
	history := []provider.Message{{Role: provider.RoleUser, Content: "task"}, {Role: provider.RoleAssistant, Content: "完成了探索"}}
	if got := lastAssistantSummary(history); got != "完成了探索" {
		t.Fatalf("summary=%q", got)
	}
}

func TestPlanRequiresApprovalBeforeExecution(t *testing.T) {
	root := t.TempDir()
	store := taskgraph.NewStore(root)
	global := taskgraph.NewIndex(t.TempDir())
	task, err := store.Create("持续改进 CLI", root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := planTask(store, global, task.ID); got != 0 {
		t.Fatalf("plan exit=%d", got)
	}
	planned, err := store.Get(task.ID)
	if err != nil || planned.Status != taskgraph.Blocked || len(planned.Nodes) != 5 {
		t.Fatalf("planned task=%+v err=%v", planned, err)
	}
	if got := approveTask(store, global, task.ID); got != 0 {
		t.Fatalf("approve exit=%d", got)
	}
	approved, err := store.Get(task.ID)
	if err != nil || approved.Status != taskgraph.Ready {
		t.Fatalf("approved task=%+v err=%v", approved, err)
	}
}

func TestTaskJSONListAndLogsAreMachineReadable(t *testing.T) {
	store := taskgraph.NewStore(t.TempDir())
	task, err := store.Create("json task", ".", []taskgraph.Node{{ID: "build", Role: taskgraph.Build}})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.AppendEvent(&task, taskgraph.Event{Type: "operator_test", Message: "ok"}); err != nil {
		t.Fatal(err)
	}
	list := captureStdout(t, func() { listTasks(store, true) })
	var tasks []taskgraph.Task
	if err := json.Unmarshal([]byte(list), &tasks); err != nil || len(tasks) != 1 || tasks[0].ID != task.ID {
		t.Fatalf("list json=%q err=%v", list, err)
	}
	logs := captureStdout(t, func() { showTaskLogs(store, task.ID, true) })
	var events []taskgraph.Event
	if err := json.Unmarshal([]byte(logs), &events); err != nil || len(events) == 0 || events[0].Type != "operator_test" {
		t.Fatalf("logs json=%q err=%v", logs, err)
	}
}

func TestTaskAgentsJSONIsMachineReadable(t *testing.T) {
	store := taskgraph.NewStore(t.TempDir())
	task, err := store.Create("agents", ".", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Heartbeat(&task, taskgraph.AgentPresence{AgentID: "builder", Role: taskgraph.Build, State: "running"}); err != nil {
		t.Fatal(err)
	}
	agents := captureStdout(t, func() { showTaskAgents(store, task.ID, true) })
	var got []taskgraph.AgentPresence
	if err := json.Unmarshal([]byte(agents), &got); err != nil || len(got) != 1 || got[0].AgentID != "builder" {
		t.Fatalf("agents json=%q err=%v", agents, err)
	}
}
