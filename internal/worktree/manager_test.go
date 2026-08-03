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

func TestCommitAndMergeWorktree(t *testing.T) {
	root := t.TempDir()
	gitRun(t, root, "init")
	gitRun(t, root, "config", "user.name", "Vcode Test")
	gitRun(t, root, "config", "user.email", "test@example.invalid")
	if err := os.WriteFile(filepath.Join(root, "README"), []byte("before\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(".vcode/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, root, "add", ".")
	gitRun(t, root, "commit", "-m", "init")
	m := NewManager(root)
	path, err := m.Create(context.Background(), "task-merge", "build")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "README"), []byte("after\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	files, err := m.ChangedFiles(context.Background(), "task-merge", "build")
	if err != nil || len(files) != 1 {
		t.Fatalf("files=%v err=%v", files, err)
	}
	files, err = m.ChangedFilesAt(context.Background(), path)
	if err != nil || len(files) != 1 {
		t.Fatalf("files at workspace=%v err=%v", files, err)
	}
	commit, err := m.Commit(context.Background(), "task-merge", "build", "task build")
	if err != nil {
		t.Fatal(err)
	}
	if err := m.MergeCommit(context.Background(), commit); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, "README"))
	if err != nil || strings.ReplaceAll(string(data), "\r\n", "\n") != "after\n" {
		t.Fatalf("merged data=%q err=%v", data, err)
	}
	_ = m.Remove(context.Background(), "task-merge", "build")
}

func TestMergeConflictAbortsCherryPick(t *testing.T) {
	root := t.TempDir()
	gitRun(t, root, "init")
	gitRun(t, root, "config", "user.name", "Vcode Test")
	gitRun(t, root, "config", "user.email", "test@example.invalid")
	if err := os.WriteFile(filepath.Join(root, "README"), []byte("before\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(".vcode/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, root, "add", ".")
	gitRun(t, root, "commit", "-m", "init")
	m := NewManager(root)
	first, err := m.Create(context.Background(), "task-conflict", "first")
	if err != nil {
		t.Fatal(err)
	}
	second, err := m.Create(context.Background(), "task-conflict", "second")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(first, "README"), []byte("first\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(second, "README"), []byte("second\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	firstCommit, err := m.Commit(context.Background(), "task-conflict", "first", "first change")
	if err != nil {
		t.Fatal(err)
	}
	secondCommit, err := m.Commit(context.Background(), "task-conflict", "second", "second change")
	if err != nil {
		t.Fatal(err)
	}
	if err := m.MergeCommit(context.Background(), firstCommit); err != nil {
		t.Fatal(err)
	}
	if err := m.MergeCommit(context.Background(), secondCommit); err == nil {
		t.Fatal("expected cherry-pick conflict")
	}
	status, err := runGitOutput(context.Background(), root, "status", "--porcelain")
	if err != nil || strings.TrimSpace(status) != "" {
		t.Fatalf("repository not clean after abort: %q err=%v", status, err)
	}
	_ = m.Remove(context.Background(), "task-conflict", "first")
	_ = m.Remove(context.Background(), "task-conflict", "second")
}

func TestMergeRejectsDirtyProjectWorktree(t *testing.T) {
	root := t.TempDir()
	gitRun(t, root, "init")
	gitRun(t, root, "config", "user.name", "Vcode Test")
	gitRun(t, root, "config", "user.email", "test@example.invalid")
	if err := os.WriteFile(filepath.Join(root, "README"), []byte("before\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, root, "add", ".")
	gitRun(t, root, "commit", "-m", "init")
	if err := os.WriteFile(filepath.Join(root, "README"), []byte("local edit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := NewManager(root).MergeCommit(context.Background(), "HEAD"); err == nil || !strings.Contains(err.Error(), "clean project worktree") {
		t.Fatalf("err=%v, want dirty-worktree guard", err)
	}
}

func TestMergeCommitReportCapturesConflictsAndAborts(t *testing.T) {
	project := t.TempDir()
	gitRun(t, project, "init")
	gitRun(t, project, "config", "user.name", "Vcode Test")
	gitRun(t, project, "config", "user.email", "test@example.invalid")
	if err := os.WriteFile(filepath.Join(project, "seed"), []byte("seed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, project, "add", ".")
	gitRun(t, project, "commit", "-m", "init")
	m := NewManager(project)
	gitRun(t, project, "checkout", "-b", "feature")
	if err := os.WriteFile(filepath.Join(project, "README.md"), []byte("feature\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, project, "add", "README.md")
	gitRun(t, project, "commit", "-m", "feature change")
	commit, err := runGitOutput(context.Background(), project, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	gitRun(t, project, "checkout", "-")
	if err := os.WriteFile(filepath.Join(project, "README.md"), []byte("main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runGit(context.Background(), project, "add", "README.md"); err != nil {
		t.Fatal(err)
	}
	if err := runGit(context.Background(), project, "commit", "-m", "main change"); err != nil {
		t.Fatal(err)
	}
	report, mergeErr := m.MergeCommitReport(context.Background(), strings.TrimSpace(commit))
	if mergeErr == nil || !report.Aborted || len(report.ConflictFiles) != 1 || report.ConflictFiles[0] != "README.md" {
		t.Fatalf("report=%+v err=%v", report, mergeErr)
	}
	if out, statusErr := runGitOutput(context.Background(), project, "status", "--porcelain"); statusErr != nil || strings.TrimSpace(out) != "" {
		t.Fatalf("project not clean: %q err=%v", out, statusErr)
	}
}

func gitRun(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}
