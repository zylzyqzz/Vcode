package bridge

import (
	"encoding/json"
	"time"

	"vcode/internal/runtime"
)

// Message is the small, transport-neutral envelope used by a future LAN or
// relay connection. Payloads are kept opaque so the bridge never needs to
// duplicate the Agent protocol.
type Message struct {
	Type      string          `json:"type"`
	RequestID string          `json:"request_id,omitempty"`
	TargetID  string          `json:"target_id,omitempty"`
	TaskID    string          `json:"task_id,omitempty"`
	SessionID string          `json:"session_id,omitempty"`
	Seq       uint64          `json:"seq,omitempty"`
	Timestamp time.Time       `json:"timestamp"`
	Payload   json.RawMessage `json:"payload,omitempty"`
}

const (
	MessageHello        = "hello"
	MessageHelloAck     = "hello_ack"
	MessageTaskSubmit   = "task_submit"
	MessageTaskControl  = "task_control"
	MessageRuntimeEvent = "runtime_event"
	MessageError        = "error"
)

// HelloPayload keeps the normal target announcement compatible while allowing
// the short-lived pairing flow to publish a one-time challenge to the relay.
// Pairing is never part of the public target list and never contains the
// long-lived bridge token.
type HelloPayload struct {
	Target  runtime.RuntimeTarget   `json:"target"`
	Pairing *runtime.PairingRequest `json:"pairing,omitempty"`
}

// RetryDelay returns the bounded reconnect schedule required by the bridge
// contract: fast recovery for short network blips, without hammering the
// relay during a longer outage.
func RetryDelay(attempt int) time.Duration {
	if attempt < 0 {
		attempt = 0
	}
	d := time.Second
	for i := 0; i < attempt && d < time.Minute; i++ {
		d *= 2
	}
	if d > time.Minute {
		d = time.Minute
	}
	return d
}
