package taskgraph

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const workspaceLockName = ".vcode/tasks/.run.lock"

func acquireWorkspaceLock(ctx context.Context, root string, required bool) (func(), error) {
	if !required {
		return func() {}, nil
	}
	if root == "" {
		return func() {}, errors.New("task workspace is required")
	}
	path := filepath.Join(root, workspaceLockName)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return func() {}, fmt.Errorf("create task lock directory: %w", err)
	}
	deadline := time.NewTimer(30 * time.Second)
	defer deadline.Stop()
	for {
		file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if err == nil {
			_, _ = fmt.Fprintf(file, "pid=%d\nstarted=%s\n", os.Getpid(), time.Now().UTC().Format(time.RFC3339))
			_ = file.Close()
			return func() { _ = os.Remove(path) }, nil
		}
		if !os.IsExist(err) {
			return func() {}, fmt.Errorf("acquire task workspace lock: %w", err)
		}
		select {
		case <-ctx.Done():
			return func() {}, fmt.Errorf("waiting for task workspace lock: %w", ctx.Err())
		case <-deadline.C:
			return func() {}, fmt.Errorf("task workspace is busy (lock: %s)", path)
		case <-time.After(150 * time.Millisecond):
		}
	}
}
