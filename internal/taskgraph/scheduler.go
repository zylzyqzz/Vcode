package taskgraph

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// NodeRunner is the boundary between durable orchestration and an Agent role.
// The scheduler owns status, retries, and dependencies; the runner owns model
// calls, tools, and role-specific execution.
type NodeRunner func(context.Context, Node) NodeResult

type NodeResult struct {
	Workspace    string
	Commit       string
	ChangedFiles []string
	Summary      string
	PromptTokens int
	OutputTokens int
	CachedTokens int
	Artifacts    []Artifact
	Verification *Verification
	Message      string
	Err          error
}

type Scheduler struct {
	Store        *Store
	MaxParallel  int
	DefaultRetry int
	OnEvent      func(Event)
}

func (s Scheduler) emit(e Event) {
	if s.OnEvent != nil {
		s.OnEvent(e)
	}
}

func (s Scheduler) Run(ctx context.Context, task *Task, runner NodeRunner) error {
	if task == nil || runner == nil {
		return errors.New("task and node runner are required")
	}
	if s.Store == nil {
		return errors.New("task store is required")
	}
	maxParallel := s.MaxParallel
	if maxParallel <= 0 {
		maxParallel = 4
	}
	if s.DefaultRetry <= 0 {
		s.DefaultRetry = 2
	}
	if task.Status == Succeeded && allSucceeded(*task) {
		if task.Outcome == "" {
			task.Outcome = aggregateOutcome(*task)
		}
		return nil
	}
	if task.Status == "" || task.Status == Interrupted {
		if err := s.Store.RecoverInterrupted(task); err != nil {
			return err
		}
	}
	if err := s.Store.SetStatus(task, Running, "scheduler started"); err != nil {
		return err
	}

	for {
		if err := ctx.Err(); err != nil {
			_ = s.Store.SetStatus(task, Interrupted, err.Error())
			return err
		}
		ready := ReadyNodes(*task)
		if len(ready) == 0 {
			if allSucceeded(*task) {
				task.Outcome = aggregateOutcome(*task)
				return s.Store.SetStatus(task, Succeeded, "all nodes completed")
			}
			if hasFailed(*task) {
				task.Outcome = "PARTIAL"
				err := errors.New("one or more nodes exhausted retries")
				_ = s.Store.SetStatus(task, Failed, err.Error())
				return err
			}
			return s.Store.SetStatus(task, Blocked, "no runnable nodes; dependency or operator action required")
		}

		if len(ready) > maxParallel {
			ready = ready[:maxParallel]
		}
		results := make(chan nodeRunResult, len(ready))
		var wg sync.WaitGroup
		for _, node := range ready {
			node = inheritWorkspace(*task, node)
			if err := s.Store.UpdateNode(task, node.ID, Running, "scheduled"); err != nil {
				return err
			}
			s.emit(Event{Type: "node_started", TaskID: task.ID, NodeID: node.ID, Role: node.Role, Message: node.Title})
			wg.Add(1)
			go func(n Node) {
				defer wg.Done()
				results <- nodeRunResult{id: n.ID, result: runner(ctx, n)}
			}(node)
		}
		wg.Wait()
		close(results)
		for result := range results {
			if err := s.applyResult(task, result); err != nil {
				return err
			}
		}
	}
}

type nodeRunResult struct {
	id     string
	result NodeResult
}

func (s Scheduler) applyResult(task *Task, rr nodeRunResult) error {
	for i := range task.Nodes {
		n := &task.Nodes[i]
		if n.ID != rr.id {
			continue
		}
		n.ChangedFiles = rr.result.ChangedFiles
		n.Summary = rr.result.Summary
		n.PromptTokens = rr.result.PromptTokens
		n.OutputTokens = rr.result.OutputTokens
		n.CachedTokens = rr.result.CachedTokens
		n.Commit = rr.result.Commit
		n.Artifacts = rr.result.Artifacts
		n.Verification = rr.result.Verification
		n.Error = ""
		if rr.result.Workspace != "" {
			n.Workspace = rr.result.Workspace
		}
		if rr.result.Err == nil {
			s.emit(Event{Type: "node_succeeded", TaskID: task.ID, NodeID: n.ID, Role: n.Role, Message: rr.result.Message})
			return s.Store.UpdateNode(task, n.ID, Succeeded, rr.result.Message)
		}
		n.Error = rr.result.Err.Error()
		maxAttempts := n.MaxAttempts
		if maxAttempts <= 0 {
			maxAttempts = s.DefaultRetry
		}
		if n.Attempt < maxAttempts {
			n.Status = Pending
			s.emit(Event{Type: "node_retrying", TaskID: task.ID, NodeID: n.ID, Role: n.Role, Message: n.Error})
			return s.Store.AppendEvent(task, Event{Type: "node_retrying", NodeID: n.ID, Role: n.Role, Message: fmt.Sprintf("attempt %d/%d: %s", n.Attempt, maxAttempts, n.Error)})
		}
		s.emit(Event{Type: "node_failed", TaskID: task.ID, NodeID: n.ID, Role: n.Role, Message: n.Error})
		return s.Store.UpdateNode(task, n.ID, Failed, n.Error)
	}
	return fmt.Errorf("scheduler result references unknown node %q", rr.id)
}

func inheritWorkspace(t Task, node Node) Node {
	if node.Workspace != "" {
		return node
	}
	for _, depID := range node.DependsOn {
		for _, candidate := range t.Nodes {
			if candidate.ID == depID && candidate.Workspace != "" {
				node.Workspace = candidate.Workspace
				return node
			}
		}
	}
	return node
}

func allSucceeded(t Task) bool {
	if len(t.Nodes) == 0 {
		return true
	}
	for _, n := range t.Nodes {
		if n.Status != Succeeded {
			return false
		}
	}
	return true
}

func hasFailed(t Task) bool {
	for _, n := range t.Nodes {
		if n.Status == Failed || n.Status == Cancelled {
			return true
		}
	}
	return false
}

func aggregateOutcome(t Task) string {
	if len(t.Nodes) == 0 {
		return "UNVERIFIED"
	}
	for _, n := range t.Nodes {
		if n.Role == Plan || n.Role == Explore || n.Role == Review {
			continue
		}
		if n.Verification == nil || n.Verification.Status == "" || n.Verification.Status == "UNVERIFIED" {
			return "UNVERIFIED"
		}
		if n.Verification.Status == "PARTIAL" {
			return "PARTIAL"
		}
	}
	for _, n := range t.Nodes {
		if n.Role == Build || n.Role == Test {
			return "VERIFIED"
		}
	}
	return "UNVERIFIED"
}

// MarkNodeVerification is a small adapter for role runners that complete their
// work before the scheduler applies the result. It keeps timestamps in UTC.
func MarkNodeVerification(n *Node, v Verification) {
	if n == nil {
		return
	}
	n.Verification = &v
	n.FinishedAt = func() *time.Time { now := time.Now().UTC(); return &now }()
}
