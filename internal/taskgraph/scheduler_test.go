package taskgraph

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestSchedulerHonorsDependenciesAndCompletes(t *testing.T) {
	s := NewStore(t.TempDir())
	task, err := s.Create("graph", ".", []Node{
		{ID: "explore", Role: Explore},
		{ID: "build", Role: Build, DependsOn: []string{"explore"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	var order []string
	if err := (Scheduler{Store: s, MaxParallel: 2}).Run(context.Background(), &task, func(_ context.Context, n Node) NodeResult {
		calls.Add(1)
		order = append(order, n.ID)
		return NodeResult{Message: "ok"}
	}); err != nil {
		t.Fatal(err)
	}
	if task.Status != Succeeded || calls.Load() != 2 || len(order) != 2 || order[0] != "explore" || order[1] != "build" {
		t.Fatalf("task=%+v calls=%d order=%v", task, calls.Load(), order)
	}
}

func TestSchedulerRetriesThenFails(t *testing.T) {
	s := NewStore(t.TempDir())
	task, err := s.Create("retry", ".", []Node{{ID: "build", Role: Build, MaxAttempts: 2}})
	if err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	err = (Scheduler{Store: s}).Run(context.Background(), &task, func(_ context.Context, _ Node) NodeResult {
		calls.Add(1)
		return NodeResult{Err: errors.New("broken")}
	})
	if err == nil || task.Status != Failed || calls.Load() != 2 {
		t.Fatalf("err=%v task=%+v calls=%d", err, task, calls.Load())
	}
}

func TestSchedulerIsIdempotentAfterSuccess(t *testing.T) {
	store := NewStore(t.TempDir())
	task, err := store.Create("idempotent", ".", []Node{{ID: "one", Role: Build}})
	if err != nil {
		t.Fatal(err)
	}
	var runs int
	runner := func(context.Context, Node) NodeResult {
		runs++
		return NodeResult{Verification: &Verification{Status: "VERIFIED"}}
	}
	s := Scheduler{Store: store, MaxParallel: 1}
	if err := s.Run(context.Background(), &task, runner); err != nil {
		t.Fatal(err)
	}
	if err := s.Run(context.Background(), &task, runner); err != nil {
		t.Fatal(err)
	}
	if runs != 1 {
		t.Fatalf("runner calls=%d, want 1", runs)
	}
}

func TestSchedulerAggregatesVerificationOutcome(t *testing.T) {
	s := NewStore(t.TempDir())
	task, err := s.Create("verify", ".", []Node{{ID: "test", Role: Test}})
	if err != nil {
		t.Fatal(err)
	}
	if err := (Scheduler{Store: s}).Run(context.Background(), &task, func(_ context.Context, _ Node) NodeResult {
		return NodeResult{Verification: &Verification{Status: "VERIFIED"}}
	}); err != nil {
		t.Fatal(err)
	}
	if task.Outcome != "VERIFIED" {
		t.Fatalf("outcome = %q", task.Outcome)
	}
}

func TestAggregateOutcomeIgnoresReadOnlyResearch(t *testing.T) {
	task := Task{Nodes: []Node{
		{ID: "explore", Role: Explore, Verification: &Verification{Status: "UNVERIFIED"}},
		{ID: "build", Role: Build, Verification: &Verification{Status: "VERIFIED"}},
	}}
	if got := aggregateOutcome(task); got != "VERIFIED" {
		t.Fatalf("outcome=%q, want VERIFIED", got)
	}
}

func TestSchedulerKeepsParallelSuccessWhenSiblingFails(t *testing.T) {
	store := NewStore(t.TempDir())
	task, err := store.Create("partial parallel", ".", []Node{
		{ID: "good", Role: Build, MaxAttempts: 1},
		{ID: "bad", Role: Build, MaxAttempts: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	var events []string
	err = (Scheduler{Store: store, MaxParallel: 2, OnEvent: func(e Event) { events = append(events, e.Type+":"+e.NodeID) }}).Run(context.Background(), &task, func(_ context.Context, n Node) NodeResult {
		if n.ID == "bad" {
			return NodeResult{Err: errors.New("broken")}
		}
		return NodeResult{Verification: &Verification{Status: "VERIFIED"}}
	})
	if err == nil || task.Status != Failed || task.Outcome != "PARTIAL" {
		t.Fatalf("err=%v status=%q outcome=%q", err, task.Status, task.Outcome)
	}
	if task.Nodes[0].Status != Succeeded {
		t.Fatalf("successful sibling was lost: %+v", task.Nodes[0])
	}
	if len(events) < 2 {
		t.Fatalf("scheduler emitted too few lifecycle events: %v", events)
	}
}

func TestSchedulerDoesNotResurrectCancelledTask(t *testing.T) {
	store := NewStore(t.TempDir())
	task, err := store.Create("cancelled", ".", []Node{{ID: "build", Role: Build}})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SetStatus(&task, Cancelled, "operator cancelled"); err != nil {
		t.Fatal(err)
	}
	runs := 0
	err = (Scheduler{Store: store}).Run(context.Background(), &task, func(context.Context, Node) NodeResult {
		runs++
		return NodeResult{Verification: &Verification{Status: "VERIFIED"}}
	})
	if err == nil || !strings.Contains(err.Error(), "cancelled") || runs != 0 || task.Status != Cancelled {
		t.Fatalf("err=%v runs=%d status=%q", err, runs, task.Status)
	}
}

func TestSchedulerSerializesWritableNodes(t *testing.T) {
	store := NewStore(t.TempDir())
	task, err := store.Create("serial writes", t.TempDir(), []Node{
		{ID: "build-a", Role: Build},
		{ID: "build-b", Role: Build},
	})
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	active, maxActive := 0, 0
	if err := (Scheduler{Store: store, MaxParallel: 4}).Run(context.Background(), &task, func(context.Context, Node) NodeResult {
		mu.Lock()
		active++
		if active > maxActive {
			maxActive = active
		}
		mu.Unlock()
		time.Sleep(20 * time.Millisecond)
		mu.Lock()
		active--
		mu.Unlock()
		return NodeResult{Verification: &Verification{Status: "VERIFIED"}}
	}); err != nil {
		t.Fatal(err)
	}
	if maxActive != 1 {
		t.Fatalf("max concurrent writable nodes=%d, want 1", maxActive)
	}
}

func TestSchedulerClassifiesVerificationFailure(t *testing.T) {
	store := NewStore(t.TempDir())
	task, err := store.Create("classification", t.TempDir(), []Node{{ID: "test", Role: Test, MaxAttempts: 1}})
	if err != nil {
		t.Fatal(err)
	}
	_ = (Scheduler{Store: store}).Run(context.Background(), &task, func(context.Context, Node) NodeResult {
		return NodeResult{Err: errors.New("verification failed: go test ./...")}
	})
	if task.Nodes[0].FailureClass != FailureTest {
		t.Fatalf("failure class=%q, want %q", task.Nodes[0].FailureClass, FailureTest)
	}
}

func TestSchedulerAddsDebuggerAndTesterAfterExhaustedTestFailure(t *testing.T) {
	store := NewStore(t.TempDir())
	task, err := store.Create("recover test", t.TempDir(), []Node{{ID: "test", Role: Test, MaxAttempts: 1}})
	if err != nil {
		t.Fatal(err)
	}
	var roles []Role
	err = (Scheduler{Store: store, MaxRecovery: 1}).Run(context.Background(), &task, func(_ context.Context, n Node) NodeResult {
		roles = append(roles, n.Role)
		if n.ID == "test" {
			return NodeResult{Err: errors.New("verification failed: go test ./...")}
		}
		return NodeResult{Verification: &Verification{Status: "VERIFIED"}}
	})
	if err != nil || task.Status != Succeeded || task.Outcome != "VERIFIED" {
		t.Fatalf("err=%v status=%q outcome=%q nodes=%+v", err, task.Status, task.Outcome, task.Nodes)
	}
	if len(roles) != 3 || roles[1] != Debug || roles[2] != Test {
		t.Fatalf("roles=%v, want test, debug, test", roles)
	}
}
