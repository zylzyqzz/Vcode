// Package worktree manages isolated Git workspaces for writable task nodes.
package worktree

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Manager struct {
	ProjectRoot string
	Root        string
}

type MergeReport struct {
	Commit        string   `json:"commit"`
	Merged        bool     `json:"merged"`
	Aborted       bool     `json:"aborted"`
	ConflictFiles []string `json:"conflict_files,omitempty"`
	Error         string   `json:"error,omitempty"`
}

func NewManager(projectRoot string) *Manager {
	return &Manager{ProjectRoot: projectRoot, Root: filepath.Join(projectRoot, ".vcode", "worktrees")}
}

func (m *Manager) Path(taskID, nodeID string) (string, error) {
	if m == nil || strings.TrimSpace(m.ProjectRoot) == "" {
		return "", errors.New("worktree project root is required")
	}
	for _, part := range []string{taskID, nodeID} {
		if part == "" || filepath.Base(part) != part || part == "." || part == ".." {
			return "", fmt.Errorf("invalid worktree identifier %q", part)
		}
	}
	return filepath.Join(m.Root, taskID, nodeID), nil
}

func (m *Manager) Create(ctx context.Context, taskID, nodeID string) (string, error) {
	path, err := m.Path(taskID, nodeID)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(path); err == nil {
		return path, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	if err := runGit(ctx, m.ProjectRoot, "worktree", "add", "--detach", path, "HEAD"); err != nil {
		return "", fmt.Errorf("create worktree: %w", err)
	}
	return path, nil
}

func (m *Manager) Remove(ctx context.Context, taskID, nodeID string) error {
	path, err := m.Path(taskID, nodeID)
	if err != nil {
		return err
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil
	}
	return runGit(ctx, m.ProjectRoot, "worktree", "remove", "--force", path)
}

func (m *Manager) List(ctx context.Context) (string, error) {
	return runGitOutput(ctx, m.ProjectRoot, "worktree", "list", "--porcelain")
}

func (m *Manager) EnsureProjectClean(ctx context.Context) error {
	if m == nil || strings.TrimSpace(m.ProjectRoot) == "" {
		return errors.New("worktree project root is required")
	}
	out, err := runGitOutput(ctx, m.ProjectRoot, "status", "--porcelain")
	if err != nil {
		return err
	}
	if strings.TrimSpace(out) != "" {
		return errors.New("project worktree has uncommitted changes; commit or stash them before integration")
	}
	return nil
}

func (m *Manager) ChangedFiles(ctx context.Context, taskID, nodeID string) ([]string, error) {
	path, err := m.Path(taskID, nodeID)
	if err != nil {
		return nil, err
	}
	return m.ChangedFilesAt(ctx, path)
}

// ChangedFilesAt inspects the actual workspace used by a node. Recovery and
// test nodes often inherit a build worktree rather than owning a node-specific
// worktree of their own.
func (m *Manager) ChangedFilesAt(ctx context.Context, workspace string) ([]string, error) {
	if strings.TrimSpace(workspace) == "" {
		return nil, errors.New("workspace is required")
	}
	out, err := runGitOutput(ctx, workspace, "status", "--porcelain")
	if err != nil {
		return nil, err
	}
	var files []string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if len(line) > 3 {
			files = append(files, strings.TrimSpace(line[2:]))
		}
	}
	return files, nil
}

func (m *Manager) Commit(ctx context.Context, taskID, nodeID, message string) (string, error) {
	path, err := m.Path(taskID, nodeID)
	if err != nil {
		return "", err
	}
	return m.CommitAt(ctx, path, message)
}

// CommitAt commits the actual workspace used by a node.
func (m *Manager) CommitAt(ctx context.Context, workspace, message string) (string, error) {
	if strings.TrimSpace(workspace) == "" {
		return "", errors.New("workspace is required")
	}
	if err := runGit(ctx, workspace, "add", "-A"); err != nil {
		return "", err
	}
	if err := runGit(ctx, workspace, "commit", "-m", message); err != nil {
		return "", err
	}
	return runGitOutput(ctx, workspace, "rev-parse", "HEAD")
}

func (m *Manager) MergeCommit(ctx context.Context, commit string) error {
	report, err := m.MergeCommitReport(ctx, commit)
	if err != nil {
		return err
	}
	if !report.Merged {
		return fmt.Errorf("merge commit %s was not applied", commit)
	}
	return nil
}

// MergeCommitReport makes integration failures actionable for a Coordinator:
// conflicting paths are captured before cherry-pick --abort restores the main
// worktree. The old MergeCommit API remains as a strict compatibility wrapper.
func (m *Manager) MergeCommitReport(ctx context.Context, commit string) (MergeReport, error) {
	report := MergeReport{Commit: strings.TrimSpace(commit)}
	commit = strings.TrimSpace(commit)
	if commit == "" {
		return report, errors.New("commit is required")
	}
	if err := m.EnsureProjectClean(ctx); err != nil {
		report.Error = err.Error()
		return report, fmt.Errorf("integration requires a clean project worktree: %w", err)
	}
	if err := runGit(ctx, m.ProjectRoot, "cherry-pick", commit); err != nil {
		if conflicts, conflictErr := runGitOutput(ctx, m.ProjectRoot, "diff", "--name-only", "--diff-filter=U"); conflictErr == nil {
			for _, path := range strings.Split(conflicts, "\n") {
				if path = strings.TrimSpace(path); path != "" {
					report.ConflictFiles = append(report.ConflictFiles, path)
				}
			}
		}
		report.Error = err.Error()
		// Abort only an in-progress cherry-pick. If Git rejected before it
		// started, abort is harmless and leaves the target worktree clean.
		if abortErr := runGit(ctx, m.ProjectRoot, "cherry-pick", "--abort"); abortErr == nil {
			report.Aborted = true
		}
		return report, fmt.Errorf("merge commit %s: %w", commit, err)
	}
	report.Merged = true
	return report, nil
}

func runGit(ctx context.Context, dir string, args ...string) error {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
	cmd.Stdout = nil
	cmd.Stderr = nil
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}

func runGitOutput(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}
