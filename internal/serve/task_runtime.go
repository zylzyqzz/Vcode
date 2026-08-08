package serve

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// TaskRequest is the transport-neutral input used by CLI and cloud callers.
// Execution is still owned by Server/controller; this type only prevents
// transport handlers from inventing their own task shape.
type TaskRequest struct {
	ID             string
	Goal           string
	Mode           string
	Model          string
	Workspace      string
	SessionID      string
	IdempotencyKey string
}

// RuntimeEvent is the durable event envelope shared by task APIs and SSE.
// Payload contains the existing eventwire object for backwards compatibility.
type RuntimeEvent struct {
	EventID   string          `json:"event_id"`
	Seq       uint64          `json:"seq"`
	TaskID    string          `json:"task_id"`
	SessionID string          `json:"session_id,omitempty"`
	Type      string          `json:"type,omitempty"`
	Timestamp time.Time       `json:"timestamp"`
	Payload   json.RawMessage `json:"payload"`
}

// TaskRuntime is the small contract all task-facing transports use. It keeps
// persistence and event replay independent from the controller/UI lifetime.
type TaskRuntime interface {
	Submit(context.Context, TaskRequest) (*TaskRecord, error)
	Resume(context.Context, string) error
	Events(string, uint64) ([]RuntimeEvent, error)
	Control(context.Context, string, string) error
}

type durableTaskRuntime struct {
	store *taskStore
}

func newDurableTaskRuntime(store *taskStore) TaskRuntime {
	return &durableTaskRuntime{store: store}
}

func (r *durableTaskRuntime) Submit(_ context.Context, req TaskRequest) (*TaskRecord, error) {
	if r == nil || r.store == nil {
		return nil, fmt.Errorf("task runtime is unavailable")
	}
	if strings.TrimSpace(req.Goal) == "" {
		return nil, fmt.Errorf("task goal is required")
	}
	id := strings.TrimSpace(req.ID)
	if id == "" {
		id = fmt.Sprintf("task-%d", time.Now().UnixNano())
	}
	if existing, err := r.store.findByIdempotency(req.IdempotencyKey); err != nil {
		return nil, err
	} else if existing != nil {
		return existing, nil
	}
	record, err := r.store.startWithKey(id, req.Goal, req.Mode, req.Model, req.Workspace, req.SessionID, req.IdempotencyKey)
	if err != nil {
		return nil, err
	}
	return &record, nil
}

func (r *durableTaskRuntime) Resume(_ context.Context, id string) error {
	if r == nil || r.store == nil {
		return fmt.Errorf("task runtime is unavailable")
	}
	if _, err := r.store.record(id); err != nil {
		return err
	}
	return r.store.adoptPaused(id)
}

func (r *durableTaskRuntime) Events(id string, after uint64) ([]RuntimeEvent, error) {
	if r == nil || r.store == nil {
		return nil, fmt.Errorf("task runtime is unavailable")
	}
	return r.store.events(id, after)
}

func (r *durableTaskRuntime) Control(_ context.Context, id, action string) error {
	if r == nil || r.store == nil {
		return fmt.Errorf("task runtime is unavailable")
	}
	switch strings.ToLower(strings.TrimSpace(action)) {
	case "pause":
		return r.store.update(id, TaskPaused, "user_paused", "task paused by user")
	case "cancel":
		return r.store.update(id, TaskCancelled, "user_cancelled", "task cancelled by user")
	case "resume", "continue", "retry":
		return r.store.adoptPaused(id)
	default:
		return fmt.Errorf("unsupported task action %q", action)
	}
}
