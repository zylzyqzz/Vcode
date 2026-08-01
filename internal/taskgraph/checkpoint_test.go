package taskgraph

import "testing"

func TestCheckpointRestoreRewindsNodesAndBlackboard(t *testing.T) {
	store := NewStore(t.TempDir())
	task, err := store.Create("checkpoint", ".", []Node{{ID: "build", Role: Build, Status: Pending}})
	if err != nil {
		t.Fatal(err)
	}
	if err := task.Blackboard.UpsertFact(Fact{Key: "scope", Value: "auth", Source: "plan"}); err != nil {
		t.Fatal(err)
	}
	cp, err := store.CreateCheckpoint(&task, "计划完成")
	if err != nil {
		t.Fatal(err)
	}
	task.Nodes[0].Status = Succeeded
	task.Nodes[0].Summary = "new implementation"
	if err := task.Blackboard.UpsertFact(Fact{Key: "scope", Value: "payments", Source: "build"}); err != nil {
		t.Fatal(err)
	}
	if err := store.RestoreCheckpoint(&task, cp.ID); err != nil {
		t.Fatal(err)
	}
	if task.Nodes[0].Status != Pending || task.Nodes[0].Summary != "" {
		t.Fatalf("nodes=%+v", task.Nodes)
	}
	if fact, ok := task.Blackboard.Fact("scope"); !ok || fact.Value != "auth" {
		t.Fatalf("fact=%+v ok=%v", fact, ok)
	}
	if len(task.Events) != 2 || task.Events[1].Type != "checkpoint_restored" {
		t.Fatalf("events=%+v", task.Events)
	}
}

func TestCheckpointRequiresLabelAndSupportsLatest(t *testing.T) {
	store := NewStore(t.TempDir())
	task, err := store.Create("checkpoint label", ".", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateCheckpoint(&task, ""); err == nil {
		t.Fatal("expected label validation")
	}
	cp, err := store.CreateCheckpoint(&task, "first")
	if err != nil {
		t.Fatal(err)
	}
	latest, ok := task.LatestCheckpoint()
	if !ok || latest.ID != cp.ID {
		t.Fatalf("latest=%+v ok=%v", latest, ok)
	}
}

func TestRestoreUnknownCheckpointDoesNotMutate(t *testing.T) {
	store := NewStore(t.TempDir())
	task, err := store.Create("checkpoint missing", ".", []Node{{ID: "one", Role: Explore}})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.RestoreCheckpoint(&task, "missing"); err == nil || len(task.Events) != 0 {
		t.Fatalf("err=%v events=%d", err, len(task.Events))
	}
}
