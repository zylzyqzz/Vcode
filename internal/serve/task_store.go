package serve

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"vcode/internal/runtime"
)

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

type TaskRecord struct {
	ID                 string     `json:"id"`
	Goal               string     `json:"goal"`
	Mode               string     `json:"mode,omitempty"`
	Model              string     `json:"model,omitempty"`
	Workspace          string     `json:"workspace,omitempty"`
	SessionID          string     `json:"session_id,omitempty"`
	Agent              string     `json:"agent,omitempty"`
	Status             TaskStatus `json:"status"`
	Outcome            string     `json:"outcome,omitempty"`
	ErrorClass         string     `json:"error_class,omitempty"`
	Error              string     `json:"error,omitempty"`
	RetryCount         int        `json:"retry_count"`
	ToolCalls          int        `json:"tool_calls"`
	ToolFailures       int        `json:"tool_failures,omitempty"`
	UnresolvedFailures int        `json:"unresolved_failures,omitempty"`
	CurrentNode        string     `json:"current_node,omitempty"`
	LastSuccessfulNode string     `json:"last_successful_node,omitempty"`
	ModifiedFiles      []string   `json:"modified_files,omitempty"`
	VerificationStatus string     `json:"verification_status,omitempty"`
	FinalResponse      string     `json:"final_response,omitempty"`
	WritesCompleted    bool       `json:"writes_completed"`
	VerificationFresh  bool       `json:"verification_fresh"`
	EvidenceRecorded   bool       `json:"evidence_recorded"`
	DiffMatchesGoal    bool       `json:"diff_matches_goal"`
	BoundaryViolations int        `json:"boundary_violations,omitempty"`
	LastWriteAt        *time.Time `json:"last_write_at,omitempty"`
	LastVerificationAt *time.Time `json:"last_verification_at,omitempty"`
	LastEvent          uint64     `json:"last_event"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
	FinishedAt         *time.Time `json:"finished_at,omitempty"`
}

type taskEvent struct {
	Seq     uint64          `json:"seq"`
	TaskID  string          `json:"task_id"`
	At      time.Time       `json:"at"`
	Payload json.RawMessage `json:"payload"`
}

type taskAudit struct {
	At     time.Time         `json:"at"`
	TaskID string            `json:"task_id"`
	Action string            `json:"action"`
	Detail map[string]string `json:"detail,omitempty"`
}

type taskStore struct {
	mu     sync.Mutex
	root   string
	active *TaskRecord
}

func terminalTaskStatus(status TaskStatus) bool {
	switch status {
	case TaskCompleted, TaskPartial, TaskFailed, TaskBlocked, TaskCancelled:
		return true
	default:
		return false
	}
}

func newTaskStore(sessionDir string) *taskStore {
	return &taskStore{root: filepath.Join(sessionDir, ".tasks")}
}

func (s *taskStore) init() error {
	if s == nil || strings.TrimSpace(s.root) == "" {
		return nil
	}
	return os.MkdirAll(s.root, 0o700)
}

func (s *taskStore) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.init(); err != nil {
		return err
	}
	entries, err := os.ReadDir(s.root)
	if err != nil {
		return err
	}
	var newest *TaskRecord
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") || strings.HasSuffix(entry.Name(), ".tmp") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.root, entry.Name()))
		if err != nil {
			continue
		}
		var record TaskRecord
		if json.Unmarshal(data, &record) != nil || record.ID == "" {
			continue
		}
		if eventSeq, err := s.lastEventSeq(record.ID); err == nil && eventSeq > record.LastEvent {
			// Recover a snapshot that was interrupted after the event was flushed
			// but before the atomic task record replacement completed.
			record.LastEvent = eventSeq
			record.UpdatedAt = time.Now().UTC()
			if err := s.writeRecordLocked(record); err != nil {
				return err
			}
		}
		if !terminalTaskStatus(record.Status) && record.Status != TaskRecovering {
			// Never restore an in-flight task as if its process were still alive.
			// Every non-terminal record is made recoverable before the store is
			// exposed to the rest of the server.
			record.Status = TaskRecovering
			record.ErrorClass = "runtime_restart"
			record.Error = "task interrupted by runtime restart; recovery is pending"
			record.UpdatedAt = time.Now().UTC()
			if err := s.writeRecordLocked(record); err != nil {
				return err
			}
		}
		if newest == nil || record.UpdatedAt.After(newest.UpdatedAt) {
			copy := record
			newest = &copy
		}
	}
	s.active = newest
	return nil
}

func (s *taskStore) list() ([]TaskRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.init(); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(s.root)
	if err != nil {
		return nil, err
	}
	var records []TaskRecord
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") || strings.HasSuffix(entry.Name(), ".tmp") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.root, entry.Name()))
		if err != nil {
			continue
		}
		var record TaskRecord
		if json.Unmarshal(data, &record) == nil && record.ID != "" {
			records = append(records, record)
		}
	}
	return records, nil
}

func (s *taskStore) start(id, goal, mode, model, workspace, sessionID string) (TaskRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.init(); err != nil {
		return TaskRecord{}, err
	}
	now := time.Now().UTC()
	record := TaskRecord{ID: id, Goal: goal, Mode: mode, Model: model, Workspace: workspace, SessionID: sessionID, Agent: "Builder", Status: TaskQueued, CreatedAt: now, UpdatedAt: now}
	if err := s.writeRecordLocked(record); err != nil {
		return TaskRecord{}, err
	}
	s.active = &record
	return record, nil
}

func (s *taskStore) update(id string, status TaskStatus, errClass, message string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	record, err := s.recordLocked(id)
	if err != nil {
		return err
	}
	record.Status = status
	record.ErrorClass = errClass
	record.Error = message
	record.UpdatedAt = time.Now().UTC()
	if status == TaskCompleted || status == TaskPartial || status == TaskFailed || status == TaskBlocked || status == TaskCancelled {
		now := time.Now().UTC()
		record.FinishedAt = &now
	}
	if s.active != nil && s.active.ID == id {
		copy := *record
		s.active = &copy
	}
	return s.writeRecordLocked(*record)
}

func (s *taskStore) adoptPaused(id string) error {
	return s.update(id, TaskQueued, "", "")
}

func (s *taskStore) toolResult(id string, failed bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	record, err := s.recordLocked(id)
	if err != nil {
		return err
	}
	record.ToolCalls++
	if failed {
		record.ToolFailures++
		record.UnresolvedFailures++
	}
	record.UpdatedAt = time.Now().UTC()
	if s.active != nil && s.active.ID == id {
		copy := *record
		s.active = &copy
	}
	return s.writeRecordLocked(*record)
}

func (s *taskStore) setAgent(id, agent string, status TaskStatus) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	record, err := s.recordLocked(id)
	if err != nil {
		return err
	}
	if strings.TrimSpace(agent) != "" {
		record.Agent = agent
	}
	if status != "" {
		record.Status = status
	}
	record.UpdatedAt = time.Now().UTC()
	if s.active != nil && s.active.ID == id {
		copy := *record
		s.active = &copy
	}
	return s.writeRecordLocked(*record)
}

func (s *taskStore) markNodeSuccess(id, node, agent string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if strings.TrimSpace(node) == "" {
		return fmt.Errorf("node is required")
	}
	record, err := s.recordLocked(id)
	if err != nil {
		return err
	}
	if terminalTaskStatus(record.Status) {
		return fmt.Errorf("cannot complete node on terminal task %q", id)
	}
	if record.LastSuccessfulNode == node {
		return nil
	}
	record.CurrentNode = node
	record.LastSuccessfulNode = node
	if strings.TrimSpace(agent) != "" {
		record.Agent = agent
	}
	record.UpdatedAt = time.Now().UTC()
	if s.active != nil && s.active.ID == id {
		copy := *record
		s.active = &copy
	}
	return s.writeRecordLocked(*record)
}

func (s *taskStore) setVerification(id, status string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	record, err := s.recordLocked(id)
	if err != nil {
		return err
	}
	record.VerificationStatus = status
	now := time.Now().UTC()
	record.LastVerificationAt = &now
	record.EvidenceRecorded = strings.TrimSpace(status) != ""
	record.VerificationFresh = strings.EqualFold(strings.TrimSpace(status), "VERIFIED") &&
		(record.LastWriteAt == nil || !record.LastWriteAt.After(now))
	if record.VerificationFresh {
		// A successful verification after the last write is the evidence that
		// recovery handled failures from the previous attempt. Keep the total
		// failure count for audit, but do not block completion on resolved work.
		record.UnresolvedFailures = 0
	}
	record.DiffMatchesGoal = len(record.ModifiedFiles) > 0
	record.UpdatedAt = time.Now().UTC()
	if s.active != nil && s.active.ID == id {
		copy := *record
		s.active = &copy
	}
	return s.writeRecordLocked(*record)
}

// markToolStart records write intent before the tool runs. This gives the
// completion gate enough information to reject a successful-looking answer
// when no write ever happened, while keeping the raw tool event authoritative.
func (s *taskStore) markToolStart(id string, readOnly bool, args string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	record, err := s.recordLocked(id)
	if err != nil {
		return err
	}
	if !readOnly {
		record.WritesCompleted = true
		now := time.Now().UTC()
		record.LastWriteAt = &now
		for _, path := range toolPaths(args) {
			if path == "" || contains(record.ModifiedFiles, path) {
				continue
			}
			record.ModifiedFiles = append(record.ModifiedFiles, path)
		}
	}
	record.UpdatedAt = time.Now().UTC()
	if s.active != nil && s.active.ID == id {
		copy := *record
		s.active = &copy
	}
	return s.writeRecordLocked(*record)
}

func (s *taskStore) setFinalResponse(id, response string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	record, err := s.recordLocked(id)
	if err != nil {
		return err
	}
	record.FinalResponse = strings.TrimSpace(response)
	record.UpdatedAt = time.Now().UTC()
	if s.active != nil && s.active.ID == id {
		copy := *record
		s.active = &copy
	}
	return s.writeRecordLocked(*record)
}

func (s *taskStore) setModifiedFiles(id string, files []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	record, err := s.recordLocked(id)
	if err != nil {
		return err
	}
	record.ModifiedFiles = append([]string(nil), files...)
	record.DiffMatchesGoal = len(record.ModifiedFiles) > 0
	record.UpdatedAt = time.Now().UTC()
	if s.active != nil && s.active.ID == id {
		copy := *record
		s.active = &copy
	}
	return s.writeRecordLocked(*record)
}

// complete is the only production path that may promote a task to completed.
// The legacy update method remains for backwards-compatible state migrations
// and tests, but event consumers must call this gate.
func (s *taskStore) complete(id string) (runtime.CompletionDecision, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	record, err := s.recordLocked(id)
	if err != nil {
		return runtime.CompletionDecision{}, err
	}
	record.DiffMatchesGoal = record.DiffMatchesGoal && len(record.ModifiedFiles) > 0
	decision := runtime.EvaluateCompletion(runtime.CompletionInput{
		FinalResponse:      record.FinalResponse,
		ChangedFiles:       record.ModifiedFiles,
		VerificationStatus: record.VerificationStatus,
		VerificationFresh:  record.VerificationFresh,
		UnresolvedFailures: record.UnresolvedFailures,
		BoundaryViolations: record.BoundaryViolations,
		DiffMatchesGoal:    record.DiffMatchesGoal,
		EvidenceRecorded:   record.EvidenceRecorded,
		WritesCompleted:    record.WritesCompleted,
	})
	record.Status = TaskPartial
	record.Outcome = decision.Outcome
	if !decision.Allowed {
		record.ErrorClass = "completion_gate"
		record.Error = strings.Join(decision.Reasons, "; ")
	}
	record.UpdatedAt = time.Now().UTC()
	now := time.Now().UTC()
	record.FinishedAt = &now
	if decision.Allowed {
		record.Status = TaskCompleted
		record.ErrorClass = ""
		record.Error = ""
	}
	if s.active != nil && s.active.ID == id {
		copy := *record
		s.active = &copy
	}
	if err := s.writeRecordLocked(*record); err != nil {
		return runtime.CompletionDecision{}, err
	}
	return decision, nil
}

func toolPaths(args string) []string {
	var raw map[string]any
	if json.Unmarshal([]byte(args), &raw) != nil {
		return nil
	}
	var paths []string
	for _, key := range []string{"path", "file", "filename"} {
		if value, ok := raw[key].(string); ok && strings.TrimSpace(value) != "" {
			paths = append(paths, value)
		}
	}
	if values, ok := raw["paths"].([]any); ok {
		for _, value := range values {
			if path, ok := value.(string); ok && strings.TrimSpace(path) != "" {
				paths = append(paths, path)
			}
		}
	}
	return paths
}

func contains(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

func (s *taskStore) appendEvent(id string, payload []byte) (uint64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.init(); err != nil {
		return 0, err
	}
	record, err := s.recordLocked(id)
	if err != nil {
		return 0, err
	}
	seq := record.LastEvent + 1
	path := filepath.Join(s.root, id+".events.jsonl")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return 0, err
	}
	entry := taskEvent{Seq: seq, TaskID: id, At: time.Now().UTC(), Payload: append([]byte(nil), payload...)}
	data, err := json.Marshal(entry)
	if err != nil {
		_ = f.Close()
		return 0, err
	}
	if _, err := f.Write(append(data, '\n')); err != nil {
		_ = f.Close()
		return 0, err
	}
	// Persist the event before advancing the task snapshot. This ordering is
	// required for restart recovery and prevents a snapshot from advertising
	// a state change whose event was never durable.
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return 0, err
	}
	if err := f.Close(); err != nil {
		return 0, err
	}
	record.LastEvent = seq
	record.UpdatedAt = time.Now().UTC()
	if err := s.writeRecordLocked(*record); err != nil {
		return 0, err
	}
	if s.active != nil && s.active.ID == id {
		copy := *record
		s.active = &copy
	}
	return seq, nil
}

func (s *taskStore) activeID() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.active == nil {
		return ""
	}
	return s.active.ID
}

func (s *taskStore) activeRecord() *TaskRecord {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.active == nil {
		return nil
	}
	copy := *s.active
	return &copy
}

func (s *taskStore) activate(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	record, err := s.recordLocked(id)
	if err != nil {
		return err
	}
	s.active = record
	return nil
}

func (s *taskStore) record(id string) (*TaskRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.recordLocked(id)
}

func (s *taskStore) recordLocked(id string) (*TaskRecord, error) {
	if id == "" || filepath.Base(id) != id {
		return nil, fmt.Errorf("invalid task id")
	}
	data, err := os.ReadFile(filepath.Join(s.root, id+".json"))
	if err != nil {
		return nil, err
	}
	var record TaskRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return nil, err
	}
	return &record, nil
}

func (s *taskStore) events(id string, after uint64) ([]taskEvent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	path := filepath.Join(s.root, filepath.Base(id)+".events.jsonl")
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return []taskEvent{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []taskEvent
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var e taskEvent
		if json.Unmarshal(scanner.Bytes(), &e) == nil && e.Seq > after {
			out = append(out, e)
		}
	}
	return out, scanner.Err()
}

func (s *taskStore) lastEventSeq(id string) (uint64, error) {
	if id == "" || filepath.Base(id) != id {
		return 0, fmt.Errorf("invalid task id")
	}
	f, err := os.Open(filepath.Join(s.root, id+".events.jsonl"))
	if err != nil {
		return 0, err
	}
	defer f.Close()
	var last uint64
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var entry taskEvent
		if json.Unmarshal(scanner.Bytes(), &entry) == nil && entry.Seq > last {
			last = entry.Seq
		}
	}
	if err := scanner.Err(); err != nil {
		return 0, err
	}
	return last, nil
}

func (s *taskStore) audit(id, action string, detail map[string]string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if id == "" || filepath.Base(id) != id || strings.TrimSpace(action) == "" {
		return fmt.Errorf("invalid task audit entry")
	}
	if err := s.init(); err != nil {
		return err
	}
	entry := taskAudit{At: time.Now().UTC(), TaskID: id, Action: action, Detail: detail}
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(s.root, id+".audit.jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(append(data, '\n'))
	return err
}

func (s *taskStore) writeRecordLocked(record TaskRecord) error {
	path := filepath.Join(s.root, record.ID+".json")
	tmp := path + ".tmp"
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(tmp, append(data, '\n'), 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("replace task record: %w", err)
	}
	return nil
}
