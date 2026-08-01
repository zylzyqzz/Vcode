package taskgraph

import (
	"context"
	"testing"
)

func TestValidateDecisionAcceptsDynamicRoleAssignment(t *testing.T) {
	task := Task{Nodes: []Node{{ID: "explore", Role: Explore}}}
	decision := CoordinationDecision{Reason: "探索结果显示需要独立安全审计", Actions: []CoordinationAction{{
		Kind: ActionAddNode,
		Node: &Node{ID: "security-audit", Role: Review, DependsOn: []string{"explore"}, Metadata: map[string]string{"tools": "read,search"}},
	}}}
	if err := ValidateDecision(task, decision); err != nil {
		t.Fatal(err)
	}
}

func TestValidateDecisionRejectsUnknownDependency(t *testing.T) {
	err := ValidateDecision(Task{}, CoordinationDecision{Reason: "split", Actions: []CoordinationAction{{
		Kind: ActionAddNode, Node: &Node{ID: "build", Role: Build, DependsOn: []string{"missing"}},
	}}})
	if err == nil {
		t.Fatal("expected unknown dependency error")
	}
}

func TestValidateDecisionRejectsWriteCapablePlan(t *testing.T) {
	err := ValidateDecision(Task{}, CoordinationDecision{Reason: "plan", Actions: []CoordinationAction{{
		Kind: ActionAddNode, Node: &Node{ID: "plan", Role: Plan, Metadata: map[string]string{"tools": "read,write_file"}},
	}}})
	if err == nil {
		t.Fatal("expected read-only plan error")
	}
}

func TestValidateDecisionSupportsRetryAndWait(t *testing.T) {
	task := Task{Nodes: []Node{{ID: "build", Role: Build}}}
	for _, action := range []CoordinationAction{
		{Kind: ActionRetryNode, NodeID: "build"},
		{Kind: ActionWait, Message: "等待用户批准"},
		{Kind: ActionCancel, Message: "预算耗尽"},
	} {
		if err := ValidateDecision(task, CoordinationDecision{Reason: "operator policy", Actions: []CoordinationAction{action}}); err != nil {
			t.Fatalf("%s: %v", action.Kind, err)
		}
	}
}

func TestCoordinatorInterfaceCanBeDrivenBySchedulerPolicy(t *testing.T) {
	var c Coordinator = coordinatorStub{}
	decision, err := c.Decide(context.Background(), CoordinationSnapshot{Task: Task{Goal: "audit"}})
	if err != nil || len(decision.Actions) != 1 || decision.Actions[0].Kind != ActionWait {
		t.Fatalf("decision=%+v err=%v", decision, err)
	}
}

func TestApplyDecisionAddsDynamicNodeAtomically(t *testing.T) {
	store := NewStore(t.TempDir())
	task, err := store.Create("dynamic", ".", []Node{{ID: "explore", Role: Explore}})
	if err != nil {
		t.Fatal(err)
	}
	err = store.ApplyDecision(&task, CoordinationDecision{Reason: "发现需要审计", Actions: []CoordinationAction{{
		Kind: ActionAddNode,
		Node: &Node{ID: "audit", Role: Review, DependsOn: []string{"explore"}},
	}}})
	if err != nil || len(task.Nodes) != 2 || task.Nodes[1].Status != Pending {
		t.Fatalf("nodes=%+v err=%v", task.Nodes, err)
	}
	loaded, err := store.Get(task.ID)
	if err != nil || len(loaded.Events) != 1 || loaded.Events[0].Type != "coordination_decision" {
		t.Fatalf("loaded=%+v err=%v", loaded, err)
	}
}

func TestApplyDecisionRetryResetsNodeState(t *testing.T) {
	store := NewStore(t.TempDir())
	task, err := store.Create("retry", ".", []Node{{ID: "build", Role: Build, Status: Failed, Error: "compile"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.ApplyDecision(&task, CoordinationDecision{Reason: "交给修复 Agent", Actions: []CoordinationAction{{Kind: ActionRetryNode, NodeID: "build"}}}); err != nil {
		t.Fatal(err)
	}
	if task.Nodes[0].Status != Pending || task.Nodes[0].Error != "" {
		t.Fatalf("node=%+v", task.Nodes[0])
	}
}

func TestApplyDecisionWaitAndCancelChangeTaskState(t *testing.T) {
	store := NewStore(t.TempDir())
	task, err := store.Create("operator", ".", []Node{{ID: "build", Role: Build}})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.ApplyDecision(&task, CoordinationDecision{Reason: "需要批准", Actions: []CoordinationAction{{Kind: ActionWait, Message: "等待批准"}}}); err != nil || task.Status != Blocked {
		t.Fatalf("wait status=%q err=%v", task.Status, err)
	}
	if err := store.ApplyDecision(&task, CoordinationDecision{Reason: "用户取消", Actions: []CoordinationAction{{Kind: ActionCancel, Message: "用户取消"}}}); err != nil || task.Status != Cancelled {
		t.Fatalf("cancel status=%q err=%v", task.Status, err)
	}
}

func TestApplyDecisionRejectsWholeMutationBeforePersisting(t *testing.T) {
	store := NewStore(t.TempDir())
	task, err := store.Create("atomic", ".", []Node{{ID: "explore", Role: Explore}})
	if err != nil {
		t.Fatal(err)
	}
	err = store.ApplyDecision(&task, CoordinationDecision{Reason: "bad", Actions: []CoordinationAction{
		{Kind: ActionAddNode, Node: &Node{ID: "audit", Role: Review}},
		{Kind: ActionAddNode, Node: &Node{ID: "audit", Role: Review}},
	}})
	if err == nil || len(task.Nodes) != 1 {
		t.Fatalf("err=%v nodes=%d", err, len(task.Nodes))
	}
}

type coordinatorStub struct{}

func (coordinatorStub) Decide(context.Context, CoordinationSnapshot) (CoordinationDecision, error) {
	return CoordinationDecision{Reason: "等待执行器", Actions: []CoordinationAction{{Kind: ActionWait, Message: "等待执行器"}}}, nil
}
