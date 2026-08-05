// Package bridge stores the local computer target, approved projects, pairing
// requests, and durable bridge metadata. It intentionally contains no Agent
// execution logic; that remains owned by the existing Vcode runtime.
package bridge

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"vcode/internal/fileutil"
	"vcode/internal/runtime"
)

type Store struct {
	Root string

	mu         sync.Mutex
	Target     runtime.RuntimeTarget   `json:"target"`
	Projects   []runtime.LocalProject  `json:"projects"`
	Pairing    *runtime.PairingRequest `json:"pairing,omitempty"`
	RelayURL   string                  `json:"relay_url,omitempty"`
	RelayToken string                  `json:"relay_token,omitempty"`
}

func (s *Store) RelayConfig() (string, string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.RelayURL, s.RelayToken
}

func (s *Store) SetRelayConfig(url, token string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.RelayURL = strings.TrimSpace(url)
	s.RelayToken = strings.TrimSpace(token)
	return s.saveLocked()
}

func DefaultRoot() string {
	if root := strings.TrimSpace(os.Getenv("VCODE_STATE_HOME")); root != "" {
		return filepath.Join(root, "bridge")
	}
	if root := strings.TrimSpace(os.Getenv("VCODE_HOME")); root != "" {
		return filepath.Join(root, "bridge")
	}
	if root := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME")); root != "" {
		return filepath.Join(root, "vcode", "bridge")
	}
	if root, err := os.UserConfigDir(); err == nil && root != "" {
		return filepath.Join(root, "vcode", "bridge")
	}
	return ""
}

func Open(root string) (*Store, error) {
	if strings.TrimSpace(root) == "" {
		root = DefaultRoot()
	}
	if root == "" {
		return nil, errors.New("vcode bridge state directory is unavailable")
	}
	s := &Store{Root: filepath.Clean(root)}
	if err := os.MkdirAll(s.Root, 0o700); err != nil {
		return nil, err
	}
	if err := s.load(); err != nil {
		return nil, err
	}
	if s.Target.ID == "" {
		s.Target = runtime.RuntimeTarget{
			ID:       "pc-" + shortID(),
			Kind:     runtime.TargetLocalComputer,
			Name:     machineName(),
			Status:   runtime.TargetOffline,
			Features: []string{"tasks", "sessions", "diff", "verification"},
		}
		if err := s.saveLocked(); err != nil {
			return nil, err
		}
	}
	return s, nil
}

func (s *Store) path() string { return filepath.Join(s.Root, "state.json") }

func (s *Store) load() error {
	b, err := os.ReadFile(s.path())
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	return json.Unmarshal(b, s)
}

func (s *Store) saveLocked() error {
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return fileutil.AtomicWriteFile(s.path(), append(b, '\n'), 0o600)
}

func (s *Store) Save() error { s.mu.Lock(); defer s.mu.Unlock(); return s.saveLocked() }

func (s *Store) Snapshot() runtime.RuntimeTarget {
	s.mu.Lock()
	defer s.mu.Unlock()
	t := s.Target
	t.Features = append([]string(nil), s.Target.Features...)
	return t
}

func (s *Store) SetStatus(status string) error {
	if status != runtime.TargetOnline && status != runtime.TargetOffline && status != runtime.TargetBusy {
		return fmt.Errorf("invalid bridge status %q", status)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Target.Status = status
	s.Target.LastSeen = time.Now().UTC()
	return s.saveLocked()
}

func (s *Store) ProjectsSnapshot() []runtime.LocalProject {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]runtime.LocalProject(nil), s.Projects...)
}

func (s *Store) AddProject(name, root string, readOnly bool) (runtime.LocalProject, error) {
	root, err := filepath.Abs(strings.TrimSpace(root))
	if err != nil || root == "" {
		return runtime.LocalProject{}, errors.New("invalid project path")
	}
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return runtime.LocalProject{}, errors.New("project path is not a directory")
	}
	if strings.TrimSpace(name) == "" {
		name = filepath.Base(root)
	}
	p := runtime.LocalProject{ID: "project-" + shortID(), Name: strings.TrimSpace(name), Root: filepath.Clean(root), ReadOnly: readOnly}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, old := range s.Projects {
		if strings.EqualFold(old.Root, p.Root) {
			return old, nil
		}
	}
	s.Projects = append(s.Projects, p)
	if err := s.saveLocked(); err != nil {
		return runtime.LocalProject{}, err
	}
	return p, nil
}

func (s *Store) RemoveProject(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, p := range s.Projects {
		if p.ID == id || p.Name == id {
			s.Projects = append(s.Projects[:i], s.Projects[i+1:]...)
			return s.saveLocked()
		}
	}
	return os.ErrNotExist
}

func (s *Store) NewPairing() (runtime.PairingRequest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return runtime.PairingRequest{}, err
	}
	code := strings.ToUpper(hex.EncodeToString(b))
	p := runtime.PairingRequest{Code: code, TargetID: s.Target.ID, CreatedAt: time.Now().UTC(), ExpiresAt: time.Now().UTC().Add(5 * time.Minute)}
	s.Pairing = &p
	if err := s.saveLocked(); err != nil {
		return runtime.PairingRequest{}, err
	}
	return p, nil
}

func (s *Store) PairingSnapshot() *runtime.PairingRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Pairing == nil {
		return nil
	}
	p := *s.Pairing
	return &p
}

func (s *Store) ClearPairing() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Pairing = nil
	return s.saveLocked()
}

func shortID() string {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

func machineName() string {
	if n, err := os.Hostname(); err == nil && strings.TrimSpace(n) != "" {
		return n
	}
	return "Vcode Computer"
}
