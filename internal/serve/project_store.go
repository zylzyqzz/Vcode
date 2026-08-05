package serve

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

var projectIDPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]{0,63}$`)

type ProjectRecord struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	RepositoryURL string    `json:"repository_url"`
	Root          string    `json:"root"`
	DefaultBranch string    `json:"default_branch"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type projectStore struct {
	mu       sync.Mutex
	root     string
	path     string
	projects map[string]ProjectRecord
}

func newProjectStore(sessionDir string) *projectStore {
	base := filepath.Dir(sessionDir)
	if strings.TrimSpace(sessionDir) == "" || base == "." || base == string(filepath.VolumeName(base)) {
		base = filepath.Join(".", ".vcode")
	}
	return &projectStore{root: filepath.Join(base, "projects"), path: filepath.Join(base, "projects.json"), projects: map[string]ProjectRecord{}}
}

func (s *projectStore) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.MkdirAll(s.root, 0o700); err != nil {
		return err
	}
	data, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var records []ProjectRecord
	if err := json.Unmarshal(data, &records); err != nil {
		return fmt.Errorf("parse projects: %w", err)
	}
	for _, p := range records {
		if projectIDPattern.MatchString(p.ID) {
			s.projects[p.ID] = p
		}
	}
	return nil
}

func (s *projectStore) list() []ProjectRecord {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]ProjectRecord, 0, len(s.projects))
	for _, p := range s.projects {
		out = append(out, p)
	}
	return out
}

func (s *projectStore) get(id string) (ProjectRecord, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.projects[id]
	return p, ok
}

func (s *projectStore) upsert(p ProjectRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !projectIDPattern.MatchString(p.ID) {
		return errors.New("invalid project id")
	}
	if p.DefaultBranch == "" {
		p.DefaultBranch = "main"
	}
	if p.CreatedAt.IsZero() {
		p.CreatedAt = time.Now().UTC()
	}
	p.UpdatedAt = time.Now().UTC()
	s.projects[p.ID] = p
	return s.saveLocked()
}

func (s *projectStore) saveLocked() error {
	list := make([]ProjectRecord, 0, len(s.projects))
	for _, p := range s.projects {
		list = append(list, p)
	}
	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, s.path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

func validateRepositoryURL(raw string) error {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" || strings.ContainsAny(raw, "\r\n") {
		return errors.New("repository_url must be a valid https or ssh URL")
	}
	if u.Scheme != "https" && u.Scheme != "http" && u.Scheme != "ssh" && u.Scheme != "git" {
		return errors.New("repository_url scheme is not allowed")
	}
	return nil
}

func runGit(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, redactGitOutput(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

func redactGitOutput(value string) string {
	for _, key := range []string{"GITHUB_TOKEN", "DEEPSEEK_API_KEY"} {
		if secret := os.Getenv(key); secret != "" {
			value = strings.ReplaceAll(value, secret, "[REDACTED]")
		}
	}
	return strings.TrimSpace(value)
}

func projectPathWithin(root, path string) bool {
	r, err1 := filepath.Abs(root)
	p, err2 := filepath.Abs(path)
	if err1 != nil || err2 != nil {
		return false
	}
	r = filepath.Clean(r)
	p = filepath.Clean(p)
	return p == r || strings.HasPrefix(p, r+string(filepath.Separator))
}

func safeBranchName(name string) error {
	if name == "" || name == "main" || name == "master" || strings.ContainsAny(name, "\r\n ") || strings.HasPrefix(name, "-") {
		return errors.New("branch is invalid or protected")
	}
	return nil
}
