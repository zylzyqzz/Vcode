package taskgraph

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
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
