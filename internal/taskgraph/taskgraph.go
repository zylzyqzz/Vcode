// Package taskgraph stores durable long-running coding tasks.
//
// The graph is intentionally independent from the CLI and Agent packages: a
// future TUI, ACP client, or headless runner can use the same state machine.
package taskgraph

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type Status string

const (
	Pending     Status = "pending"
	Ready       Status = "ready"
	Running     Status = "running"
	Succeeded   Status = "succeeded"
	Failed      Status = "failed"
	Blocked     Status = "blocked"
	Cancelled   Status = "cancelled"
	Interrupted Status = "interrupted"
)

type Role string

const (
	Plan    Role = "plan"
	Explore Role = "explore"
	Build   Role = "build"
	Test    Role = "test"
	Review  Role = "review"
)

type Task struct {
	ID          string    `json:"id"`
	Goal        string    `json:"goal"`
	Status      Status    `json:"status"`
	Outcome     string    `json:"outcome,omitempty"` // VERIFIED|PARTIAL|UNVERIFIED|BLOCKED
	ProjectRoot string    `json:"project_root"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Nodes       []Node    `json:"nodes"`
	Events      []Event   `json:"events,omitempty"`
}

type Node struct {
	ID           string            `json:"id"`
	Title        string            `json:"title"`
	Prompt       string            `json:"prompt"`
	Role         Role              `json:"role"`
	DependsOn    []string          `json:"depends_on,omitempty"`
	Status       Status            `json:"status"`
	Attempt      int               `json:"attempt"`
	MaxAttempts  int               `json:"max_attempts"`
	MaxSteps     int               `json:"max_steps,omitempty"`
	Model        string            `json:"model,omitempty"`
	Effort       string            `json:"effort,omitempty"`
	Workspace    string            `json:"workspace,omitempty"`
	SessionPath  string            `json:"session_path,omitempty"`
	Commit       string            `json:"commit,omitempty"`
	Integrated   bool              `json:"integrated,omitempty"`
	ChangedFiles []string          `json:"changed_files,omitempty"`
	Summary      string            `json:"summary,omitempty"`
	PromptTokens int               `json:"prompt_tokens,omitempty"`
	OutputTokens int               `json:"output_tokens,omitempty"`
	CachedTokens int               `json:"cached_tokens,omitempty"`
	Artifacts    []Artifact        `json:"artifacts,omitempty"`
	Verification *Verification     `json:"verification,omitempty"`
	Error        string            `json:"error,omitempty"`
	StartedAt    *time.Time        `json:"started_at,omitempty"`
	FinishedAt   *time.Time        `json:"finished_at,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

type Artifact struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Kind string `json:"kind,omitempty"`
}

type Verification struct {
	Status   string          `json:"status"`
	Evidence []CheckEvidence `json:"evidence,omitempty"`
	Passed   []string        `json:"passed,omitempty"`
	Failed   []string        `json:"failed,omitempty"`
	Skipped  string          `json:"skipped,omitempty"`
}

type CheckEvidence struct {
	Name       string `json:"name"`
	Command    string `json:"command"`
	Status     string `json:"status"`
	Output     string `json:"output,omitempty"`
	DurationMS int64  `json:"duration_ms"`
}

type Event struct {
	Type      string    `json:"type"`
	TaskID    string    `json:"task_id"`
	NodeID    string    `json:"node_id,omitempty"`
	Role      Role      `json:"role,omitempty"`
	Message   string    `json:"message,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

type Store struct {
	root string
	mu   sync.Mutex
}

func NewStore(projectRoot string) *Store {
	return &Store{root: filepath.Join(projectRoot, ".vcode", "tasks")}
}

func (s *Store) Root() string { return s.root }

func (s *Store) Create(goal, projectRoot string, nodes []Node) (Task, error) {
	goal = strings.TrimSpace(goal)
	if goal == "" {
		return Task{}, errors.New("task goal is required")
	}
	if projectRoot == "" {
		projectRoot = "."
	}
	now := time.Now().UTC()
	t := Task{ID: now.Format("20060102-150405.000000000"), Goal: goal, Status: Ready, ProjectRoot: projectRoot, CreatedAt: now, UpdatedAt: now, Nodes: normalizeNodes(nodes)}
	if err := validate(t); err != nil {
		return Task{}, err
	}
	if err := s.Save(t); err != nil {
		return Task{}, err
	}
	return t, nil
}

func normalizeNodes(nodes []Node) []Node {
	out := append([]Node(nil), nodes...)
	for i := range out {
		if out[i].ID == "" {
			out[i].ID = fmt.Sprintf("node-%02d", i+1)
		}
		if out[i].Status == "" {
			out[i].Status = Pending
		}
		if out[i].MaxAttempts <= 0 {
			out[i].MaxAttempts = 2
		}
	}
	return out
}

func validate(t Task) error {
	seen := map[string]bool{}
	for _, n := range t.Nodes {
		if n.ID == "" || seen[n.ID] {
			return fmt.Errorf("invalid or duplicate node id %q", n.ID)
		}
		seen[n.ID] = true
	}
	for _, n := range t.Nodes {
		for _, dep := range n.DependsOn {
			if !seen[dep] {
				return fmt.Errorf("node %q depends on missing node %q", n.ID, dep)
			}
			if dep == n.ID {
				return fmt.Errorf("node %q cannot depend on itself", n.ID)
			}
		}
	}
	return nil
}

func (s *Store) path(id string) string { return filepath.Join(s.root, id+".json") }

func (s *Store) Save(t Task) error {
	if err := validate(t); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.MkdirAll(s.root, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(s.root, ".task-*.tmp")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(name, s.path(t.ID))
}

func (s *Store) Get(id string) (Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := os.ReadFile(s.path(strings.TrimSpace(id)))
	if err != nil {
		return Task{}, err
	}
	var t Task
	if err := json.Unmarshal(data, &t); err != nil {
		return Task{}, err
	}
	return t, nil
}

func (s *Store) List() ([]Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := os.ReadDir(s.root)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	out := []Task{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.root, e.Name()))
		if err != nil {
			return nil, err
		}
		var t Task
		if json.Unmarshal(data, &t) == nil {
			out = append(out, t)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt.After(out[j].UpdatedAt) })
	return out, nil
}

func (s *Store) AppendEvent(t *Task, e Event) error {
	if t == nil {
		return errors.New("task is nil")
	}
	e.TaskID = t.ID
	e.Timestamp = time.Now().UTC()
	t.Events = append(t.Events, e)
	t.UpdatedAt = e.Timestamp
	return s.Save(*t)
}

func ReadyNodes(t Task) []Node {
	byID := make(map[string]Node, len(t.Nodes))
	for _, n := range t.Nodes {
		byID[n.ID] = n
	}
	ready := []Node{}
	for _, n := range t.Nodes {
		if n.Status != Pending && n.Status != Ready && n.Status != Interrupted {
			continue
		}
		ok := true
		for _, dep := range n.DependsOn {
			if byID[dep].Status != Succeeded {
				ok = false
				break
			}
		}
		if ok {
			ready = append(ready, n)
		}
	}
	return ready
}

func (s *Store) RecoverInterrupted(t *Task) error {
	if t == nil {
		return errors.New("task is nil")
	}
	for i := range t.Nodes {
		if t.Nodes[i].Status == Running {
			t.Nodes[i].Status = Interrupted
			t.Nodes[i].Error = "process interrupted before node completion"
		}
	}
	return s.Save(*t)
}

func (s *Store) UpdateNode(t *Task, nodeID string, status Status, message string) error {
	if t == nil {
		return errors.New("task is nil")
	}
	for i := range t.Nodes {
		if t.Nodes[i].ID != nodeID {
			continue
		}
		now := time.Now().UTC()
		t.Nodes[i].Status = status
		if status == Failed || status == Cancelled || status == Interrupted {
			t.Nodes[i].Error = message
		} else if status == Pending || status == Ready || status == Running || status == Succeeded {
			t.Nodes[i].Error = ""
		}
		if status == Running {
			t.Nodes[i].Attempt++
			t.Nodes[i].StartedAt = &now
		}
		if status == Pending || status == Ready || status == Running {
			t.Nodes[i].FinishedAt = nil
		}
		if status == Succeeded || status == Failed || status == Cancelled {
			t.Nodes[i].FinishedAt = &now
		}
		return s.AppendEvent(t, Event{Type: "node_" + string(status), NodeID: nodeID, Role: t.Nodes[i].Role, Message: message})
	}
	return fmt.Errorf("node %q not found", nodeID)
}

func (s *Store) SetStatus(t *Task, status Status, message string) error {
	if t == nil {
		return errors.New("task is nil")
	}
	t.Status = status
	return s.AppendEvent(t, Event{Type: "task_" + string(status), Message: message})
}
