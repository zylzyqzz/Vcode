// Package verify discovers and runs conservative project checks for CLI runs.
// It deliberately uses commands already declared by the project rather than
// inventing a new build system.
package verify

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"vcode/internal/shellparse"
)

type Status string

const (
	Verified   Status = "VERIFIED"
	Partial    Status = "PARTIAL"
	Unverified Status = "UNVERIFIED"
)

type Check struct {
	Name    string `json:"name"`
	Command string `json:"command"`
}

type Evidence struct {
	Name       string `json:"name"`
	Command    string `json:"command"`
	Status     string `json:"status"`
	Output     string `json:"output,omitempty"`
	DurationMS int64  `json:"duration_ms"`
}

type Result struct {
	Status   Status     `json:"status"`
	Checks   []Check    `json:"checks,omitempty"`
	Evidence []Evidence `json:"evidence,omitempty"`
	Passed   []string   `json:"passed,omitempty"`
	Failed   []string   `json:"failed,omitempty"`
	Skipped  string     `json:"skipped,omitempty"`
}

func (r Result) Error() string {
	if len(r.Failed) == 0 {
		return ""
	}
	return strings.Join(r.Failed, "; ")
}

// Plan discovers checks without running them. Existing project scripts win;
// otherwise language-standard checks are selected.
func Plan(root string) []Check {
	var checks []Check
	if fileExists(filepath.Join(root, "go.mod")) {
		checks = append(checks, Check{"go test", "go test ./..."}, Check{"go vet", "go vet ./..."}, Check{"go build", "go build ./..."})
		return checks
	}
	if packageJSON := filepath.Join(root, "package.json"); fileExists(packageJSON) {
		var doc struct {
			Scripts map[string]string `json:"scripts"`
		}
		if data, err := os.ReadFile(packageJSON); err == nil && json.Unmarshal(data, &doc) == nil {
			manager := nodePackageManager(root)
			for _, name := range []string{"test", "lint", "build"} {
				if strings.TrimSpace(doc.Scripts[name]) != "" {
					checks = append(checks, Check{manager + " " + name, manager + " run " + name})
				}
			}
		}
		return checks
	}
	if fileExists(filepath.Join(root, "pyproject.toml")) || fileExists(filepath.Join(root, "pytest.ini")) || fileExists(filepath.Join(root, "setup.cfg")) {
		if fileExists(filepath.Join(root, "pytest.ini")) || hasPytestConfig(filepath.Join(root, "pyproject.toml")) {
			return []Check{{"pytest", "python -m pytest"}}
		}
		return []Check{{"unittest", "python -m unittest discover"}}
	}
	if fileExists(filepath.Join(root, "Cargo.toml")) {
		return []Check{{"cargo test", "cargo test"}, {"cargo check", "cargo check"}}
	}
	return nil
}

// Run executes the discovered checks in order and never hides a failed check.
func Run(ctx context.Context, root string) Result {
	checks := Plan(root)
	result := Result{Checks: checks}
	if len(checks) == 0 {
		result.Status = Unverified
		result.Skipped = "no supported project verification command was found"
		return result
	}
	for _, check := range checks {
		if err := ctx.Err(); err != nil {
			result.Failed = append(result.Failed, fmt.Sprintf("%s: verification stopped: %s", check.Command, err))
			break
		}
		started := time.Now()
		cmd, commandErr := commandFor(ctx, root, check.Command)
		if commandErr != nil {
			result.Failed = append(result.Failed, fmt.Sprintf("%s: %v", check.Command, commandErr))
			result.Evidence = append(result.Evidence, Evidence{Name: check.Name, Command: check.Command, Status: "failed", Output: commandErr.Error(), DurationMS: time.Since(started).Milliseconds()})
			continue
		}
		cmd.Dir = root
		output, err := cmd.CombinedOutput()
		evidence := Evidence{Name: check.Name, Command: check.Command, DurationMS: time.Since(started).Milliseconds()}
		if err != nil {
			text := strings.TrimSpace(string(output))
			if len(text) > 1200 {
				text = text[len(text)-1200:]
			}
			if ctx.Err() != nil {
				result.Failed = append(result.Failed, fmt.Sprintf("%s: verification stopped: %s", check.Command, ctx.Err()))
				evidence.Status = "cancelled"
			} else if text == "" {
				result.Failed = append(result.Failed, fmt.Sprintf("%s: command failed without output (%v)", check.Command, err))
				evidence.Status = "failed"
			} else {
				result.Failed = append(result.Failed, fmt.Sprintf("%s: %s", check.Command, text))
				evidence.Status = "failed"
			}
			evidence.Output = text
			result.Evidence = append(result.Evidence, evidence)
			continue
		}
		result.Passed = append(result.Passed, check.Command)
		evidence.Status = "passed"
		result.Evidence = append(result.Evidence, evidence)
	}
	if len(result.Failed) == 0 {
		result.Status = Verified
	} else if len(result.Passed) > 0 {
		result.Status = Partial
	} else {
		result.Status = Partial
	}
	return result
}

// commandFor preserves quoted arguments instead of splitting them on spaces.
// Static commands bypass a shell; commands with shell syntax use the platform's
// explicit shell so Windows PowerShell behavior is deterministic.
func commandFor(ctx context.Context, root, command string) (*exec.Cmd, error) {
	if strings.TrimSpace(command) == "" {
		return nil, fmt.Errorf("empty verification command")
	}
	parsed, err := shellparse.ParseStaticCommand(command, shellparse.StaticCommandPolicy{})
	if err == nil && len(parsed.Argv) > 0 {
		cmd := exec.CommandContext(ctx, parsed.Argv[0], parsed.Argv[1:]...)
		cmd.Dir = root
		if len(parsed.Env) > 0 {
			cmd.Env = append(os.Environ(), parsed.Env...)
		}
		return cmd, nil
	}
	if runtime.GOOS == "windows" {
		return exec.CommandContext(ctx, "powershell.exe", "-NoLogo", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", command), nil
	}
	return exec.CommandContext(ctx, "/bin/sh", "-lc", command), nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func hasPytestConfig(path string) bool {
	data, err := os.ReadFile(path)
	return err == nil && strings.Contains(string(data), "pytest")
}

func nodePackageManager(root string) string {
	switch {
	case fileExists(filepath.Join(root, "pnpm-lock.yaml")):
		return "pnpm"
	case fileExists(filepath.Join(root, "yarn.lock")):
		return "yarn"
	case fileExists(filepath.Join(root, "bun.lockb")), fileExists(filepath.Join(root, "bun.lock")):
		return "bun"
	default:
		return "npm"
	}
}
