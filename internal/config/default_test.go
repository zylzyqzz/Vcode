package config

import "testing"

func TestDefaultAutoPlanOff(t *testing.T) {
	if got := Default().Agent.AutoPlan; got != "off" {
		t.Fatalf("default auto_plan = %q, want off", got)
	}
}

func TestDefaultReasoningLanguageAuto(t *testing.T) {
	if got := Default().ReasoningLanguage(); got != "auto" {
		t.Fatalf("default reasoning_language = %q, want auto", got)
	}
}

func TestDefaultMemoryCompilerDisabled(t *testing.T) {
	cfg := Default()
	if cfg.MemoryCompilerEnabled() {
		t.Fatal("default memory compiler = true, want false")
	}
	if got := cfg.MemoryCompilerVerbosity(); got != MemoryCompilerVerbosityObserve {
		t.Fatalf("default memory compiler verbosity = %q, want observe", got)
	}
}

func TestDefaultBashModeAuto(t *testing.T) {
	if got := Default().BashMode(); got != "auto" {
		t.Fatalf("default bash mode = %q, want auto", got)
	}
}

func TestAgentRoleFallsBackToLegacyFields(t *testing.T) {
	cfg := Default()
	cfg.Agent.PlannerModel = "legacy-plan"
	if got := cfg.AgentRoleModel("plan", cfg.Agent.PlannerModel); got != "legacy-plan" {
		t.Fatalf("role model fallback = %q", got)
	}
	cfg.Agent.Roles = map[string]AgentRoleConfig{"plan": {Model: "role-plan", Effort: "high"}}
	if got := cfg.AgentRoleModel("plan", cfg.Agent.PlannerModel); got != "role-plan" {
		t.Fatalf("role model = %q", got)
	}
	if got := cfg.AgentRoleEffort("plan", "low"); got != "high" {
		t.Fatalf("role effort = %q", got)
	}
}

func TestAgentRoleToolsReturnsCopy(t *testing.T) {
	cfg := Default()
	cfg.Agent.Roles = map[string]AgentRoleConfig{"build": {Tools: []string{"read_file", "patch"}}}
	got := cfg.AgentRoleTools("build")
	got[0] = "changed"
	if cfg.Agent.Roles["build"].Tools[0] != "read_file" {
		t.Fatal("role tool list was not copied")
	}
	if len(cfg.AgentRoleTools("missing")) != 0 {
		t.Fatal("missing role should have no tool allowlist")
	}
}

func TestDefaultDesktopAppearanceAutoGraphite(t *testing.T) {
	cfg := Default()
	if got := cfg.DesktopTheme(); got != "auto" {
		t.Fatalf("default desktop theme = %q, want auto", got)
	}
	if got := cfg.DesktopThemeStyle(); got != "" {
		t.Fatalf("default desktop theme style = %q, want empty so frontend resolves graphite", got)
	}
}

func TestDefaultDesktopMetricsOn(t *testing.T) {
	cfg := Default()
	if !cfg.DesktopMetrics() {
		t.Fatal("default desktop metrics = false, want true")
	}
	disabled := false
	cfg.Desktop.Metrics = &disabled
	if cfg.DesktopMetrics() {
		t.Fatal("desktop metrics explicit false = true, want false")
	}
}
