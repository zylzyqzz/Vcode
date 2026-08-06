package runtime

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// TaskStatus is the only lifecycle vocabulary used by local, cloud and
// bridge runtimes. Status is changed by the host, never by model output.
type TaskStatus string

const (
	TaskQueued            TaskStatus = "queued"
	TaskRunning           TaskStatus = "running"
	TaskWaitingPermission TaskStatus = "waiting_permission"
	TaskVerifying         TaskStatus = "verifying"
	TaskPaused            TaskStatus = "paused"
	TaskRecovering        TaskStatus = "recovering"
	TaskCompleted         TaskStatus = "completed"
	TaskPartial           TaskStatus = "partial"
	TaskFailed            TaskStatus = "failed"
	TaskBlocked           TaskStatus = "blocked"
	TaskCancelled         TaskStatus = "cancelled"
)

type Task struct {
	ID                 string     `json:"id"`
	Goal               string     `json:"goal"`
	Mode               string     `json:"mode,omitempty"`
	Model              string     `json:"model,omitempty"`
	TargetID           string     `json:"target_id,omitempty"`
	Workspace          string     `json:"workspace"`
	SessionID          string     `json:"session_id,omitempty"`
	CurrentNode        string     `json:"current_node,omitempty"`
	CurrentAgent       string     `json:"current_agent,omitempty"`
	Status             TaskStatus `json:"status"`
	Outcome            string     `json:"outcome,omitempty"`
	RetryCount         int        `json:"retry_count"`
	LastEventSeq       uint64     `json:"last_event_seq"`
	LastSuccessfulNode string     `json:"last_successful_node,omitempty"`
	ModifiedFiles      []string   `json:"modified_files,omitempty"`
	UnresolvedFailures int        `json:"unresolved_failures"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
	FinishedAt         *time.Time `json:"finished_at,omitempty"`
}

type Event struct {
	EventID   string          `json:"event_id"`
	Seq       uint64          `json:"seq"`
	TaskID    string          `json:"task_id"`
	SessionID string          `json:"session_id,omitempty"`
	Type      string          `json:"type"`
	Timestamp time.Time       `json:"timestamp"`
	Payload   json.RawMessage `json:"payload,omitempty"`
}

type Store struct {
	root string
	mu   sync.Mutex
}

func NewStore(root string) (*Store, error) {
	if strings.TrimSpace(root) == "" {
		return nil, errors.New("runtime store root is required")
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return nil, fmt.Errorf("create runtime store: %w", err)
	}
	return &Store{root: root}, nil
}

func (s *Store) Create(task Task) (Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := validateTask(task); err != nil {
		return Task{}, err
	}
	if task.ID == "" {
		task.ID = newID()
	}
	now := time.Now().UTC()
	task.Status = TaskQueued
	task.CreatedAt = now
	task.UpdatedAt = now
	if err := os.MkdirAll(s.taskDir(task.ID), 0o700); err != nil {
		return Task{}, fmt.Errorf("create task directory: %w", err)
	}
	if _, err := os.Stat(s.snapshotPath(task.ID)); err == nil {
		return Task{}, fmt.Errorf("task %q already exists", task.ID)
	} else if !os.IsNotExist(err) {
		return Task{}, err
	}
	if err := writeJSONAtomic(s.snapshotPath(task.ID), task); err != nil {
		return Task{}, err
	}
	return task, nil
}

func (s *Store) Load(id string) (Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loadLocked(id)
}

// List returns every durable task without selecting an "active" task. The
// caller can choose which task to resume; persistence never silently switches
// another task's state.
func (s *Store) List() ([]Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := os.ReadDir(s.root)
	if err != nil {
		return nil, err
	}
	var tasks []Task
	for _, entry := range entries {
		if !entry.IsDir() || strings.TrimSpace(entry.Name()) == "" {
			continue
		}
		task, err := s.loadLocked(entry.Name())
		if err == nil {
			tasks = append(tasks, task)
		}
	}
	return tasks, nil
}

// MarkNodeSuccess records an idempotent node boundary. Replaying a node that
// already succeeded is a no-op, which is the key property needed after a
// process or network restart.
func (s *Store) MarkNodeSuccess(id, node, agent string) (Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if strings.TrimSpace(node) == "" {
		return Task{}, errors.New("node id is required")
	}
	task, err := s.loadLocked(id)
	if err != nil {
		return Task{}, err
	}
	if task.LastSuccessfulNode == node {
		return task, nil
	}
	if terminal(task.Status) {
		return Task{}, fmt.Errorf("cannot complete node on terminal task %q", id)
	}
	task.CurrentNode = node
	task.LastSuccessfulNode = node
	if strings.TrimSpace(agent) != "" {
		task.CurrentAgent = agent
	}
	task.UpdatedAt = time.Now().UTC()
	if _, err := s.appendEventLocked(&task, "node_completed", map[string]string{"node": node, "agent": agent}); err != nil {
		return Task{}, err
	}
	if err := writeJSONAtomic(s.snapshotPath(id), task); err != nil {
		return Task{}, err
	}
	return task, nil
}

func (s *Store) Transition(id string, next TaskStatus, outcome, reason string) (Task, Event, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	task, err := s.loadLocked(id)
	if err != nil {
		return Task{}, Event{}, err
	}
	if !validTransition(task.Status, next) {
		return Task{}, Event{}, fmt.Errorf("invalid task transition %s -> %s", task.Status, next)
	}
	if next == TaskCompleted {
		return Task{}, Event{}, errors.New("completed status requires the completion gate")
	}
	task.Status = next
	if outcome != "" {
		task.Outcome = outcome
	}
	task.UpdatedAt = time.Now().UTC()
	if reason != "" {
		if task.UnresolvedFailures == 0 && next == TaskFailed {
			task.UnresolvedFailures = 1
		}
	}
	if terminal(next) {
		now := time.Now().UTC()
		task.FinishedAt = &now
	}
	e, err := s.appendEventLocked(&task, "task_"+string(next), map[string]string{"reason": reason, "outcome": outcome})
	if err != nil {
		return Task{}, Event{}, err
	}
	if err := writeJSONAtomic(s.snapshotPath(id), task); err != nil {
		return Task{}, Event{}, err
	}
	return task, e, nil
}

// Complete is the only runtime API that can promote a task to completed. The
// caller must provide the host-collected evidence; model text alone is never
// enough. A rejected completion is persisted as partial so the task remains
// inspectable and recoverable instead of being silently discarded.
func (s *Store) Complete(id string, input CompletionInput) (Task, CompletionDecision, Event, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	task, err := s.loadLocked(id)
	if err != nil {
		return Task{}, CompletionDecision{}, Event{}, err
	}
	if terminal(task.Status) {
		return Task{}, CompletionDecision{}, Event{}, fmt.Errorf("task %q is already terminal", id)
	}
	decision := EvaluateCompletion(input)
	task.Status = TaskPartial
	task.Outcome = decision.Outcome
	task.UnresolvedFailures = input.UnresolvedFailures
	if decision.Allowed {
		task.Status = TaskCompleted
	}
	task.UpdatedAt = time.Now().UTC()
	if terminal(task.Status) {
		now := time.Now().UTC()
		task.FinishedAt = &now
	}
	reason := strings.Join(decision.Reasons, "; ")
	eventType := "task_partial"
	if decision.Allowed {
		eventType = "task_completed"
	}
	e, err := s.appendEventLocked(&task, eventType, map[string]string{
		"outcome": decision.Outcome,
		"reason":  reason,
	})
	if err != nil {
		return Task{}, CompletionDecision{}, Event{}, err
	}
	if err := writeJSONAtomic(s.snapshotPath(id), task); err != nil {
		return Task{}, CompletionDecision{}, Event{}, err
	}
	return task, decision, e, nil
}

func (s *Store) AppendEvent(id, typ string, payload any) (Event, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	task, err := s.loadLocked(id)
	if err != nil {
		return Event{}, err
	}
	e, err := s.appendEventLocked(&task, typ, payload)
	if err != nil {
		return Event{}, err
	}
	task.UpdatedAt = e.Timestamp
	if err := writeJSONAtomic(s.snapshotPath(id), task); err != nil {
		return Event{}, err
	}
	return e, nil
}

func (s *Store) EventsAfter(id string, after uint64) ([]Event, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, err := os.Open(s.eventsPath(id))
	if os.IsNotExist(err) {
		return []Event{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []Event
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		var e Event
		if err := json.Unmarshal(scanner.Bytes(), &e); err != nil {
			return nil, fmt.Errorf("decode event journal: %w", err)
		}
		if e.Seq > after {
			out = append(out, e)
		}
	}
	return out, scanner.Err()
}

func (s *Store) appendEventLocked(task *Task, typ string, payload any) (Event, error) {
	if strings.TrimSpace(typ) == "" {
		return Event{}, errors.New("event type is required")
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return Event{}, fmt.Errorf("encode event payload: %w", err)
	}
	e := Event{EventID: task.ID + ":" + fmt.Sprint(task.LastEventSeq+1), Seq: task.LastEventSeq + 1, TaskID: task.ID, SessionID: task.SessionID, Type: typ, Timestamp: time.Now().UTC(), Payload: data}
	line, err := json.Marshal(e)
	if err != nil {
		return Event{}, err
	}
	f, err := os.OpenFile(s.eventsPath(task.ID), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return Event{}, fmt.Errorf("open event journal: %w", err)
	}
	if _, err := f.Write(append(line, '\n')); err != nil {
		_ = f.Close()
		return Event{}, fmt.Errorf("append event journal: %w", err)
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return Event{}, fmt.Errorf("flush event journal: %w", err)
	}
	if err := f.Close(); err != nil {
		return Event{}, err
	}
	task.LastEventSeq = e.Seq
	return e, nil
}

func (s *Store) taskDir(id string) string      { return filepath.Join(s.root, id) }
func (s *Store) snapshotPath(id string) string { return filepath.Join(s.taskDir(id), "manifest.json") }
func (s *Store) eventsPath(id string) string   { return filepath.Join(s.taskDir(id), "events.jsonl") }

func (s *Store) loadLocked(id string) (Task, error) {
	if filepath.Base(id) != id || strings.TrimSpace(id) == "" {
		return Task{}, errors.New("invalid task id")
	}
	data, err := os.ReadFile(s.snapshotPath(id))
	if err != nil {
		return Task{}, err
	}
	var task Task
	if err := json.Unmarshal(data, &task); err != nil {
		return Task{}, fmt.Errorf("decode task manifest: %w", err)
	}
	if eventSeq, err := s.lastEventSeqLocked(id); err != nil {
		return Task{}, err
	} else if eventSeq > task.LastEventSeq {
		// The event may have been flushed immediately before a process died
		// during manifest replacement. Repair the sequence before callers append
		// another event, otherwise the next event could reuse an ID.
		task.LastEventSeq = eventSeq
		task.UpdatedAt = time.Now().UTC()
		if err := writeJSONAtomic(s.snapshotPath(id), task); err != nil {
			return Task{}, err
		}
	}
	return task, nil
}

func (s *Store) lastEventSeqLocked(id string) (uint64, error) {
	f, err := os.Open(s.eventsPath(id))
	if os.IsNotExist(err) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	defer f.Close()
	var last uint64
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		var event Event
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			return 0, fmt.Errorf("decode event journal: %w", err)
		}
		if event.Seq > last {
			last = event.Seq
		}
	}
	if err := scanner.Err(); err != nil {
		return 0, err
	}
	return last, nil
}

func validateTask(task Task) error {
	if task.ID != "" && (filepath.Base(task.ID) != task.ID || strings.TrimSpace(task.ID) == "") {
		return errors.New("invalid task id")
	}
	if strings.TrimSpace(task.Goal) == "" {
		return errors.New("task goal is required")
	}
	if filepath.Clean(task.Workspace) == "." || strings.TrimSpace(task.Workspace) == "" {
		return errors.New("task workspace is required")
	}
	return nil
}

func validTransition(from, to TaskStatus) bool {
	if from == to {
		return true
	}
	if terminal(from) {
		return false
	}
	switch from {
	case TaskQueued:
		return to == TaskRunning || to == TaskCancelled || to == TaskBlocked
	case TaskRunning:
		return to == TaskWaitingPermission || to == TaskVerifying || to == TaskPaused || to == TaskRecovering || to == TaskCompleted || to == TaskPartial || to == TaskFailed || to == TaskBlocked || to == TaskCancelled
	case TaskWaitingPermission:
		return to == TaskRunning || to == TaskPaused || to == TaskCancelled || to == TaskBlocked
	case TaskVerifying:
		return to == TaskRunning || to == TaskRecovering || to == TaskCompleted || to == TaskPartial || to == TaskFailed || to == TaskBlocked || to == TaskCancelled
	case TaskPaused:
		return to == TaskQueued || to == TaskRunning || to == TaskCancelled
	case TaskRecovering:
		return to == TaskRunning || to == TaskVerifying || to == TaskPartial || to == TaskFailed || to == TaskBlocked || to == TaskCancelled
	default:
		return false
	}
}

func terminal(status TaskStatus) bool {
	return status == TaskCompleted || status == TaskPartial || status == TaskFailed || status == TaskBlocked || status == TaskCancelled
}

func writeJSONAtomic(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("replace %s: %w", path, err)
	}
	return nil
}

func newID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("task-%d", time.Now().UnixNano())
	}
	return "task-" + hex.EncodeToString(b)
}
