package runtime

import (
	"path/filepath"
	"testing"
)

func TestStorePersistsEventsBeforeSnapshotAndResumesBySequence(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	task, err := store.Create(Task{ID: "task-1", Goal: "change code", Workspace: filepath.Join(t.TempDir(), "project")})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.Transition(task.ID, TaskRunning, "", "started"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.AppendEvent(task.ID, "tool_finished", map[string]any{"ok": true}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.Transition(task.ID, TaskVerifying, "", "verify"); err != nil {
		t.Fatal(err)
	}
	got, err := store.EventsAfter(task.ID, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Seq != 3 || got[0].Type != "task_verifying" {
		t.Fatalf("events after sequence = %+v", got)
	}
	// Simulate a process dying after the journal flush but before the manifest
	// replacement. Loading must repair the sequence from the durable journal.
	stale, err := store.Load(task.ID)
	if err != nil {
		t.Fatal(err)
	}
	stale.LastEventSeq = 0
	if err := writeJSONAtomic(store.snapshotPath(task.ID), stale); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.Load(task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.LastEventSeq != 3 || loaded.Status != TaskVerifying {
		t.Fatalf("loaded task = %+v", loaded)
	}
}

func TestStoreRejectsInvalidTransitionsAndTerminalReopen(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	task, err := store.Create(Task{Goal: "test", Workspace: filepath.Join(t.TempDir(), "project")})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.Transition(task.ID, TaskCompleted, "VERIFIED", "fake"); err == nil {
		t.Fatal("queued task accepted direct completion")
	}
	for _, status := range []TaskStatus{TaskRunning, TaskVerifying, TaskPartial} {
		if _, _, err := store.Transition(task.ID, status, "", "step"); err != nil {
			t.Fatal(err)
		}
	}
	if _, _, err := store.Transition(task.ID, TaskRunning, "", "reopen"); err == nil {
		t.Fatal("terminal task was reopened")
	}
}

func TestCompletionGateRejectsClaimsWithoutEvidence(t *testing.T) {
	decision := EvaluateCompletion(CompletionInput{
		FinalResponse:      "done",
		VerificationStatus: "VERIFIED",
		VerificationFresh:  true,
		WritesCompleted:    true,
		DiffMatchesGoal:    true,
		EvidenceRecorded:   true,
	})
	if decision.Allowed || decision.Outcome != "UNVERIFIED" {
		t.Fatalf("decision = %+v", decision)
	}
}

func TestCompletionGateAllowsOnlyCompleteEvidence(t *testing.T) {
	decision := EvaluateCompletion(CompletionInput{
		FinalResponse:      "implemented",
		ChangedFiles:       []string{"main.go"},
		VerificationStatus: "VERIFIED",
		VerificationFresh:  true,
		WritesCompleted:    true,
		DiffMatchesGoal:    true,
		EvidenceRecorded:   true,
	})
	if !decision.Allowed || decision.Outcome != "VERIFIED" || len(decision.Reasons) != 0 {
		t.Fatalf("decision = %+v", decision)
	}
}

func TestStoreListsTasksAndNodeCompletionIsIdempotent(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	created, err := store.Create(Task{Goal: "first", Workspace: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.MarkNodeSuccess(created.ID, "builder-1", "Builder"); err != nil {
		t.Fatal(err)
	}
	replayed, err := store.MarkNodeSuccess(created.ID, "builder-1", "Builder")
	if err != nil || replayed.LastEventSeq != 1 {
		t.Fatalf("replayed node = %+v, err=%v", replayed, err)
	}
	tasks, err := store.List()
	if err != nil || len(tasks) != 1 || tasks[0].LastSuccessfulNode != "builder-1" {
		t.Fatalf("tasks = %+v, err=%v", tasks, err)
	}
}
