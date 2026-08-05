// Package runtime contains transport-neutral types shared by local and cloud
// execution targets. Clients use the same task and event vocabulary regardless
// of where the Vcode Agent is running.
package runtime

import (
	"encoding/json"
	"time"
)

const (
	TargetLocalComputer = "local_computer"
	TargetCloud         = "cloud"

	TargetOnline  = "online"
	TargetOffline = "offline"
	TargetBusy    = "busy"
)

const (
	TaskQueued            = "queued"
	TaskRunning           = "running"
	TaskWaitingPermission = "waiting_permission"
	TaskVerifying         = "verifying"
	TaskPaused            = "paused"
	TaskCompleted         = "completed"
	TaskPartial           = "partial"
	TaskFailed            = "failed"
	TaskBlocked           = "blocked"
	TaskCancelled         = "cancelled"
)

type RuntimeTarget struct {
	ID        string    `json:"id"`
	Kind      string    `json:"kind"`
	Name      string    `json:"name"`
	Status    string    `json:"status"`
	Model     string    `json:"model,omitempty"`
	Workspace string    `json:"workspace,omitempty"`
	Features  []string  `json:"features,omitempty"`
	LastSeen  time.Time `json:"last_seen,omitempty"`
}

type LocalProject struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Root     string `json:"root"`
	Branch   string `json:"branch,omitempty"`
	ReadOnly bool   `json:"read_only"`
}

type RuntimeEvent struct {
	EventID   string          `json:"event_id"`
	Seq       uint64          `json:"seq"`
	TargetID  string          `json:"target_id"`
	TaskID    string          `json:"task_id"`
	SessionID string          `json:"session_id"`
	Type      string          `json:"type"`
	Timestamp time.Time       `json:"timestamp"`
	Payload   json.RawMessage `json:"payload,omitempty"`
}

type PairingRequest struct {
	Code      string    `json:"code"`
	TargetID  string    `json:"target_id"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
	Used      bool      `json:"used"`
}
