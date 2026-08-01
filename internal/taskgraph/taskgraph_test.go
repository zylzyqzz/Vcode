package taskgraph

import (
	"path/filepath"
	"testing"
)

func TestStorePersistsDependencyGraphAndReadyNodes(t *testing.T) {
	root := t.TempDir()
	s := NewStore(root)
	task, err := s.Create("refactor auth", root, []Node{
		{ID: "explore", Role: Explore, Title: "inspect auth"},
		{ID: "build", Role: Build, Title: "implement auth", DependsOn: []string{"explore"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	ready := ReadyNodes(task)
	if len(ready) != 1 || ready[0].ID != "explore" {
		t.Fatalf("ready nodes = %+v", ready)
	}
	loaded, err := s.Get(task.ID)
	if err != nil || loaded.Goal != "refactor auth" {
		t.Fatalf("loaded task = %+v, err=%v", loaded, err)
	}
	if filepath.Dir(s.Root()) != filepath.Join(root, ".vcode") {
		t.Fatalf("unexpected store root %q", s.Root())
	}
}

func TestRecoverInterruptedNodes(t *testing.T) {
	root := t.TempDir()
	s := NewStore(root)
	task, err := s.Create("resume", ".", []Node{{ID: "build", Status: Running}})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.RecoverInterrupted(&task); err != nil {
		t.Fatal(err)
	}
	if task.Nodes[0].Status != Interrupted {
		t.Fatalf("status = %q, want interrupted", task.Nodes[0].Status)
	}
	loaded, err := NewStore(root).Get(task.ID)
	if err != nil || loaded.Nodes[0].Status != Interrupted {
		t.Fatalf("recovered state was not persisted: task=%+v err=%v", loaded, err)
	}
}

func TestRejectsMissingDependency(t *testing.T) {
	s := NewStore(t.TempDir())
	if _, err := s.Create("bad", ".", []Node{{ID: "build", DependsOn: []string{"missing"}}}); err == nil {
		t.Fatal("expected missing dependency error")
	}
}

func TestUpdateNodeRecordsLifecycleEvent(t *testing.T) {
	s := NewStore(t.TempDir())
	task, err := s.Create("build", ".", []Node{{ID: "build", Role: Build}})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateNode(&task, "build", Running, "started"); err != nil {
		t.Fatal(err)
	}
	if task.Nodes[0].Attempt != 1 || task.Nodes[0].Status != Running {
		t.Fatalf("node = %+v", task.Nodes[0])
	}
	if len(task.Events) != 1 || task.Events[0].Type != "node_running" {
		t.Fatalf("events = %+v", task.Events)
	}
}

func TestRetryableNodeClearsStaleFailureState(t *testing.T) {
	s := NewStore(t.TempDir())
	task, err := s.Create("retry", ".", []Node{{ID: "build", Role: Build}})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateNode(&task, "build", Running, "started"); err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateNode(&task, "build", Failed, "compiler failed"); err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateNode(&task, "build", Pending, "retry scheduled"); err != nil {
		t.Fatal(err)
	}
	if task.Nodes[0].Error != "" || task.Nodes[0].FinishedAt != nil || task.Nodes[0].Status != Pending {
		t.Fatalf("stale failure state survived retry: %+v", task.Nodes[0])
	}
}

func TestNodeSummaryAndIntegrationStatePersist(t *testing.T) {
	s := NewStore(t.TempDir())
	task, err := s.Create("persist result", ".", []Node{{ID: "build", Role: Build, Summary: "changed auth.go", Commit: "abc123", Integrated: true, MaxSteps: 100, PromptTokens: 10, OutputTokens: 4, CachedTokens: 8, Verification: &Verification{Status: "PARTIAL", Evidence: []CheckEvidence{{Name: "go test", Command: "go test ./...", Status: "failed", Output: "compile error", DurationMS: 12}}}}})
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := s.Get(task.ID)
	if err != nil || loaded.Nodes[0].Summary != "changed auth.go" || !loaded.Nodes[0].Integrated || loaded.Nodes[0].CachedTokens != 8 || len(loaded.Nodes[0].Verification.Evidence) != 1 {
		t.Fatalf("loaded node=%+v err=%v", loaded.Nodes[0], err)
	}
}
