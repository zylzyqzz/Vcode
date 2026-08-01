package taskgraph

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// Coordinator is the policy boundary for multi-agent execution. It decides
// what should happen next; Scheduler remains responsible for durable state,
// dependency ordering, retries, and running the assigned node.
type Coordinator interface {
	Decide(context.Context, CoordinationSnapshot) (CoordinationDecision, error)
}

// CoordinationSnapshot is deliberately read-only from the Coordinator's
// perspective. A coordinator may inspect the task and recent events, but it
// can only mutate the graph through a validated decision.
type CoordinationSnapshot struct {
	Task         Task
	RecentEvents []Event
	Budget       Budget
}

type Budget struct {
	MaxNodes       int `json:"max_nodes"`
	MaxParallel    int `json:"max_parallel"`
	MaxAttempts    int `json:"max_attempts"`
	MaxSteps       int `json:"max_steps"`
	PromptTokens   int `json:"prompt_tokens"`
	OutputTokens   int `json:"output_tokens"`
	ElapsedSeconds int `json:"elapsed_seconds"`
}

// CoordinationDecision is the smallest useful unit of dynamic orchestration:
// a coordinator can add work, retry a failed node, or explicitly wait for an
// external/operator condition. Every action is validated before persistence.
type CoordinationDecision struct {
	Reason  string               `json:"reason"`
	Actions []CoordinationAction `json:"actions"`
}

type CoordinationAction struct {
	Kind        string `json:"kind"` // add_node|retry_node|wait|cancel
	Node        *Node  `json:"node,omitempty"`
	NodeID      string `json:"node_id,omitempty"`
	Message     string `json:"message,omitempty"`
	RequestedBy string `json:"requested_by,omitempty"`
}

const (
	ActionAddNode   = "add_node"
	ActionRetryNode = "retry_node"
	ActionWait      = "wait"
	ActionCancel    = "cancel"
)

var nodeIDPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]{0,63}$`)

// ValidateDecision protects the durable graph from an overly creative model:
// no unknown actions, duplicate IDs, missing dependencies, write-capable plan
// roles, or unbounded fan-out may enter the scheduler.
func ValidateDecision(task Task, decision CoordinationDecision) error {
	if strings.TrimSpace(decision.Reason) == "" {
		return errors.New("coordination decision reason is required")
	}
	if len(decision.Actions) == 0 {
		return errors.New("coordination decision has no actions")
	}
	known := make(map[string]bool, len(task.Nodes))
	for _, n := range task.Nodes {
		known[n.ID] = true
	}
	added := map[string]bool{}
	for i, action := range decision.Actions {
		switch action.Kind {
		case ActionAddNode:
			if action.Node == nil {
				return fmt.Errorf("action %d: add_node requires node", i)
			}
			n := action.Node
			if !nodeIDPattern.MatchString(n.ID) {
				return fmt.Errorf("action %d: invalid node id %q", i, n.ID)
			}
			if known[n.ID] || added[n.ID] {
				return fmt.Errorf("action %d: duplicate node id %q", i, n.ID)
			}
			if n.Role == Plan && hasWriteCapability(*n) {
				return fmt.Errorf("action %d: plan node %q cannot request write capability", i, n.ID)
			}
			for _, dep := range n.DependsOn {
				if !known[dep] && !added[dep] {
					return fmt.Errorf("action %d: node %q depends on unknown node %q", i, n.ID, dep)
				}
			}
			added[n.ID] = true
		case ActionRetryNode:
			if !known[action.NodeID] {
				return fmt.Errorf("action %d: retry references unknown node %q", i, action.NodeID)
			}
		case ActionWait, ActionCancel:
			if strings.TrimSpace(action.Message) == "" {
				return fmt.Errorf("action %d: %s requires message", i, action.Kind)
			}
		default:
			return fmt.Errorf("action %d: unknown coordination action %q", i, action.Kind)
		}
	}
	if len(task.Nodes)+len(added) > 128 {
		return errors.New("coordination decision exceeds maximum graph size of 128 nodes")
	}
	return nil
}

// ApplyDecision is the durable mutation point for Coordinator output. The
// decision is validated in full before any graph mutation, so a malformed
// model response cannot leave half of a fan-out persisted.
func (s *Store) ApplyDecision(t *Task, decision CoordinationDecision) error {
	if t == nil {
		return errors.New("task is nil")
	}
	if err := ValidateDecision(*t, decision); err != nil {
		return err
	}
	for _, action := range decision.Actions {
		switch action.Kind {
		case ActionAddNode:
			n := *action.Node
			if n.Status == "" {
				n.Status = Pending
			}
			if n.MaxAttempts <= 0 {
				n.MaxAttempts = 2
			}
			t.Nodes = append(t.Nodes, n)
		case ActionRetryNode:
			for i := range t.Nodes {
				if t.Nodes[i].ID == action.NodeID {
					t.Nodes[i].Status = Pending
					t.Nodes[i].Error = ""
					t.Nodes[i].FinishedAt = nil
				}
			}
		case ActionWait:
			t.Status = Blocked
		case ActionCancel:
			t.Status = Cancelled
		}
	}
	return s.AppendEvent(t, Event{Type: "coordination_decision", Message: decision.Reason, Data: map[string]string{
		"actions": fmt.Sprintf("%d", len(decision.Actions)),
	}})
}

func hasWriteCapability(n Node) bool {
	for _, configured := range n.Metadata {
		for _, tool := range strings.FieldsFunc(configured, func(r rune) bool { return r == ',' || r == ' ' || r == ';' }) {
			if strings.EqualFold(tool, "write") || strings.EqualFold(tool, "write_file") || strings.EqualFold(tool, "patch") || strings.EqualFold(tool, "bash") {
				return true
			}
		}
	}
	return n.Role == Build || n.Role == Test
}
