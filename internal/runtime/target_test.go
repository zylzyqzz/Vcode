package runtime

import (
	"encoding/json"
	"testing"
)

func TestRuntimeTypesRoundTrip(t *testing.T) {
	in := RuntimeEvent{EventID: "evt-1", Seq: 4, TargetID: "pc-1", Type: "task_started", Payload: json.RawMessage(`{"ok":true}`)}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	var out RuntimeEvent
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	if out.Seq != 4 || string(out.Payload) != `{"ok":true}` {
		t.Fatalf("round trip = %+v", out)
	}
}
