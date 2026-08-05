package serve

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type workspaceLock struct {
	Path string
	ID   string
}

type workspaceLockOwner struct {
	TaskID  string    `json:"task_id"`
	PID     int       `json:"pid"`
	Created time.Time `json:"created_at"`
}

func acquireWorkspaceLock(workspace, taskID string) (*workspaceLock, error) {
	workspace = strings.TrimSpace(workspace)
	taskID = strings.TrimSpace(taskID)
	if workspace == "" || taskID == "" {
		return nil, errors.New("workspace and task id are required")
	}
	root, err := filepath.Abs(workspace)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return nil, fmt.Errorf("workspace is unavailable: %s", root)
	}
	lockDir := filepath.Join(root, ".vcode", "tasks")
	if err := os.MkdirAll(lockDir, 0o700); err != nil {
		return nil, fmt.Errorf("create workspace lock directory: %w", err)
	}
	path := filepath.Join(lockDir, ".run.lock")
	owner := workspaceLockOwner{TaskID: taskID, PID: os.Getpid(), Created: time.Now().UTC()}
	data, _ := json.Marshal(owner)
	for {
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err == nil {
			if _, err := f.Write(append(data, '\n')); err != nil {
				_ = f.Close()
				_ = os.Remove(path)
				return nil, err
			}
			if err := f.Close(); err != nil {
				_ = os.Remove(path)
				return nil, err
			}
			break
		}
		if !os.IsExist(err) {
			return nil, fmt.Errorf("create workspace lock: %w", err)
		}
		var existing workspaceLockOwner
		if raw, readErr := os.ReadFile(path); readErr == nil {
			_ = json.Unmarshal(raw, &existing)
		}
		if existing.TaskID == taskID {
			return nil, errors.New("workspace is busy")
		}
		if existing.PID > 0 && !workspaceProcessAlive(existing.PID) {
			if removeErr := os.Remove(path); removeErr == nil || os.IsNotExist(removeErr) {
				continue
			}
		}
		if existing.TaskID != "" {
			return nil, fmt.Errorf("workspace is busy with task %s", existing.TaskID)
		}
		return nil, errors.New("workspace is busy")
	}
	return &workspaceLock{Path: path, ID: taskID}, nil
}

func (l *workspaceLock) release() error {
	if l == nil || l.Path == "" {
		return nil
	}
	raw, err := os.ReadFile(l.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var owner workspaceLockOwner
	if json.Unmarshal(raw, &owner) == nil && owner.TaskID != "" && owner.TaskID != l.ID {
		return errors.New("workspace lock ownership changed")
	}
	if err := os.Remove(l.Path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
