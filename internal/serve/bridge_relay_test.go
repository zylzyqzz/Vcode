package serve

import (
	"encoding/json"
	"testing"
	"time"

	"vcode/internal/bridge"
)

func TestBridgeRelayPersistsRemoteTaskAndSequencesEvents(t *testing.T) {
	relay := newBridgeRelay("secret")
	now := time.Now().UTC()
	relay.addTask(&TaskRecord{ID: "remote-1", Status: TaskQueued, CreatedAt: now, UpdatedAt: now})

	started, _ := json.Marshal(map[string]string{"type": "task_started"})
	finished, _ := json.Marshal(map[string]string{"type": "task_completed"})
	relay.record(bridge.Message{TaskID: "remote-1", Payload: started})
	relay.record(bridge.Message{TaskID: "remote-1", Payload: finished})

	events, known := relay.eventsAfter("remote-1", 1)
	if !known || len(events) != 1 || events[0].Seq != 2 {
		t.Fatalf("events after sequence = known:%v events:%+v", known, events)
	}
	task, ok := relay.task("remote-1")
	if !ok || task.Status != TaskCompleted || task.LastEvent != 2 {
		t.Fatalf("remote task = ok:%v task:%+v", ok, task)
	}
}
