package taskgraph

import "testing"

func TestAgentMessagesPersistAndFilterMailbox(t *testing.T) {
	store := NewStore(t.TempDir())
	task, err := store.Create("mailbox", ".", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SendMessage(&task, AgentMessage{From: "coordinator", To: "builder", Kind: "handoff", Body: "实现登录接口"}); err != nil {
		t.Fatal(err)
	}
	if err := store.SendMessage(&task, AgentMessage{From: "coordinator", To: "*", Kind: "system", Body: "请记录验证证据"}); err != nil {
		t.Fatal(err)
	}
	if got := len(task.PendingMessages("builder")); got != 2 {
		t.Fatalf("pending=%d", got)
	}
	if got := len(task.PendingMessages("tester")); got != 1 {
		t.Fatalf("tester pending=%d", got)
	}
	loaded, err := store.Get(task.ID)
	if err != nil || len(loaded.Messages) != 2 {
		t.Fatalf("loaded messages=%d err=%v", len(loaded.Messages), err)
	}
}

func TestMarkMessagesDeliveredIsIdempotent(t *testing.T) {
	store := NewStore(t.TempDir())
	task, err := store.Create("delivery", ".", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SendMessage(&task, AgentMessage{ID: "m1", From: "a", To: "b", Kind: "result", Body: "done"}); err != nil {
		t.Fatal(err)
	}
	if err := store.MarkMessagesDelivered(&task, "b", "m1"); err != nil {
		t.Fatal(err)
	}
	if len(task.PendingMessages("b")) != 0 {
		t.Fatal("message should be delivered")
	}
	if err := store.MarkMessagesDelivered(&task, "b", "m1"); err != nil {
		t.Fatal(err)
	}
}

func TestHeartbeatUpdatesExistingAgentAndPersistsEvent(t *testing.T) {
	store := NewStore(t.TempDir())
	task, err := store.Create("heartbeat", ".", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Heartbeat(&task, AgentPresence{AgentID: "builder", Role: Build, State: "running"}); err != nil {
		t.Fatal(err)
	}
	first := task.Agents[0].LastSeen
	if err := store.Heartbeat(&task, AgentPresence{AgentID: "builder", Role: Build, State: "done"}); err != nil {
		t.Fatal(err)
	}
	if len(task.Agents) != 1 || task.Agents[0].State != "done" || task.Agents[0].LastSeen.Before(first) {
		t.Fatalf("agents=%+v", task.Agents)
	}
}

func TestMessageRequiresRoutingAndBody(t *testing.T) {
	store := NewStore(t.TempDir())
	task, err := store.Create("message validation", ".", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SendMessage(&task, AgentMessage{From: "a", To: "b", Kind: "handoff"}); err == nil {
		t.Fatal("expected message validation error")
	}
}
