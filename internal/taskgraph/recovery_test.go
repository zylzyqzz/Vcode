package taskgraph

import "testing"

func TestClassifyAgentFailure(t *testing.T) {
	tests := map[string]FailureClass{
		"context deadline exceeded": FailureTimeout,
		"401 unauthorized":          FailureAuth,
		"permission denied":         FailurePermission,
		"go test failed: TestLogin": FailureTest,
		"merge conflict":            FailureConflict,
		"connection reset":          FailureTransient,
		"unknown model response":    FailureUnknown,
	}
	for message, want := range tests {
		if got := ClassifyAgentFailure(message); got != want {
			t.Errorf("%q: got %q want %q", message, got, want)
		}
	}
}

func TestDefaultRecoveryRetriesTransientWithinBudget(t *testing.T) {
	f := FailureContext{Node: Node{ID: "build", Attempt: 1, MaxAttempts: 3}, Message: "connection reset", Class: FailureTransient}
	d := DefaultRecoveryDecision(f)
	if len(d.Actions) != 1 || d.Actions[0].Kind != ActionRetryNode || d.Actions[0].NodeID != "build" {
		t.Fatalf("decision=%+v", d)
	}
}

func TestDefaultRecoveryDelegatesCodeFailureToReadOnlyAgent(t *testing.T) {
	f := FailureContext{Node: Node{ID: "build", Role: Build, Attempt: 2, DependsOn: []string{"plan"}}, Message: "compile failed", Class: FailureCompile}
	d := DefaultRecoveryDecision(f)
	if len(d.Actions) != 1 || d.Actions[0].Kind != ActionAddNode || d.Actions[0].Node == nil {
		t.Fatalf("decision=%+v", d)
	}
	if d.Actions[0].Node.Role != Review || d.Actions[0].Node.Metadata["tools"] != "read,search" {
		t.Fatalf("diagnostic node=%+v", d.Actions[0].Node)
	}
}

func TestDefaultRecoveryPausesPermanentFailure(t *testing.T) {
	f := FailureContext{Node: Node{ID: "build", Attempt: 1, MaxAttempts: 3}, Message: "permission denied", Class: FailurePermission}
	d := DefaultRecoveryDecision(f)
	if len(d.Actions) != 1 || d.Actions[0].Kind != ActionWait {
		t.Fatalf("decision=%+v", d)
	}
}

func TestWithinBudget(t *testing.T) {
	if !WithinBudget(Budget{PromptTokens: 100, OutputTokens: 50}, 100, 50) || WithinBudget(Budget{PromptTokens: 100}, 101, 0) {
		t.Fatal("budget boundary is incorrect")
	}
}
