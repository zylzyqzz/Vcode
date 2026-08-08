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
	"vcode/internal/verify"
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
	ID                   string            `json:"id"`
	Goal                 string            `json:"goal"`
	Mode                 string            `json:"mode,omitempty"`
	Model                string            `json:"model,omitempty"`
	Workspace            string            `json:"workspace,omitempty"`
	SessionID            string            `json:"session_id,omitempty"`
	IdempotencyKey       string            `json:"idempotency_key,omitempty"`
	Agent                string            `json:"agent,omitempty"`
	Status               TaskStatus        `json:"status"`
	Outcome              string            `json:"outcome,omitempty"`
	ErrorClass           string            `json:"error_class,omitempty"`
	Error                string            `json:"error,omitempty"`
	RetryCount           int               `json:"retry_count"`
	ToolCalls            int               `json:"tool_calls"`
	ToolFailures         int               `json:"tool_failures,omitempty"`
	UnresolvedFailures   int               `json:"unresolved_failures,omitempty"`
	CurrentNode          string            `json:"current_node,omitempty"`
	LastSuccessfulNode   string            `json:"last_successful_node,omitempty"`
	ModifiedFiles        []string          `json:"modified_files,omitempty"`
	VerificationStatus   string            `json:"verification_status,omitempty"`
	VerificationChecks   []verify.Check    `json:"verification_checks,omitempty"`
	VerificationEvidence []verify.Evidence `json:"verification_evidence,omitempty"`
	VerificationFailed   []string          `json:"verification_failed,omitempty"`
	VerificationSkipped  string            `json:"verification_skipped,omitempty"`
	FinalResponse        string            `json:"final_response,omitempty"`
	WritesCompleted      bool              `json:"writes_completed"`
	VerificationFresh    bool              `json:"verification_fresh"`
	EvidenceRecorded     bool              `json:"evidence_recorded"`
	DiffMatchesGoal      bool              `json:"diff_matches_goal"`
	PendingWriteFiles    []string          `json:"pending_write_files,omitempty"`
	PendingReadOnlyIDs   []string          `json:"pending_read_only_ids,omitempty"`
	BoundaryViolations   int               `json:"boundary_violations,omitempty"`
	LastWriteAt          *time.Time        `json:"last_write_at,omitempty"`
	LastVerificationAt   *time.Time        `json:"last_verification_at,omitempty"`
	LastEvent            uint64            `json:"last_event"`
	CreatedAt            time.Time         `json:"created_at"`
	UpdatedAt            time.Time         `json:"updated_at"`
	FinishedAt           *time.Time        `json:"finished_at,omitempty"`
}

type taskEvent = RuntimeEvent

type taskAudit struct {
	At     time.Time         `json:"at"`
	TaskID string            `json:"task_id"`
	Action string            `json:"action"`
	Detail map[string]string `json:"detail,omitempty"`
}

type taskStore struct {
	mu       sync.Mutex
	root     string
	latestID string
}

func terminalTaskStatus(status TaskStatus) bool {
	switch status {
	case TaskCompleted, TaskPartial, TaskFailed, TaskBlocked, TaskCancelled:
		return true
	default:
		return false
	}
}

// verificationErrorClass keeps an unavailable verifier distinct from an
// actual failed check. A conversational turn or a workspace without a
// recognized project manifest is not the same thing as a test failure.
func verificationErrorClass(result verify.Result) string {
	if result.Status == verify.Unverified && strings.TrimSpace(result.Skipped) != "" {
		return "verification_unavailable"
	}
	return "verification_failed"
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
	if newest != nil {
		s.latestID = newest.ID
	}
	return nil
}

func (s *taskStore) list() ([]TaskRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.listLocked()
}

func (s *taskStore) listLocked() ([]TaskRecord, error) {
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
	return s.startWithKey(id, goal, mode, model, workspace, sessionID, "")
}

func (s *taskStore) startWithKey(id, goal, mode, model, workspace, sessionID, idempotencyKey string) (TaskRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.init(); err != nil {
		return TaskRecord{}, err
	}
	if existing, err := s.findByIdempotencyLocked(idempotencyKey); err != nil {
		return TaskRecord{}, err
	} else if existing != nil {
		return *existing, nil
	}
	now := time.Now().UTC()
	record := TaskRecord{ID: id, Goal: goal, Mode: mode, Model: model, Workspace: workspace, SessionID: sessionID, IdempotencyKey: idempotencyKey, Agent: "Builder", Status: TaskQueued, CreatedAt: now, UpdatedAt: now}
	if err := s.writeRecordLocked(record); err != nil {
		return TaskRecord{}, err
	}
	s.latestID = record.ID
	return record, nil
}

func (s *taskStore) findByIdempotency(key string) (*TaskRecord, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return nil, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.findByIdempotencyLocked(key)
}

func (s *taskStore) findByIdempotencyLocked(key string) (*TaskRecord, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return nil, nil
	}
	records, err := s.listLocked()
	if err != nil {
		return nil, err
	}
	for _, record := range records {
		if record.IdempotencyKey == key {
			copy := record
			return &copy, nil
		}
	}
	return nil, nil
}

func (s *taskStore) pausedForSession(sessionID string) (*TaskRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	records, err := s.listLocked()
	if err != nil {
		return nil, err
	}
	var newest *TaskRecord
	for i := range records {
		candidate := records[i]
		if candidate.Status != TaskPaused || candidate.SessionID != sessionID {
			continue
		}
		if newest == nil || candidate.UpdatedAt.After(newest.UpdatedAt) {
			copy := candidate
			newest = &copy
		}
	}
	return newest, nil
}

func (s *taskStore) update(id string, status TaskStatus, errClass, message string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if status == TaskCompleted {
		return fmt.Errorf("completed status requires the completion gate")
	}
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
		record.ErrorClass = "tool_failure"
		record.Error = "tool execution failed"
	}
	record.UpdatedAt = time.Now().UTC()
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
	return s.writeRecordLocked(*record)
}

func (s *taskStore) setVerification(id, status string) error {
	return s.setVerificationResult(id, verify.Result{Status: verify.Status(status)})
}

func (s *taskStore) setVerificationResult(id string, result verify.Result) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	record, err := s.recordLocked(id)
	if err != nil {
		return err
	}
	record.VerificationStatus = string(result.Status)
	record.VerificationChecks = append([]verify.Check(nil), result.Checks...)
	record.VerificationEvidence = append([]verify.Evidence(nil), result.Evidence...)
	record.VerificationFailed = append([]string(nil), result.Failed...)
	record.VerificationSkipped = result.Skipped
	now := time.Now().UTC()
	record.LastVerificationAt = &now
	record.EvidenceRecorded = len(result.Evidence) > 0 || len(result.Checks) > 0 || strings.TrimSpace(string(result.Status)) != ""
	record.VerificationFresh = strings.EqualFold(strings.TrimSpace(string(result.Status)), "VERIFIED") &&
		(record.LastWriteAt == nil || !record.LastWriteAt.After(now))
	if record.VerificationFresh {
		// A successful verification after the last write is the evidence that
		// recovery handled failures from the previous attempt. Keep the total
		// failure count for audit, but do not block completion on resolved work.
		record.UnresolvedFailures = 0
	}
	record.DiffMatchesGoal = len(record.ModifiedFiles) > 0
	record.UpdatedAt = time.Now().UTC()
	return s.writeRecordLocked(*record)
}

// markToolStart records a pending write, but does not claim that a write
// happened. The completion gate only receives write evidence after the tool
// result succeeds and the final workspace diff confirms the change.
func (s *taskStore) markToolStart(id string, readOnly bool, args string) error {
	return s.markToolStartWithID(id, "", readOnly, args)
}

func (s *taskStore) markToolStartWithID(id, toolID string, readOnly bool, args string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	record, err := s.recordLocked(id)
	if err != nil {
		return err
	}
	if !readOnly {
		for _, path := range toolPaths(args) {
			if path == "" || contains(record.PendingWriteFiles, path) || contains(record.ModifiedFiles, path) {
				continue
			}
			record.PendingWriteFiles = append(record.PendingWriteFiles, path)
		}
	} else if strings.TrimSpace(toolID) != "" && !contains(record.PendingReadOnlyIDs, toolID) {
		record.PendingReadOnlyIDs = append(record.PendingReadOnlyIDs, toolID)
	}
	record.UpdatedAt = time.Now().UTC()
	return s.writeRecordLocked(*record)
}

// toolResultConfirmed records the result of a tool call. A successful
// non-read-only result confirms the tool completed, but the actual changed
// file list is still refreshed from the workspace before verification.
func (s *taskStore) toolResultConfirmed(id, toolID string, failed, fallbackReadOnly bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	record, err := s.recordLocked(id)
	if err != nil {
		return err
	}
	record.ToolCalls++
	readOnly := fallbackReadOnly || contains(record.PendingReadOnlyIDs, toolID)
	// Some legacy synthetic tool results omit their ID and read-only bit. With
	// no pending write intent, conservatively treat those as read-only; a write
	// must carry a dispatch ID so it can be confirmed explicitly.
	if toolID == "" && len(record.PendingWriteFiles) == 0 {
		readOnly = true
	}
	if toolID != "" {
		filtered := record.PendingReadOnlyIDs[:0]
		for _, pendingID := range record.PendingReadOnlyIDs {
			if pendingID != toolID {
				filtered = append(filtered, pendingID)
			}
		}
		record.PendingReadOnlyIDs = filtered
	}
	if failed {
		record.ToolFailures++
		record.UnresolvedFailures++
		record.PendingWriteFiles = nil
	} else if !readOnly {
		record.WritesCompleted = true
		now := time.Now().UTC()
		record.LastWriteAt = &now
		for _, path := range record.PendingWriteFiles {
			if path != "" && !contains(record.ModifiedFiles, path) {
				record.ModifiedFiles = append(record.ModifiedFiles, path)
			}
		}
		record.PendingWriteFiles = nil
	}
	record.UpdatedAt = time.Now().UTC()
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
	return s.writeRecordLocked(*record)
}

func (s *taskStore) setModifiedFiles(id string, files []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	record, err := s.recordLocked(id)
	if err != nil {
		return err
	}
	changed := !sameStringList(record.ModifiedFiles, files)
	record.ModifiedFiles = append([]string(nil), files...)
	record.DiffMatchesGoal = len(record.ModifiedFiles) > 0
	if changed && len(record.ModifiedFiles) > 0 {
		record.WritesCompleted = true
		now := time.Now().UTC()
		record.LastWriteAt = &now
	}
	record.UpdatedAt = time.Now().UTC()
	return s.writeRecordLocked(*record)
}

// complete is the only production path that may promote a task to completed.
// The legacy update method remains for backwards-compatible state migrations,
// but explicitly rejects the completed status so it cannot bypass this gate.
func (s *taskStore) complete(id string) (runtime.CompletionDecision, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	record, err := s.recordLocked(id)
	if err != nil {
		return runtime.CompletionDecision{}, err
	}
	if terminalTaskStatus(record.Status) {
		return runtime.CompletionDecision{}, fmt.Errorf("task %q is already terminal", id)
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

func sameStringList(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
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
	typeHint := "event"
	var hint map[string]any
	if json.Unmarshal(payload, &hint) == nil {
		if value, ok := hint["kind"].(string); ok && strings.TrimSpace(value) != "" {
			typeHint = value
		} else if value, ok := hint["type"].(string); ok && strings.TrimSpace(value) != "" {
			typeHint = value
		}
	}
	entry := taskEvent{
		EventID:   fmt.Sprintf("%s:%d", id, seq),
		Seq:       seq,
		TaskID:    id,
		SessionID: record.SessionID,
		Type:      typeHint,
		Timestamp: time.Now().UTC(),
		Payload:   append([]byte(nil), payload...),
	}
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
	return seq, nil
}

func (s *taskStore) activeID() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.latestID
}

func (s *taskStore) activeRecord() *TaskRecord {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.latestID != "" {
		if record, err := s.recordLocked(s.latestID); err == nil {
			return record
		}
	}
	return s.newestRecordLocked()
}

func (s *taskStore) activate(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	record, err := s.recordLocked(id)
	if err != nil {
		return err
	}
	s.latestID = record.ID
	return nil
}

func (s *taskStore) newestRecordLocked() *TaskRecord {
	entries, err := os.ReadDir(s.root)
	if err != nil {
		return nil
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
	return newest
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
