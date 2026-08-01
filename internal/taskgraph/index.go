package taskgraph

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type IndexEntry struct {
	ID          string    `json:"id"`
	Goal        string    `json:"goal"`
	Status      Status    `json:"status"`
	ProjectRoot string    `json:"project_root"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Index struct {
	path string
	mu   sync.Mutex
}

func NewIndex(vcodeHome string) *Index {
	return &Index{path: filepath.Join(vcodeHome, "tasks", "index.json")}
}

func (i *Index) Path() string { return i.path }

func (i *Index) List() ([]IndexEntry, error) {
	i.mu.Lock()
	defer i.mu.Unlock()
	entries, err := i.readLocked()
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	sort.Slice(entries, func(a, b int) bool { return entries[a].UpdatedAt.After(entries[b].UpdatedAt) })
	return entries, nil
}

func (i *Index) Upsert(t Task) error {
	i.mu.Lock()
	defer i.mu.Unlock()
	entries, err := i.readLocked()
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	entry := IndexEntry{ID: t.ID, Goal: t.Goal, Status: t.Status, ProjectRoot: t.ProjectRoot, UpdatedAt: t.UpdatedAt}
	found := false
	for n := range entries {
		if entries[n].ID == entry.ID && strings.EqualFold(entries[n].ProjectRoot, entry.ProjectRoot) {
			entries[n] = entry
			found = true
			break
		}
	}
	if !found {
		entries = append(entries, entry)
	}
	return i.writeLocked(entries)
}

func (i *Index) readLocked() ([]IndexEntry, error) {
	data, err := os.ReadFile(i.path)
	if err != nil {
		return nil, err
	}
	var entries []IndexEntry
	return entries, json.Unmarshal(data, &entries)
}

func (i *Index) writeLocked(entries []IndexEntry) error {
	if err := os.MkdirAll(filepath.Dir(i.path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(i.path), ".index-*.tmp")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(name, i.path)
}
