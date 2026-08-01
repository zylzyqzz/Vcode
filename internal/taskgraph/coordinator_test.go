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

type coordinatorStub struct{}

func (coordinatorStub) Decide(context.Context, CoordinationSnapshot) (CoordinationDecision, error) {
	return CoordinationDecision{Reason: "等待执行器", Actions: []CoordinationAction{{Kind: ActionWait, Message: "等待执行器"}}}, nil
}
