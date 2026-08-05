package serve

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWorkspaceLockSerializesWritersAndReleasesOwnership(t *testing.T) {
	root := t.TempDir()
	first, err := acquireWorkspaceLock(root, "task-a")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := acquireWorkspaceLock(root, "task-b"); err == nil {
		t.Fatal("second writer acquired the workspace")
	}
	if err := first.release(); err != nil {
		t.Fatal(err)
	}
	second, err := acquireWorkspaceLock(root, "task-b")
	if err != nil {
		t.Fatal(err)
	}
	if err := second.release(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, ".vcode", "tasks", ".run.lock")); !os.IsNotExist(err) {
		t.Fatalf("workspace lock remains: %v", err)
	}
}

func TestWorkspaceLockRecoversDeadOwner(t *testing.T) {
	root := t.TempDir()
	lockDir := filepath.Join(root, ".vcode", "tasks")
	if err := os.MkdirAll(lockDir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(lockDir, ".run.lock")
	if err := os.WriteFile(path, []byte(`{"task_id":"dead","pid":999999999,"created_at":"2020-01-01T00:00:00Z"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	lock, err := acquireWorkspaceLock(root, "new")
	if err != nil {
		t.Fatal(err)
	}
	defer lock.release()
	if lock.ID != "new" {
		t.Fatalf("lock owner = %q", lock.ID)
	}
}
