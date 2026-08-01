package taskgraph

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestSchedulerEnforcesParallelismLimit(t *testing.T) {
	s := NewStore(t.TempDir())
	task, err := s.Create("parallel", ".", []Node{
		{ID: "a", Role: Explore}, {ID: "b", Role: Explore}, {ID: "c", Role: Explore}, {ID: "d", Role: Explore},
	})
	if err != nil {
		t.Fatal(err)
	}
	var active, peak atomic.Int32
	if err := (Scheduler{Store: s, MaxParallel: 2}).Run(context.Background(), &task, func(_ context.Context, _ Node) NodeResult {
		current := active.Add(1)
		for {
			old := peak.Load()
			if current <= old || peak.CompareAndSwap(old, current) {
				break
			}
		}
		time.Sleep(10 * time.Millisecond)
		active.Add(-1)
		return NodeResult{Verification: &Verification{Status: "VERIFIED"}}
	}); err != nil {
		t.Fatal(err)
	}
	if peak.Load() > 2 {
		t.Fatalf("peak concurrency = %d, want <= 2", peak.Load())
	}
}

func TestSchedulerCancellationLeavesTaskInterrupted(t *testing.T) {
	s := NewStore(t.TempDir())
	task, err := s.Create("cancel", ".", []Node{{ID: "slow", Role: Build}})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	var once sync.Once
	err = (Scheduler{Store: s}).Run(ctx, &task, func(ctx context.Context, _ Node) NodeResult {
		once.Do(cancel)
		<-ctx.Done()
		return NodeResult{Err: ctx.Err()}
	})
	if err == nil || task.Status != Interrupted {
		t.Fatalf("err=%v status=%q", err, task.Status)
	}
}
