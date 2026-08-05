package bridge

import (
	"os"
	"path/filepath"
	"testing"

	"vcode/internal/runtime"
)

func TestStorePersistsTargetProjectsAndPairing(t *testing.T) {
	root := t.TempDir()
	projects := filepath.Join(root, "project")
	if err := os.Mkdir(projects, 0o700); err != nil {
		t.Fatal(err)
	}
	s, err := Open(filepath.Join(root, "bridge"))
	if err != nil {
		t.Fatal(err)
	}
	p, err := s.AddProject("demo", projects, false)
	if err != nil || p.ID == "" {
		t.Fatalf("add project = %+v, %v", p, err)
	}
	if err := s.SetStatus(runtime.TargetOnline); err != nil {
		t.Fatal(err)
	}
	code, err := s.NewPairing()
	if err != nil || code.Code == "" {
		t.Fatalf("pairing = %+v, %v", code, err)
	}
	s2, err := Open(filepath.Join(root, "bridge"))
	if err != nil {
		t.Fatal(err)
	}
	if s2.Snapshot().Status != runtime.TargetOnline || len(s2.ProjectsSnapshot()) != 1 || s2.PairingSnapshot() == nil {
		t.Fatalf("state did not persist: target=%+v projects=%v pairing=%v", s2.Snapshot(), s2.ProjectsSnapshot(), s2.PairingSnapshot())
	}
}
