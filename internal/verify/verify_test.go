package verify

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

func TestRunReportsCancellationAsVerificationEvidence(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
	defer cancel()
	result := Run(ctx, root)
	if result.Status != Partial || len(result.Failed) == 0 {
		t.Fatalf("result=%+v, want partial failure evidence", result)
	}
	if !strings.Contains(result.Failed[0], "verification stopped") {
		t.Fatalf("failure=%q, want cancellation detail", result.Failed[0])
	}
}
