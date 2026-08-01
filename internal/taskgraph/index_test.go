package taskgraph

import "testing"

func TestGlobalIndexUpsertAndList(t *testing.T) {
	i := NewIndex(t.TempDir())
	first := Task{ID: "one", Goal: "first", Status: Running, ProjectRoot: "C:/one"}
	if err := i.Upsert(first); err != nil {
		t.Fatal(err)
	}
	first.Status = Succeeded
	if err := i.Upsert(first); err != nil {
		t.Fatal(err)
	}
	entries, err := i.List()
	if err != nil || len(entries) != 1 || entries[0].Status != Succeeded {
		t.Fatalf("entries=%+v err=%v", entries, err)
	}
}
