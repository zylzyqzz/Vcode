package evolution

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"vcode/internal/verify"
)

// RunBaseline executes a benchmark against the current Build Agent in fresh
// workspaces. It intentionally uses the public Vcode CLI so the same model,
// permission, sandbox, session and verification paths are exercised as normal
// coding work.
func (s *Store) RunBaseline(ctx context.Context, projectRoot, agent, benchmarkName string, repeats int) (Run, error) {
	return s.runBaseline(ctx, projectRoot, agent, benchmarkName, repeats, "")
}

func (s *Store) runBaseline(ctx context.Context, projectRoot, agent, benchmarkName string, repeats int, stateOverride string) (Run, error) {
	if repeats <= 0 {
		repeats = DefaultRepeats
	}
	benchPath := filepath.Join(s.Root, "benchmarks", benchmarkName+".toml")
	benchmark, err := LoadBenchmark(benchPath)
	if err != nil {
		return Run{}, err
	}
	state, err := s.LoadState(agent)
	if err != nil {
		return Run{}, err
	}
	if stateOverride != "" {
		if err := readJSON(filepath.Join(stateOverride, "version.json"), &state); err != nil {
			return Run{}, err
		}
	}
	if _, err := s.Snapshot(agent); err != nil {
		return Run{}, err
	}
	run := Run{ID: time.Now().UTC().Format("20060102-150405.000000000"), Agent: normalizeAgent(agent), Benchmark: benchmark.Name, Version: state.Version, Status: "running", StartedAt: time.Now().UTC()}
	exe, err := os.Executable()
	if err != nil {
		return Run{}, err
	}
	var totals []float64
	for _, benchmarkCase := range benchmark.Cases {
		count := benchmarkCase.Repeats
		if count <= 0 {
			count = repeats
		}
		for repeat := 1; repeat <= count; repeat++ {
			if err := ctx.Err(); err != nil {
				run.Status = "interrupted"
				run.Reason = err.Error()
				run.FinishedAt = time.Now().UTC()
				_ = s.saveRun(run)
				return run, err
			}
			overlay := stateOverride
			if overlay == "" {
				overlay = filepath.Join(s.Root, "agents", normalizeAgent(agent))
			}
			caseRun, err := executeCase(ctx, exe, projectRoot, overlay, benchmarkCase, run.ID, repeat)
			if err != nil {
				run.Status = "failed"
				run.Reason = err.Error()
				run.Cases = append(run.Cases, caseRun)
				run.FinishedAt = time.Now().UTC()
				_ = s.saveRun(run)
				return run, err
			}
			run.Cases = append(run.Cases, caseRun)
			totals = append(totals, caseRun.Score.Total)
		}
	}
	if len(totals) > 0 {
		for _, total := range totals {
			run.Score += total
		}
		run.Score /= float64(len(totals))
	}
	run.Status = "completed"
	run.FinishedAt = time.Now().UTC()
	if err := s.saveRun(run); err != nil {
		return Run{}, err
	}
	return run, nil
}

func executeCase(ctx context.Context, exe, projectRoot, agentState string, benchmarkCase Case, runID string, repeat int) (CaseRun, error) {
	workspace, err := os.MkdirTemp(filepath.Join(projectRoot, ".vcode", "evolution", "runs"), runID+"-"+benchmarkCase.ID+"-")
	if err != nil {
		return CaseRun{}, err
	}
	caseRun := CaseRun{CaseID: benchmarkCase.ID, Repeat: repeat, Workspace: workspace}
	if err := copyProject(projectRoot, workspace); err != nil {
		return caseRun, err
	}
	if strings.TrimSpace(benchmarkCase.Fixture) != "" {
		if err := copyTree(filepath.Join(projectRoot, benchmarkCase.Fixture), workspace); err != nil {
			return caseRun, err
		}
	}
	prompt := strings.TrimSpace(benchmarkCase.Task) + "\n\n完成后必须实际修改工作区并运行可用验证。不要读取 .vcode/evolution/benchmarks 下的 rubric 或其他 case。"
	cmd := exec.CommandContext(ctx, exe, "run", "--dir", workspace, "--no-verify", prompt)
	cmd.Env = append(os.Environ(),
		"VCODE_EVOLUTION_AGENT=build",
		"VCODE_EVOLUTION_AGENT_STATE="+agentState,
	)
	output, err := cmd.CombinedOutput()
	caseRun.Output = trimOutput(string(output))
	if err != nil {
		caseRun.ExitCode = exitCode(err)
	}
	caseRun.ChangedFiles = changedFiles(projectRoot, workspace)
	verification := verify.RunCommands(ctx, workspace, benchmarkCase.VerifyCommands)
	caseRun.Verification = string(verification.Status)
	completion := 0.0
	if expectedFilesPresent(workspace, benchmarkCase.ExpectedFiles) && len(caseRun.ChangedFiles) > 0 {
		completion = 100
	}
	verifyScore := 0.0
	if len(verification.Checks) > 0 {
		verifyScore = float64(len(verification.Passed)) / float64(len(verification.Checks)) * 100
	}
	withinScope := changedFilesWithinScope(caseRun.ChangedFiles, benchmarkCase.AllowedPaths)
	quality := 100.0
	if !withinScope {
		quality = 0
	}
	evidence := len(verification.Checks) > 0
	hardGate := completion == 100 && evidence && len(verification.Failed) == 0 && len(caseRun.ChangedFiles) > 0 && withinScope
	score, scoreErr := CalculateScore(completion, verifyScore, quality, 100, hardGate)
	if scoreErr != nil {
		return caseRun, scoreErr
	}
	caseRun.Score = score
	return caseRun, nil
}

func changedFilesWithinScope(files, allowed []string) bool {
	if len(allowed) == 0 {
		return true
	}
	for _, file := range files {
		ok := false
		for _, prefix := range allowed {
			prefix = strings.TrimSuffix(filepath.ToSlash(filepath.Clean(prefix)), "/")
			file = filepath.ToSlash(filepath.Clean(file))
			if file == prefix || strings.HasPrefix(file, prefix+"/") {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
	}
	return true
}

func (s *Store) saveRun(run Run) error {
	path := filepath.Join(s.Root, "runs", run.ID+".json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := marshalJSON(run)
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return err
	}
	return s.AppendHistory(run)
}

func marshalJSON(value any) ([]byte, error) {
	data, err := jsonMarshalIndent(value)
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func jsonMarshalIndent(value any) ([]byte, error) {
	return json.MarshalIndent(value, "", "  ")
}

func copyProject(source, dest string) error {
	return filepath.Walk(source, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == dest || strings.HasPrefix(path, dest+string(filepath.Separator)) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		if rel == ".git" || rel == ".vcode" || rel == ".env" || strings.HasPrefix(rel, ".git"+string(filepath.Separator)) || strings.HasPrefix(rel, ".vcode"+string(filepath.Separator)) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		target := filepath.Join(dest, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o600)
	})
}

func expectedFilesPresent(root string, files []string) bool {
	if len(files) == 0 {
		return true
	}
	for _, file := range files {
		path := filepath.Join(root, filepath.Clean(file))
		if _, err := os.Stat(path); err != nil {
			return false
		}
	}
	return true
}

func changedFiles(before, after string) []string {
	var files []string
	_ = filepath.Walk(after, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		rel, err := filepath.Rel(after, path)
		if err != nil || rel == ".vcode" || strings.HasPrefix(rel, ".vcode"+string(filepath.Separator)) {
			return nil
		}
		original := filepath.Join(before, rel)
		left, leftErr := os.ReadFile(original)
		right, rightErr := os.ReadFile(path)
		if leftErr != nil || rightErr != nil || string(left) != string(right) {
			files = append(files, filepath.ToSlash(rel))
		}
		return nil
	})
	sort.Strings(files)
	return files
}

func trimOutput(output string) string {
	output = strings.TrimSpace(output)
	if len(output) > 4000 {
		return output[len(output)-4000:]
	}
	return output
}

func exitCode(err error) int {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	return -1
}
