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
)

type TaskStatus string

const (
	TaskQueued            TaskStatus = "queued"
	TaskRunning           TaskStatus = "running"
	TaskWaitingPermission TaskStatus = "waiting_permission"
	TaskVerifying         TaskStatus = "verifying"
	TaskPaused            TaskStatus = "paused"
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
	ErrorClass         string     `json:"error_class,omitempty"`
	Error              string     `json:"error,omitempty"`
	RetryCount         int        `json:"retry_count"`
	ToolCalls          int        `json:"tool_calls"`
	ToolFailures       int        `json:"tool_failures,omitempty"`
	ModifiedFiles      []string   `json:"modified_files,omitempty"`
	VerificationStatus string     `json:"verification_status,omitempty"`
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
		if newest == nil || record.UpdatedAt.After(newest.UpdatedAt) {
			copy := record
			newest = &copy
		}
	}
	if newest != nil && !terminalTaskStatus(newest.Status) {
		// A process restart cannot safely pretend an in-flight turn is still
		// running. Keep the durable record visible and make recovery explicit.
		newest.Status = TaskPaused
		newest.ErrorClass = "runtime_restart"
		newest.Error = "任务运行时已重启，请恢复后继续"
		newest.UpdatedAt = time.Now().UTC()
		if err := s.writeRecordLocked(*newest); err != nil {
			return err
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

func (s *taskStore) setVerification(id, status string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	record, err := s.recordLocked(id)
	if err != nil {
		return err
	}
	record.VerificationStatus = status
	record.UpdatedAt = time.Now().UTC()
	if s.active != nil && s.active.ID == id {
		copy := *record
		s.active = &copy
	}
	return s.writeRecordLocked(*record)
}

func (s *taskStore) appendEvent(id string, payload []byte) (uint64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.init(); err != nil {
		return 0, err
	}
	seq := uint64(1)
	if record, err := s.recordLocked(id); err == nil {
		seq = record.LastEvent + 1
		record.LastEvent = seq
		record.UpdatedAt = time.Now().UTC()
		if err := s.writeRecordLocked(*record); err != nil {
			return 0, err
		}
		if s.active != nil && s.active.ID == id {
			copy := *record
			s.active = &copy
		}
	} else if s.active == nil || s.active.ID != id {
		return 0, err
	}
	path := filepath.Join(s.root, id+".events.jsonl")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	entry := taskEvent{Seq: seq, TaskID: id, At: time.Now().UTC(), Payload: append([]byte(nil), payload...)}
	data, err := json.Marshal(entry)
	if err != nil {
		return 0, err
	}
	if _, err := f.Write(append(data, '\n')); err != nil {
		return 0, err
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
