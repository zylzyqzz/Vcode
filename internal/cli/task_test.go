package cli

import (
	"strings"
	"testing"

	"vcode/internal/taskgraph"
)

func TestDefaultTaskNodesAreChineseAndOrdered(t *testing.T) {
	nodes := defaultTaskNodes("实现登录功能")
	if len(nodes) != 4 {
		t.Fatalf("nodes=%d, want 4", len(nodes))
	}
	wantRoles := []taskgraph.Role{taskgraph.Explore, taskgraph.Plan, taskgraph.Build, taskgraph.Test}
	for i, node := range nodes {
		if node.Role != wantRoles[i] || strings.TrimSpace(node.Title) == "" || strings.TrimSpace(node.Prompt) == "" {
			t.Fatalf("node %d is incomplete: %+v", i, node)
		}
		if i > 0 && len(node.DependsOn) != 1 || i > 0 && node.DependsOn[0] != nodes[i-1].ID {
			t.Fatalf("node %d dependency=%v, want %s", i, node.DependsOn, nodes[i-1].ID)
		}
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
	if err != nil || planned.Status != taskgraph.Blocked || len(planned.Nodes) != 4 {
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
