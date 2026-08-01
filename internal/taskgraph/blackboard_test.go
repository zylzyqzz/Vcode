package taskgraph

import (
	"testing"
)

func TestBlackboardUpsertReplacesFactByKey(t *testing.T) {
	var b Blackboard
	if err := b.UpsertFact(Fact{Key: "entrypoint", Value: "cmd/app", Source: "explorer", Confidence: "high"}); err != nil {
		t.Fatal(err)
	}
	if err := b.UpsertFact(Fact{Key: "entrypoint", Value: "cmd/vcode", Source: "reviewer"}); err != nil {
		t.Fatal(err)
	}
	if len(b.Facts) != 1 || b.Facts[0].Value != "cmd/vcode" || b.Facts[0].Confidence != "medium" {
		t.Fatalf("facts=%+v", b.Facts)
	}
}

func TestRecordFactPersistsAndEmitsStructuredEvent(t *testing.T) {
	store := NewStore(t.TempDir())
	task, err := store.Create("blackboard", ".", []Node{{ID: "explore", Role: Explore}})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.RecordFact(&task, Fact{Key: "test_command", Value: "go test ./...", Source: "explore", NodeID: "explore", Confidence: "high"}); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.Get(task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if fact, ok := loaded.Blackboard.Fact("test_command"); !ok || fact.Value != "go test ./..." {
		t.Fatalf("fact=%+v ok=%v", fact, ok)
	}
	if len(loaded.Events) != 1 || loaded.Events[0].Type != "agent_observation" || loaded.Events[0].Data["source"] != "explore" {
		t.Fatalf("events=%+v", loaded.Events)
	}
}

func TestFactRequiresSourceAndValue(t *testing.T) {
	var b Blackboard
	if err := b.UpsertFact(Fact{Key: "x", Value: ""}); err == nil {
		t.Fatal("expected invalid fact")
	}
}
