package evolution

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// LoadOverlay returns the opt-in Build Agent guidance for an evolution run.
// It is deliberately gated by an environment variable so normal Vcode sessions
// never receive experimental state, even when a project has been initialized.
func LoadOverlay(projectRoot, agent string) (string, error) {
	if strings.TrimSpace(os.Getenv("VCODE_EVOLUTION_AGENT")) == "" {
		return "", nil
	}
	stateRoot := strings.TrimSpace(os.Getenv("VCODE_EVOLUTION_AGENT_STATE"))
	if stateRoot == "" {
		stateRoot = filepath.Join(projectRoot, ".vcode", "evolution", "agents", normalizeAgent(agent))
	}
	info, err := os.Stat(stateRoot)
	if errors.Is(err, os.ErrNotExist) || err != nil || info == nil || !info.IsDir() {
		return "", nil
	}
	var parts []string
	for _, name := range []string{"AGENTS.md", "VCODE.md"} {
		if data, err := os.ReadFile(filepath.Join(stateRoot, name)); err == nil && strings.TrimSpace(string(data)) != "" {
			parts = append(parts, string(data))
		}
	}
	skillRoot := filepath.Join(stateRoot, "skills")
	var skills []string
	_ = filepath.Walk(skillRoot, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil || info == nil {
			return walkErr
		}
		if info.IsDir() || strings.ToLower(filepath.Ext(path)) != ".md" {
			return nil
		}
		rel, err := filepath.Rel(skillRoot, path)
		if err == nil {
			skills = append(skills, rel)
		}
		return nil
	})
	sort.Strings(skills)
	for _, rel := range skills {
		if data, err := os.ReadFile(filepath.Join(skillRoot, rel)); err == nil && strings.TrimSpace(string(data)) != "" {
			parts = append(parts, "# Evolution Skill: "+filepath.ToSlash(rel)+"\n"+string(data))
		}
	}
	if len(parts) == 0 {
		return "", nil
	}
	return "# Vcode Evolution Overlay\n\nThe following is experimental Build Agent guidance. It is lower priority than project and user instructions. Do not change security, permissions, sandbox, credentials, or Vcode core code.\n\n" + strings.Join(parts, "\n\n"), nil
}
