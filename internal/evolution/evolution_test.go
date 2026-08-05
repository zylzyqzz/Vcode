package evolution

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStoreInitSnapshotAndRollback(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)
	if err := store.Init(DefaultAgent); err != nil {
		t.Fatal(err)
	}
	state, err := store.LoadState(DefaultAgent)
	if err != nil || state.Version != 1 {
		t.Fatalf("initial state = %+v, err=%v", state, err)
	}
	agentDir := filepath.Join(store.Root, "agents", DefaultAgent)
	if err := os.WriteFile(filepath.Join(agentDir, "AGENTS.md"), []byte("baseline\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Snapshot(DefaultAgent); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(agentDir, "AGENTS.md"), []byte("candidate\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	state.Version = 2
	if err := store.SaveState(state); err != nil {
		t.Fatal(err)
	}
	if err := store.Rollback(DefaultAgent, 1); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(agentDir, "AGENTS.md"))
	if err != nil || string(data) != "baseline\n" {
		t.Fatalf("rolled back AGENTS.md = %q, err=%v", data, err)
	}
}

func TestSnapshotExcludesSecrets(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)
	if err := store.Init(DefaultAgent); err != nil {
		t.Fatal(err)
	}
	agentDir := filepath.Join(store.Root, "agents", DefaultAgent)
	if err := os.WriteFile(filepath.Join(agentDir, ".env"), []byte("SECRET=do-not-copy"), 0o600); err != nil {
		t.Fatal(err)
	}
	path, err := store.Snapshot(DefaultAgent)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 || bytes.Contains(data, []byte("SECRET=do-not-copy")) {
		t.Fatal("snapshot unexpectedly contains raw secret")
	}
}

func TestBenchmarkValidationAndScoreGate(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)
	if err := store.Init(DefaultAgent); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "benchmark.toml")
	content := "name = \"go-fix\"\nversion = 1\n\n[[cases]]\nid = \"compile\"\ntask = \"Fix the compile error\"\nverify_commands = [\"go test ./...\"]\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	benchmark, err := store.AddBenchmark(path)
	if err != nil || len(benchmark.Cases) != 1 {
		t.Fatalf("benchmark = %+v, err=%v", benchmark, err)
	}
	base, err := CalculateScore(70, 80, 70, 60, true)
	if err != nil {
		t.Fatal(err)
	}
	candidate, err := CalculateScore(80, 80, 70, 60, true)
	if err != nil || !AcceptCandidate(base, candidate) {
		t.Fatalf("candidate should be accepted: base=%+v candidate=%+v err=%v", base, candidate, err)
	}
	failing, err := CalculateScore(100, 100, 100, 100, false)
	if err != nil || AcceptCandidate(base, failing) {
		t.Fatalf("hard-gated candidate should be rejected: %+v", failing)
	}
}

func TestBenchmarkRejectsUnsafePaths(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "unsafe.toml")
	if err := os.WriteFile(path, []byte("name=\"unsafe\"\n[[cases]]\nid=\"x\"\ntask=\"x\"\nexpected_files=[\"../secret\"]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadBenchmark(path); err == nil {
		t.Fatal("expected traversal path to be rejected")
	}
}

func TestOverlayIsOptInAndLoadsSkills(t *testing.T) {
	root := t.TempDir()
	state := filepath.Join(root, ".vcode", "evolution", "agents", "build")
	if err := os.MkdirAll(filepath.Join(state, "skills"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(state, "AGENTS.md"), []byte("keep tasks small"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(state, "skills", "verify.md"), []byte("run checks"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("VCODE_EVOLUTION_AGENT", "")
	if got, err := LoadOverlay(root, "build"); err != nil || got != "" {
		t.Fatalf("overlay should be disabled: %q, %v", got, err)
	}
	t.Setenv("VCODE_EVOLUTION_AGENT", "build")
	got, err := LoadOverlay(root, "build")
	if err != nil || !strings.Contains(got, "keep tasks small") || !strings.Contains(got, "verify.md") {
		t.Fatalf("overlay missing state: %q, %v", got, err)
	}
}
