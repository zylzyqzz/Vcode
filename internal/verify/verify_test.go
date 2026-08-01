package verify

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPlanGoProject(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	checks := Plan(root)
	if len(checks) != 3 || checks[0].Command != "go test ./..." || checks[2].Command != "go build ./..." {
		t.Fatalf("unexpected checks: %+v", checks)
	}
}

func TestPlanNodeUsesExistingScripts(t *testing.T) {
	root := t.TempDir()
	data := []byte(`{"scripts":{"test":"vitest","build":"vite"}}`)
	if err := os.WriteFile(filepath.Join(root, "package.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	checks := Plan(root)
	if len(checks) != 2 || checks[0].Command != "npm run test" || checks[1].Command != "npm run build" {
		t.Fatalf("unexpected checks: %+v", checks)
	}
}

func TestPlanUnknownProjectIsUnverified(t *testing.T) {
	if got := Plan(t.TempDir()); got != nil {
		t.Fatalf("checks = %+v, want nil", got)
	}
}
