package serve

import (
	"encoding/json"
	"strings"
	"sync"

	"vcode/internal/event"
	"vcode/internal/eventwire"
)

// Broadcaster is the event.Sink the controller emits to in server mode. It
// marshals each event once and fans it out to every connected SSE subscriber.
// A slow subscriber's buffer is allowed to drop rather than back-pressure the
// agent goroutine — a browser that can't keep up loses intermediate frames, not
// the whole session (it can refetch /history).
type Broadcaster struct {
	mu             sync.Mutex
	subs           map[chan []byte]struct{}
	journal        *taskStore
	activeID       string
	onDone         func(string)
	pipelineActive func(string) bool
}

// NewBroadcaster returns an empty Broadcaster ready to accept subscribers.
func NewBroadcaster() *Broadcaster {
	return &Broadcaster{subs: map[chan []byte]struct{}{}}
}

func (b *Broadcaster) SetTaskJournal(journal *taskStore) {
	b.mu.Lock()
	b.journal = journal
	b.mu.Unlock()
}

func (b *Broadcaster) SetActiveTask(id string) {
	b.mu.Lock()
	b.activeID = id
	b.mu.Unlock()
}

func (b *Broadcaster) SetTaskDoneHandler(fn func(string)) {
	b.mu.Lock()
	b.onDone = fn
	b.mu.Unlock()
}

func (b *Broadcaster) SetPipelineActive(fn func(string) bool) {
	b.mu.Lock()
	b.pipelineActive = fn
	b.mu.Unlock()
}

// Emit marshals the event to JSON and delivers it to every subscriber. Drops to
// a subscriber whose buffer is full rather than blocking. A marshal failure is
// dropped silently — one bad event shouldn't stall the stream.
func (b *Broadcaster) Emit(e event.Event) {
	data, err := json.Marshal(eventwire.ToWire(e))
	if err != nil {
		return
	}
	b.mu.Lock()
	var doneID string
	var done func(string)
	if b.journal != nil && b.activeID != "" {
		seq, _ := b.journal.appendEvent(b.activeID, data)
		var envelope map[string]any
		if json.Unmarshal(data, &envelope) == nil {
			envelope["task_id"] = b.activeID
			// The journal is the source of the sequence. Live clients receive
			// the same sequence so reconnects can request only missing events.
			envelope["task_seq"] = seq
			if framed, marshalErr := json.Marshal(envelope); marshalErr == nil {
				data = framed
			}
		}
		switch e.Kind {
		case event.TurnStarted:
			_ = b.journal.update(b.activeID, TaskRunning, "", "")
		case event.ApprovalRequest:
			_ = b.journal.update(b.activeID, TaskWaitingPermission, "permission", "waiting for approval")
		case event.Phase:
			phase := strings.ToLower(e.Text)
			agent := "Builder"
			status := TaskRunning
			switch {
			case strings.Contains(phase, "plan"):
				agent = "Planner"
			case strings.Contains(phase, "debug"):
				agent = "Debugger"
			case strings.Contains(phase, "test") || strings.Contains(phase, "verif"):
				agent = "Tester"
				status = TaskVerifying
			case strings.Contains(phase, "review"):
				agent = "Reviewer"
			case strings.Contains(phase, "explor"):
				agent = "Explorer"
			}
			_ = b.journal.setAgent(b.activeID, agent, status)
		case event.Message:
			_ = b.journal.setFinalResponse(b.activeID, e.Text)
		case event.ToolDispatch:
			_ = b.journal.markToolStart(b.activeID, e.Tool.ReadOnly, e.Tool.Args)
		case event.TurnDone:
			if b.pipelineActive == nil || !b.pipelineActive(b.activeID) {
				if e.Err != nil {
					_ = b.journal.update(b.activeID, TaskFailed, "unknown", e.Err.Error())
				} else {
					// Verification and the completion gate run from the server's
					// done callback. Do not promote a task from a model event alone.
					_ = b.journal.update(b.activeID, TaskVerifying, "", "verification pending")
				}
				id := b.activeID
				b.activeID = ""
				doneID, done = id, b.onDone
			}
		case event.ToolResult:
			_ = b.journal.toolResult(b.activeID, strings.TrimSpace(e.Tool.Err) != "")
		}
	}
	for ch := range b.subs {
		select {
		case ch <- data:
		default: // subscriber is behind; drop this frame for it
		}
	}
	b.mu.Unlock()
	if done != nil && doneID != "" {
		done(doneID)
	}
}

// EmitRemote forwards a persisted event produced by a computer Bridge to the
// same SSE subscribers used by cloud tasks. Remote task events are already
// journaled by the relay, so this method only fans out the wire payload.
func (b *Broadcaster) EmitRemote(taskID string, seq uint64, payload json.RawMessage) {
	var envelope map[string]any
	if json.Unmarshal(payload, &envelope) != nil {
		return
	}
	envelope["task_id"] = taskID
	envelope["task_seq"] = seq
	data, err := json.Marshal(envelope)
	if err != nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	for sub := range b.subs {
		select {
		case sub <- data:
		default:
		}
	}
}

// Subscribe registers a new SSE client and returns its channel plus an
// unsubscribe func the handler must call (defer) when the client disconnects.
func (b *Broadcaster) Subscribe() (<-chan []byte, func()) {
	ch := make(chan []byte, 64)
	b.mu.Lock()
	b.subs[ch] = struct{}{}
	b.mu.Unlock()
	return ch, func() {
		b.mu.Lock()
		if _, ok := b.subs[ch]; ok {
			delete(b.subs, ch)
			close(ch)
		}
		b.mu.Unlock()
	}
}

// Subscribers reports the current connection count (for diagnostics/tests).
func (b *Broadcaster) Subscribers() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.subs)
}
