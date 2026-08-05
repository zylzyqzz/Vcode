package bridge

import (
	"encoding/json"
	"testing"
	"time"
)

func TestMessageRoundTripAndRetrySchedule(t *testing.T) {
	in := Message{Type: MessageRuntimeEvent, TargetID: "pc-1", TaskID: "task-1", Seq: 7, Timestamp: time.Now().UTC(), Payload: json.RawMessage(`{"ok":true}`)}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	var out Message
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	if out.Type != in.Type || out.Seq != 7 || string(out.Payload) != string(in.Payload) {
		t.Fatalf("message = %+v", out)
	}
	for i, want := range []time.Duration{time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second, 16 * time.Second, 32 * time.Second, time.Minute, time.Minute} {
		if got := RetryDelay(i); got != want {
			t.Fatalf("retry %d = %s, want %s", i, got, want)
		}
	}
}
