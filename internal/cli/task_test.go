package cli

import (
	"strings"
	"testing"

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
		if i >= 3 && (len(node.DependsOn) != 1 || node.DependsOn[0] != nodes[i-1].ID) {
			t.Fatalf("node %d dependency=%v, want %s", i, node.DependsOn, nodes[i-1].ID)
		}
	}
}

func TestDependencySummariesFlowIntoNextRole(t *testing.T) {
	task := taskgraph.Task{Nodes: []taskgraph.Node{
		{ID: "explore", Summary: "发现配置位于 vcode.toml"},
		{ID: "build", DependsOn: []string{"explore"}},
	}}
	got := dependencySummaries(task, task.Nodes[1])
	if !strings.Contains(got, "发现配置位于") {
		t.Fatalf("dependency summary missing: %q", got)
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
