package worktree

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPathRejectsTraversal(t *testing.T) {
	m := NewManager(t.TempDir())
	if _, err := m.Path("../task", "node"); err == nil {
		t.Fatal("expected traversal rejection")
	}
	if _, err := m.Path("task", "../node"); err == nil {
		t.Fatal("expected node traversal rejection")
	}
}

func TestCreateListRemoveWorktree(t *testing.T) {
	root := t.TempDir()
	gitRun(t, root, "init")
	gitRun(t, root, "config", "user.name", "Vcode Test")
	gitRun(t, root, "config", "user.email", "test@example.invalid")
	if err := os.WriteFile(filepath.Join(root, "README"), []byte("test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, root, "add", ".")
	gitRun(t, root, "commit", "-m", "init")
	m := NewManager(root)
	path, err := m.Create(context.Background(), "task-1", "build")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
	listing, err := m.List(context.Background())
	if err != nil || !strings.Contains(strings.ToLower(filepath.ToSlash(listing)), "task-1/build") {
		t.Fatalf("listing=%q err=%v", listing, err)
	}
	if err := m.Remove(context.Background(), "task-1", "build"); err != nil {
		t.Fatal(err)
	}
}

func gitRun(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}
