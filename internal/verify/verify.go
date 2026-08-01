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
	"strings"
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

type Result struct {
	Status  Status   `json:"status"`
	Checks  []Check  `json:"checks,omitempty"`
	Passed  []string `json:"passed,omitempty"`
	Failed  []string `json:"failed,omitempty"`
	Skipped string   `json:"skipped,omitempty"`
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
			for _, name := range []string{"test", "lint", "build"} {
				if strings.TrimSpace(doc.Scripts[name]) != "" {
					checks = append(checks, Check{"npm " + name, "npm run " + name})
				}
			}
		}
		return checks
	}
	if fileExists(filepath.Join(root, "pyproject.toml")) || fileExists(filepath.Join(root, "pytest.ini")) || fileExists(filepath.Join(root, "setup.cfg")) {
		if fileExists(filepath.Join(root, "pytest.ini")) || hasPytestConfig(filepath.Join(root, "pyproject.toml")) {
			return []Check{{"pytest", "python -m pytest"}}
		}
		return nil
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
		var cmd *exec.Cmd
		if strings.HasPrefix(check.Command, "npm ") {
			cmd = exec.CommandContext(ctx, "npm", "run", strings.TrimPrefix(check.Command, "npm "))
		} else {
			parts := strings.Fields(check.Command)
			cmd = exec.CommandContext(ctx, parts[0], parts[1:]...)
		}
		cmd.Dir = root
		if output, err := cmd.CombinedOutput(); err != nil {
			text := strings.TrimSpace(string(output))
			if len(text) > 1200 {
				text = text[len(text)-1200:]
			}
			result.Failed = append(result.Failed, fmt.Sprintf("%s: %s", check.Command, text))
			continue
		}
		result.Passed = append(result.Passed, check.Command)
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

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func hasPytestConfig(path string) bool {
	data, err := os.ReadFile(path)
	return err == nil && strings.Contains(string(data), "pytest")
}
