package evolution

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"vcode/internal/fileutil"
)

type Store struct {
	Root string
}

func NewStore(projectRoot string) *Store {
	return &Store{Root: filepath.Join(projectRoot, ".vcode", "evolution")}
}

func (s *Store) Validate() error {
	if s == nil || strings.TrimSpace(s.Root) == "" {
		return errors.New("evolution root is empty")
	}
	return nil
}

func (s *Store) Init(agent string) error {
	if err := s.Validate(); err != nil {
		return err
	}
	agent = normalizeAgent(agent)
	paths := []string{
		s.Root,
		filepath.Join(s.Root, "agents", agent, "skills"),
		filepath.Join(s.Root, "agents", agent, "snapshots"),
		filepath.Join(s.Root, "benchmarks"),
		filepath.Join(s.Root, "runs"),
	}
	for _, path := range paths {
		if err := os.MkdirAll(path, 0o755); err != nil {
			return err
		}
	}
	if _, err := os.Stat(s.configPath()); errors.Is(err, os.ErrNotExist) {
		body := "enabled = true\nagent = \"" + agent + "\"\nrounds = 3\nrepeats = 2\n"
		if err := fileutil.AtomicWriteFile(s.configPath(), []byte(body), 0o644); err != nil {
			return err
		}
	}
	if _, err := os.Stat(s.statePath(agent)); errors.Is(err, os.ErrNotExist) {
		if err := s.saveJSON(s.statePath(agent), State{Agent: agent, Version: 1, Status: "ready", UpdatedAt: time.Now().UTC()}); err != nil {
			return err
		}
	}
	if _, err := os.Stat(filepath.Join(s.Root, "benchmarks", "example.toml")); errors.Is(err, os.ErrNotExist) {
		const example = `name = "example"
version = 1

[[cases]]
id = "add-marker"
task = "Create a file named evolution-example.txt containing one short line that says Vcode evolution works."
expected_files = ["evolution-example.txt"]
verify_commands = []
repeats = 2
`
		if err := fileutil.AtomicWriteFile(filepath.Join(s.Root, "benchmarks", "example.toml"), []byte(example), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) configPath() string { return filepath.Join(s.Root, "config.toml") }
func (s *Store) statePath(agent string) string {
	return filepath.Join(s.Root, "agents", normalizeAgent(agent), "version.json")
}
func (s *Store) agentPath(agent string) string {
	return filepath.Join(s.Root, "agents", normalizeAgent(agent))
}

func (s *Store) LoadState(agent string) (State, error) {
	var state State
	if err := readJSON(s.statePath(agent), &state); err != nil {
		return State{}, err
	}
	if state.Agent == "" {
		state.Agent = normalizeAgent(agent)
	}
	if state.Version < 1 {
		return State{}, fmt.Errorf("invalid evolution version %d", state.Version)
	}
	return state, nil
}

func (s *Store) SaveState(state State) error {
	state.Agent = normalizeAgent(state.Agent)
	if state.Version < 1 {
		return fmt.Errorf("invalid evolution version %d", state.Version)
	}
	state.UpdatedAt = time.Now().UTC()
	return s.saveJSON(s.statePath(state.Agent), state)
}

func (s *Store) AppendHistory(run Run) error {
	if err := os.MkdirAll(s.Root, 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(run)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(s.Root, "history.jsonl"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(append(data, '\n'))
	return err
}

func (s *Store) Snapshot(agent string) (string, error) {
	state, err := s.LoadState(agent)
	if err != nil {
		return "", err
	}
	snapDir := filepath.Join(s.agentPath(agent), "snapshots")
	if err := os.MkdirAll(snapDir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(snapDir, "v"+strconv.Itoa(state.Version)+".tar.gz")
	if _, err := os.Stat(path); err == nil {
		return path, nil
	}
	tmp, err := os.CreateTemp(snapDir, ".snapshot-*.tmp")
	if err != nil {
		return "", err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := writeSnapshot(tmp, s.agentPath(agent)); err != nil {
		tmp.Close()
		return "", err
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return "", err
	}
	return path, nil
}

func (s *Store) Rollback(agent string, version int) error {
	if version < 1 {
		return fmt.Errorf("invalid rollback version %d", version)
	}
	path := filepath.Join(s.agentPath(agent), "snapshots", "v"+strconv.Itoa(version)+".tar.gz")
	archive, err := os.Open(path)
	if err != nil {
		return err
	}
	defer archive.Close()
	staging, err := os.MkdirTemp(filepath.Dir(s.agentPath(agent)), ".rollback-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(staging)
	if err := extractSnapshot(archive, staging); err != nil {
		return err
	}
	stateDir := filepath.Join(staging, "agent")
	if _, err := os.Stat(filepath.Join(stateDir, "version.json")); err != nil {
		return fmt.Errorf("snapshot missing version.json: %w", err)
	}
	current := s.agentPath(agent)
	backup := current + ".rollback-backup"
	_ = os.RemoveAll(backup)
	if err := os.Rename(current, backup); err != nil {
		// Windows can keep a directory handle open briefly (for example from an
		// editor or antivirus scan). Fall back to an in-place restore while
		// preserving the snapshot archive directory.
		if restoreErr := restoreAgentInPlace(stateDir, current); restoreErr != nil {
			return fmt.Errorf("rename agent state: %v; in-place restore: %w", err, restoreErr)
		}
		return nil
	}
	if err := os.Rename(stateDir, current); err != nil {
		_ = os.Rename(backup, current)
		return err
	}
	_ = os.RemoveAll(backup)
	return nil
}

func restoreAgentInPlace(source, dest string) error {
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}
	entries, err := os.ReadDir(dest)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.Name() == "snapshots" {
			continue
		}
		if err := os.RemoveAll(filepath.Join(dest, entry.Name())); err != nil {
			return err
		}
	}
	return copyTree(source, dest)
}

func copyTree(source, dest string) error {
	return filepath.Walk(source, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dest, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o600)
	})
}

func (s *Store) ListHistory() ([]Run, error) {
	path := filepath.Join(s.Root, "history.jsonl")
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []Run
	dec := json.NewDecoder(f)
	for {
		var run Run
		if err := dec.Decode(&run); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			return nil, err
		}
		out = append(out, run)
	}
	return out, nil
}

func (s *Store) LoadRun(id string) (Run, error) {
	var run Run
	if strings.TrimSpace(id) == "" {
		return run, errors.New("run id is empty")
	}
	if err := readJSON(filepath.Join(s.Root, "runs", id+".json"), &run); err != nil {
		return Run{}, err
	}
	return run, nil
}

func (s *Store) saveJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return fileutil.AtomicWriteFile(path, append(data, '\n'), 0o644)
}

func readJSON(path string, target any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}

func writeSnapshot(dst io.Writer, root string) error {
	gz := gzip.NewWriter(dst)
	tarWriter := tar.NewWriter(gz)
	err := filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == filepath.Join(root, "snapshots") || strings.Contains(filepath.Base(path), ".tmp") {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if info.Name() == ".env" || info.Name() == ".vault.toml" {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		name := filepath.ToSlash(filepath.Join("agent", rel))
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = name
		if err := tarWriter.WriteHeader(header); err != nil {
			return err
		}
		if info.Mode().IsRegular() {
			f, err := os.Open(path)
			if err != nil {
				return err
			}
			_, copyErr := io.Copy(tarWriter, f)
			closeErr := f.Close()
			if copyErr != nil {
				return copyErr
			}
			return closeErr
		}
		return nil
	})
	if closeErr := tarWriter.Close(); err == nil {
		err = closeErr
	}
	if closeErr := gz.Close(); err == nil {
		err = closeErr
	}
	return err
}

func extractSnapshot(src io.Reader, dest string) error {
	gz, err := gzip.NewReader(src)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		name := filepath.Clean(filepath.FromSlash(h.Name))
		if name != "agent" && !strings.HasPrefix(name, "agent"+string(filepath.Separator)) {
			return fmt.Errorf("snapshot path escapes agent root: %q", h.Name)
		}
		target := filepath.Join(dest, name)
		if h.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(f, tr)
		closeErr := f.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
}

func normalizeAgent(agent string) string {
	agent = strings.ToLower(strings.TrimSpace(agent))
	if agent == "" {
		return DefaultAgent
	}
	return agent
}
