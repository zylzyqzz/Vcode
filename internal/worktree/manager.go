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
