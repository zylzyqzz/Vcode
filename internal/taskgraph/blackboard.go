package taskgraph

import (
	"errors"
	"strings"
	"time"
)

// Blackboard is the durable, structured hand-off between agents. It keeps
// facts separate from prose summaries so later agents can consume evidence
// without replaying an entire conversation.
type Blackboard struct {
	Facts []Fact `json:"facts,omitempty"`
}

type Fact struct {
	Key        string    `json:"key"`
	Value      string    `json:"value"`
	Source     string    `json:"source"`
	NodeID     string    `json:"node_id,omitempty"`
	Confidence string    `json:"confidence,omitempty"` // low|medium|high
	UpdatedAt  time.Time `json:"updated_at"`
}

func (b *Blackboard) UpsertFact(f Fact) error {
	if b == nil {
		return errors.New("blackboard is nil")
	}
	f.Key = strings.TrimSpace(f.Key)
	f.Value = strings.TrimSpace(f.Value)
	f.Source = strings.TrimSpace(f.Source)
	if f.Key == "" || f.Value == "" || f.Source == "" {
		return errors.New("fact key, value, and source are required")
	}
	if f.Confidence == "" {
		f.Confidence = "medium"
	}
	f.UpdatedAt = time.Now().UTC()
	for i := range b.Facts {
		if b.Facts[i].Key == f.Key {
			b.Facts[i] = f
			return nil
		}
	}
	b.Facts = append(b.Facts, f)
	return nil
}

func (b Blackboard) Fact(key string) (Fact, bool) {
	for _, f := range b.Facts {
		if f.Key == strings.TrimSpace(key) {
			return f, true
		}
	}
	return Fact{}, false
}

// RecordFact persists the observation and emits a structured event, making it
// visible to both a future Coordinator and CLI event consumers.
func (s *Store) RecordFact(t *Task, f Fact) error {
	if t == nil {
		return errors.New("task is nil")
	}
	if err := t.Blackboard.UpsertFact(f); err != nil {
		return err
	}
	return s.AppendEvent(t, Event{Type: "agent_observation", NodeID: f.NodeID, Message: f.Key, Data: map[string]string{
		"value": f.Value, "source": f.Source, "confidence": f.Confidence,
	}})
}
