package serve

import (
	"encoding/json"
	"testing"

	"vcode/internal/event"
)

func TestBroadcasterJournalsSequencedLiveFrames(t *testing.T) {
	store := newTaskStore(t.TempDir())
	if _, err := store.start("task-1", "inspect", "build", "model", "", "session"); err != nil {
		t.Fatal(err)
	}
	b := NewBroadcaster()
	b.SetTaskJournal(store)
	b.SetActiveTask("task-1")
	ch, unsubscribe := b.Subscribe()
	defer unsubscribe()
	b.Emit(event.Event{Kind: event.TurnStarted})
	var frame map[string]any
	if err := json.Unmarshal(<-ch, &frame); err != nil {
		t.Fatal(err)
	}
	if frame["task_id"] != "task-1" || frame["task_seq"] != float64(1) {
		t.Fatalf("missing live task sequence: %#v", frame)
	}
	replayed, err := store.events("task-1", 0)
	if err != nil || len(replayed) != 1 || replayed[0].Seq != 1 {
		t.Fatalf("journal replay mismatch: %#v %v", replayed, err)
	}
}

func TestBroadcasterJournalsTaskLifecycle(t *testing.T) {
	store := newTaskStore(t.TempDir())
	if _, err := store.start("task-2", "finish", "build", "model", "", "session"); err != nil {
		t.Fatal(err)
	}
	b := NewBroadcaster()
	b.SetTaskJournal(store)
	ch, unsubscribe := b.Subscribe()
	defer unsubscribe()
	b.EmitTaskLifecycle("task-2", "task_completed", map[string]any{"status": "completed"})
	var frame map[string]any
	if err := json.Unmarshal(<-ch, &frame); err != nil {
		t.Fatal(err)
	}
	if frame["type"] != "task_completed" || frame["task_id"] != "task-2" || frame["task_seq"] != float64(1) {
		t.Fatalf("unexpected lifecycle frame: %#v", frame)
	}
	replayed, err := store.events("task-2", 0)
	if err != nil || len(replayed) != 1 {
		t.Fatalf("lifecycle event was not journaled: %#v %v", replayed, err)
	}
}

func TestTaskLifecycleKindPreservesPartialOutcome(t *testing.T) {
	if got := taskLifecycleKind(TaskPartial); got != "task_partial" {
		t.Fatalf("partial lifecycle kind=%q", got)
	}
	if got := taskLifecycleKind(TaskFailed); got != "task_failed" {
		t.Fatalf("failed lifecycle kind=%q", got)
	}
}
