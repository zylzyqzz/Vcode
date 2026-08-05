package serve

import (
	"os"
	"path/filepath"
	"testing"
)

func TestProjectStorePersistsAndValidates(t *testing.T) {
	root := t.TempDir()
	store := newProjectStore(filepath.Join(root, "sessions"))
	if err := store.upsert(ProjectRecord{ID: "demo", RepositoryURL: "https://github.com/acme/demo.git", Root: filepath.Join(store.root, "demo")}); err != nil {
		t.Fatal(err)
	}
	reloaded := newProjectStore(filepath.Join(root, "sessions"))
	if err := reloaded.load(); err != nil {
		t.Fatal(err)
	}
	p, ok := reloaded.get("demo")
	if !ok || p.DefaultBranch != "main" {
		t.Fatalf("project was not persisted: %#v %v", p, ok)
	}
	if _, err := os.Stat(filepath.Join(root, "projects.json")); err != nil {
		t.Fatal(err)
	}
}

func TestProjectValidationAndBranchProtection(t *testing.T) {
	for _, raw := range []string{"https://github.com/acme/demo", "ssh://git@example.com/acme/demo"} {
		if err := validateRepositoryURL(raw); err != nil {
			t.Errorf("valid URL rejected: %q: %v", raw, err)
		}
	}
	for _, raw := range []string{"file:///tmp/demo", "https://github.com/acme/demo\n", "not-a-url"} {
		if err := validateRepositoryURL(raw); err == nil {
			t.Errorf("invalid URL accepted: %q", raw)
		}
	}
	for _, branch := range []string{"main", "master", "-bad", "feature bad"} {
		if err := safeBranchName(branch); err == nil {
			t.Errorf("protected branch accepted: %q", branch)
		}
	}
	if err := safeBranchName("vcode/task-1"); err != nil {
		t.Fatal(err)
	}
	if projectPathWithin(filepath.Join("/tmp", "projects"), filepath.Join("/tmp", "projects2")) {
		t.Fatal("path prefix escaped project root")
	}
}
