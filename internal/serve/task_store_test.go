package serve

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"vcode/internal/config"
	"vcode/internal/control"
)

func TestTaskStorePersistsRecordsAndResumesEventSequence(t *testing.T) {
	s := newTaskStore(t.TempDir())
	if _, err := s.start("task-1", "修复测试", "build", "deepseek", `C:\work`, "session.jsonl"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.appendEvent("task-1", []byte(`{"kind":"turn_started"}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.appendEvent("task-1", []byte(`{"kind":"turn_done"}`)); err != nil {
		t.Fatal(err)
	}
	if err := s.update("task-1", TaskCompleted, "", ""); err != nil {
		t.Fatal(err)
	}

	record := s.activeRecord()
	if record == nil || record.Status != TaskCompleted || record.LastEvent != 2 {
		t.Fatalf("unexpected record: %+v", record)
	}
	events, err := s.events("task-1", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Seq != 2 {
		t.Fatalf("unexpected replay: %+v", events)
	}

	loaded := newTaskStore(s.root)
	loaded.root = s.root
	if err := loaded.load(); err != nil {
		t.Fatal(err)
	}
	if got := loaded.activeRecord(); got == nil || got.Status != TaskCompleted {
		t.Fatalf("loaded record missing: %+v", got)
	}
}

func TestTaskStoreMarksInFlightTaskRecoveringAfterRestart(t *testing.T) {
	root := t.TempDir()
	s := newTaskStore(root)
	if _, err := s.start("task-2", "长任务", "goal", "deepseek", root, "session.jsonl"); err != nil {
		t.Fatal(err)
	}

	restarted := newTaskStore(root)
	if err := restarted.load(); err != nil {
		t.Fatal(err)
	}
	got := restarted.activeRecord()
	if got == nil || got.Status != TaskRecovering || got.ErrorClass != "runtime_restart" {
		t.Fatalf("expected recovering record: %+v", got)
	}

	data, err := os.ReadFile(filepath.Join(root, ".tasks", "task-2.json"))
	if err != nil {
		t.Fatal(err)
	}
	var persisted TaskRecord
	if err := json.Unmarshal(data, &persisted); err != nil {
		t.Fatal(err)
	}
	if persisted.Status != TaskRecovering {
		t.Fatalf("restart status was not persisted: %+v", persisted)
	}
}

func TestTaskStoreMarksEveryInFlightTaskRecoveringAfterRestart(t *testing.T) {
	root := t.TempDir()
	s := newTaskStore(root)
	for _, id := range []string{"task-recover-a", "task-recover-b"} {
		if _, err := s.start(id, "long task "+id, "build", "deepseek", root, id+".jsonl"); err != nil {
			t.Fatal(err)
		}
	}

	restarted := newTaskStore(root)
	if err := restarted.load(); err != nil {
		t.Fatal(err)
	}
	records, err := restarted.list()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 2 {
		t.Fatalf("expected two recovered tasks, got %d", len(records))
	}
	for _, record := range records {
		if record.Status != TaskRecovering || record.ErrorClass != "runtime_restart" {
			t.Fatalf("task %s was not marked recoverable: %+v", record.ID, record)
		}
	}
}

func TestTaskStoreAppendsEventBeforeAdvancingSnapshot(t *testing.T) {
	root := t.TempDir()
	s := newTaskStore(root)
	if _, err := s.start("task-order", "event order", "build", "deepseek", root, "session.jsonl"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.appendEvent("task-order", []byte(`{"kind":"turn_started"}`)); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(s.root, "task-order.json"))
	if err != nil {
		t.Fatal(err)
	}
	var record TaskRecord
	if err := json.Unmarshal(data, &record); err != nil {
		t.Fatal(err)
	}
	events, err := s.events("task-order", 0)
	if err != nil {
		t.Fatal(err)
	}
	if record.LastEvent != 1 || len(events) != 1 || events[0].Seq != record.LastEvent {
		t.Fatalf("snapshot/event sequence diverged: record=%+v events=%+v", record, events)
	}
}

func TestTaskJournalHTTPListAndReplay(t *testing.T) {
	dir := t.TempDir()
	bc := NewBroadcaster()
	ctrl := control.New(control.Options{Sink: bc, SessionDir: dir})
	srv := New(ctrl, bc, config.ServeConfig{})
	if _, err := srv.tasks.start("task-http", "检查服务", "build", "deepseek", dir, "active.jsonl"); err != nil {
		t.Fatal(err)
	}
	if _, err := srv.tasks.appendEvent("task-http", []byte(`{"kind":"turn_started"}`)); err != nil {
		t.Fatal(err)
	}

	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()
	for _, path := range []string{"/tasks", "/tasks/task-http", "/tasks/task-http/events?after=0"} {
		resp, err := http.Get(ts.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			t.Fatalf("GET %s returned %d", path, resp.StatusCode)
		}
		resp.Body.Close()
	}
}

func TestTaskAuditDoesNotCreateWorldReadableState(t *testing.T) {
	root := t.TempDir()
	s := newTaskStore(root)
	if _, err := s.start("task-audit", "audit", "build", "deepseek", root, "session.jsonl"); err != nil {
		t.Fatal(err)
	}
	if err := s.audit("task-audit", "approval_decision", map[string]string{"allow": "true"}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(s.root, "task-audit.audit.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() == 0 {
		t.Fatal("audit entry was not written")
	}
}

func TestTaskStoreUpdatesNonActiveTaskByID(t *testing.T) {
	s := newTaskStore(t.TempDir())
	if _, err := s.start("task-old", "old", "build", "deepseek", "", "old.jsonl"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.start("task-new", "new", "build", "deepseek", "", "new.jsonl"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.appendEvent("task-old", []byte(`{"kind":"turn_started"}`)); err != nil {
		t.Fatal(err)
	}
	if err := s.update("task-old", TaskPaused, "user_cancelled", "paused"); err != nil {
		t.Fatal(err)
	}
	old, err := s.record("task-old")
	if err != nil || old.Status != TaskPaused || old.LastEvent != 1 {
		t.Fatalf("old task was not independently updated: %+v %v", old, err)
	}
	if current := s.activeRecord(); current == nil || current.ID != "task-new" {
		t.Fatalf("active task was unexpectedly replaced: %+v", current)
	}
}

func TestTaskStoreCanReactivateQueuedTaskByID(t *testing.T) {
	s := newTaskStore(t.TempDir())
	if _, err := s.start("task-first", "first", "build", "deepseek", "", "first.jsonl"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.start("task-second", "second", "build", "deepseek", "", "second.jsonl"); err != nil {
		t.Fatal(err)
	}
	if err := s.update("task-first", TaskQueued, "", ""); err != nil {
		t.Fatal(err)
	}
	if err := s.activate("task-first"); err != nil {
		t.Fatal(err)
	}
	if current := s.activeRecord(); current == nil || current.ID != "task-first" || current.Status != TaskQueued {
		t.Fatalf("task was not reactivated: %+v", current)
	}
}

func TestTaskStoreNodeCompletionIsIdempotent(t *testing.T) {
	s := newTaskStore(t.TempDir())
	if _, err := s.start("task-node", "node tracking", "build", "deepseek", "", "session.jsonl"); err != nil {
		t.Fatal(err)
	}
	if err := s.markNodeSuccess("task-node", "builder", "Builder"); err != nil {
		t.Fatal(err)
	}
	if err := s.markNodeSuccess("task-node", "builder", "Builder"); err != nil {
		t.Fatal(err)
	}
	record, err := s.record("task-node")
	if err != nil {
		t.Fatal(err)
	}
	if record.CurrentNode != "builder" || record.LastSuccessfulNode != "builder" || record.Agent != "Builder" {
		t.Fatalf("node state = %+v", record)
	}
	if err := s.update("task-node", TaskCompleted, "", "legacy test terminal state"); err != nil {
		t.Fatal(err)
	}
	if err := s.markNodeSuccess("task-node", "reviewer", "Reviewer"); err == nil {
		t.Fatal("terminal task accepted a node completion")
	}
}

func TestTaskStoreCompletionGateRejectsModelOnlyCompletion(t *testing.T) {
	s := newTaskStore(t.TempDir())
	if _, err := s.start("task-gate", "change code", "build", "deepseek", t.TempDir(), "session.jsonl"); err != nil {
		t.Fatal(err)
	}
	if err := s.setFinalResponse("task-gate", "done"); err != nil {
		t.Fatal(err)
	}
	decision, err := s.complete("task-gate")
	if err != nil {
		t.Fatal(err)
	}
	if decision.Allowed {
		t.Fatal("completion gate accepted a model-only completion")
	}
	record, err := s.record("task-gate")
	if err != nil {
		t.Fatal(err)
	}
	if record.Status != TaskPartial || record.ErrorClass != "completion_gate" {
		t.Fatalf("record = %+v", record)
	}
}

func TestTaskStoreCompletionGateRequiresFreshVerification(t *testing.T) {
	s := newTaskStore(t.TempDir())
	if _, err := s.start("task-gate-ok", "change code", "build", "deepseek", t.TempDir(), "session.jsonl"); err != nil {
		t.Fatal(err)
	}
	if err := s.markToolStart("task-gate-ok", false, `{"path":"main.go"}`); err != nil {
		t.Fatal(err)
	}
	if err := s.setFinalResponse("task-gate-ok", "implemented"); err != nil {
		t.Fatal(err)
	}
	if err := s.setVerification("task-gate-ok", "VERIFIED"); err != nil {
		t.Fatal(err)
	}
	decision, err := s.complete("task-gate-ok")
	if err != nil {
		t.Fatal(err)
	}
	if !decision.Allowed {
		t.Fatalf("decision = %+v", decision)
	}
	record, err := s.record("task-gate-ok")
	if err != nil || record.Status != TaskCompleted {
		t.Fatalf("record = %+v, err=%v", record, err)
	}
}

func TestTaskStoreSuccessfulRecoveryClearsOnlyUnresolvedFailures(t *testing.T) {
	s := newTaskStore(t.TempDir())
	if _, err := s.start("task-recovered", "recover code", "build", "deepseek", t.TempDir(), "session.jsonl"); err != nil {
		t.Fatal(err)
	}
	if err := s.toolResult("task-recovered", true); err != nil {
		t.Fatal(err)
	}
	if err := s.markToolStart("task-recovered", false, `{"path":"main.go"}`); err != nil {
		t.Fatal(err)
	}
	if err := s.setFinalResponse("task-recovered", "fixed and verified"); err != nil {
		t.Fatal(err)
	}
	if err := s.setVerification("task-recovered", "VERIFIED"); err != nil {
		t.Fatal(err)
	}
	decision, err := s.complete("task-recovered")
	if err != nil {
		t.Fatal(err)
	}
	if !decision.Allowed {
		t.Fatalf("decision = %+v", decision)
	}
	record, err := s.record("task-recovered")
	if err != nil {
		t.Fatal(err)
	}
	if record.Status != TaskCompleted || record.ToolFailures != 1 || record.UnresolvedFailures != 0 {
		t.Fatalf("recovery counters = %+v", record)
	}
}
