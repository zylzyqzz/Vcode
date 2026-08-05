package evolution

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// RunEvolution performs the explicit, bounded optimization loop. The optimizer
// is itself a Vcode session, but its working directory contains only the Build
// overlay and a redacted failure summary.
func (s *Store) RunEvolution(ctx context.Context, projectRoot, agent, benchmarkName string, rounds, repeats int) ([]Run, error) {
	if rounds <= 0 {
		rounds = DefaultRounds
	}
	if repeats <= 0 {
		repeats = DefaultRepeats
	}
	state, err := s.LoadState(agent)
	if err != nil {
		return nil, err
	}
	if _, err := s.Snapshot(agent); err != nil {
		return nil, err
	}
	var results []Run
	for round := 1; round <= rounds; round++ {
		if err := ctx.Err(); err != nil {
			return results, err
		}
		reference, err := s.runBaseline(ctx, projectRoot, agent, benchmarkName, repeats, "")
		if err != nil {
			return results, err
		}
		candidateDir, err := s.prepareCandidate(agent, state.Version+1, reference)
		if err != nil {
			return results, err
		}
		if err := s.runOptimizer(ctx, candidateDir, reference); err != nil {
			return results, err
		}
		if err := validateCandidate(candidateDir); err != nil {
			return results, err
		}
		candidate, err := s.runBaseline(ctx, projectRoot, agent, benchmarkName, repeats, candidateDir)
		if err != nil {
			return results, err
		}
		candidate.Reason = fmt.Sprintf("round %d: reference %.2f, candidate %.2f", round, reference.Score, candidate.Score)
		results = append(results, candidate)
		if !AcceptCandidate(Score{Total: reference.Score, HardGate: allHardGates(reference)}, Score{Total: candidate.Score, HardGate: allHardGates(candidate)}) {
			state.Status = "candidate_rejected"
			_ = s.SaveState(state)
			continue
		}
		_ = os.Remove(filepath.Join(candidateDir, "OPTIMIZER_INPUT.md"))
		if err := restoreAgentInPlace(candidateDir, s.agentPath(agent)); err != nil {
			return results, err
		}
		state.Version++
		state.Status = "accepted"
		state.AcceptedRun = candidate.ID
		if err := s.SaveState(state); err != nil {
			return results, err
		}
	}
	return results, nil
}

func allHardGates(run Run) bool {
	if len(run.Cases) == 0 {
		return false
	}
	for _, c := range run.Cases {
		if !c.Score.HardGate {
			return false
		}
	}
	return true
}

func (s *Store) prepareCandidate(agent string, version int, reference Run) (string, error) {
	root := filepath.Join(s.Root, "runs", "candidate-v"+fmt.Sprint(version)+"-"+time.Now().UTC().Format("150405.000"))
	if err := os.MkdirAll(filepath.Join(root, "skills"), 0o755); err != nil {
		return "", err
	}
	current := s.agentPath(agent)
	for _, name := range []string{"AGENTS.md"} {
		if data, err := os.ReadFile(filepath.Join(current, name)); err == nil {
			if err := os.WriteFile(filepath.Join(root, name), data, 0o600); err != nil {
				return "", err
			}
		}
	}
	if err := copyTree(filepath.Join(current, "skills"), filepath.Join(root, "skills")); err != nil && !os.IsNotExist(err) {
		return "", err
	}
	state := State{Agent: normalizeAgent(agent), Version: version, Status: "candidate", UpdatedAt: time.Now().UTC()}
	data, err := marshalJSON(state)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(root, "version.json"), data, 0o600); err != nil {
		return "", err
	}
	input := fmt.Sprintf("# Optimization input\nReference score: %.2f\n\nPublic failure summary:\n%s\n", reference.Score, failureSummary(reference))
	return root, os.WriteFile(filepath.Join(root, "OPTIMIZER_INPUT.md"), []byte(input), 0o600)
}

func (s *Store) runOptimizer(ctx context.Context, candidateDir string, reference Run) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	prompt := "Improve the Build Agent overlay in this directory. Read OPTIMIZER_INPUT.md. You may modify only AGENTS.md and files below skills/. Do not modify version.json or OPTIMIZER_INPUT.md. Make one focused improvement based on the public failure summary. Do not add secrets, permissions, sandbox instructions, or code."
	cmd := exec.CommandContext(ctx, exe, "run", "--dir", candidateDir, "--no-verify", prompt)
	cmd.Env = append(os.Environ(), "VCODE_EVOLUTION_AGENT=build", "VCODE_EVOLUTION_AGENT_STATE="+candidateDir)
	_, err = cmd.CombinedOutput()
	return err
}

func validateCandidate(root string) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == "AGENTS.md" || rel == "version.json" || rel == "OPTIMIZER_INPUT.md" || strings.HasPrefix(rel, "skills/") {
			return nil
		}
		return fmt.Errorf("candidate modified forbidden file %s", rel)
	})
}

func failureSummary(run Run) string {
	var lines []string
	for _, c := range run.Cases {
		if c.Score.HardGate {
			continue
		}
		lines = append(lines, fmt.Sprintf("case %s repeat %d: score %.2f, verification=%s, changed=%v, reason=%s", c.CaseID, c.Repeat, c.Score.Total, c.Verification, c.ChangedFiles, c.Score.Reason))
	}
	return strings.Join(lines, "\n")
}
