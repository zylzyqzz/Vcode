package taskgraph

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

type Checkpoint struct {
	ID         string     `json:"id"`
	Label      string     `json:"label"`
	CreatedAt  time.Time  `json:"created_at"`
	Status     Status     `json:"status"`
	Outcome    string     `json:"outcome,omitempty"`
	Nodes      []Node     `json:"nodes"`
	Blackboard Blackboard `json:"blackboard,omitempty"`
}

func (s *Store) CreateCheckpoint(t *Task, label string) (Checkpoint, error) {
	if t == nil {
		return Checkpoint{}, errors.New("task is nil")
	}
	label = strings.TrimSpace(label)
	if label == "" {
		return Checkpoint{}, errors.New("checkpoint label is required")
	}
	now := time.Now().UTC()
	cp := Checkpoint{ID: fmt.Sprintf("cp-%d-%d", now.UnixNano(), len(t.Checkpoints)+1), Label: label, CreatedAt: now, Status: t.Status, Outcome: t.Outcome, Nodes: cloneNodes(t.Nodes), Blackboard: cloneBlackboard(t.Blackboard)}
	t.Checkpoints = append(t.Checkpoints, cp)
	if err := s.AppendEvent(t, Event{Type: "checkpoint_created", Message: label, Data: map[string]string{"checkpoint_id": cp.ID}}); err != nil {
		return Checkpoint{}, err
	}
	return cp, nil
}

func (s *Store) RestoreCheckpoint(t *Task, checkpointID string) error {
	if t == nil {
		return errors.New("task is nil")
	}
	checkpointID = strings.TrimSpace(checkpointID)
	for _, cp := range t.Checkpoints {
		if cp.ID != checkpointID {
			continue
		}
		t.Nodes = cloneNodes(cp.Nodes)
		t.Blackboard = cloneBlackboard(cp.Blackboard)
		t.Status = cp.Status
		t.Outcome = cp.Outcome
		return s.AppendEvent(t, Event{Type: "checkpoint_restored", Message: cp.Label, Data: map[string]string{"checkpoint_id": cp.ID}})
	}
	return fmt.Errorf("checkpoint %q not found", checkpointID)
}

func (t Task) LatestCheckpoint() (Checkpoint, bool) {
	if len(t.Checkpoints) == 0 {
		return Checkpoint{}, false
	}
	return t.Checkpoints[len(t.Checkpoints)-1], true
}

func cloneNodes(nodes []Node) []Node {
	out := append([]Node(nil), nodes...)
	for i := range out {
		out[i].DependsOn = append([]string(nil), out[i].DependsOn...)
		out[i].ChangedFiles = append([]string(nil), out[i].ChangedFiles...)
		out[i].Artifacts = append([]Artifact(nil), out[i].Artifacts...)
		if out[i].Metadata != nil {
			out[i].Metadata = map[string]string{}
			for k, v := range nodes[i].Metadata {
				out[i].Metadata[k] = v
			}
		}
	}
	return out
}

func cloneBlackboard(b Blackboard) Blackboard {
	b.Facts = append([]Fact(nil), b.Facts...)
	return b
}
