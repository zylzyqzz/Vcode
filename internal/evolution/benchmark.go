package evolution

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"vcode/internal/fileutil"
)

func LoadBenchmark(path string) (Benchmark, error) {
	var benchmark Benchmark
	if _, err := toml.DecodeFile(path, &benchmark); err != nil {
		return Benchmark{}, err
	}
	benchmark.Name = strings.TrimSpace(benchmark.Name)
	if benchmark.Name == "" {
		benchmark.Name = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}
	if benchmark.Version == 0 {
		benchmark.Version = 1
	}
	if !validName(benchmark.Name) {
		return Benchmark{}, fmt.Errorf("invalid benchmark name %q", benchmark.Name)
	}
	if len(benchmark.Cases) == 0 {
		return Benchmark{}, errors.New("benchmark has no cases")
	}
	seen := map[string]bool{}
	for i := range benchmark.Cases {
		c := &benchmark.Cases[i]
		c.ID = strings.TrimSpace(c.ID)
		if c.ID == "" {
			return Benchmark{}, fmt.Errorf("benchmark case %d has no id", i+1)
		}
		if seen[c.ID] {
			return Benchmark{}, fmt.Errorf("duplicate benchmark case %q", c.ID)
		}
		seen[c.ID] = true
		if strings.TrimSpace(c.Task) == "" {
			return Benchmark{}, fmt.Errorf("benchmark case %q has no task", c.ID)
		}
		for field, paths := range map[string][]string{
			"fixture":        {c.Fixture},
			"allowed_paths":  c.AllowedPaths,
			"expected_files": c.ExpectedFiles,
		} {
			for _, path := range paths {
				if strings.TrimSpace(path) == "" {
					continue
				}
				if !validRelativePath(path) {
					return Benchmark{}, fmt.Errorf("benchmark case %q has unsafe %s path %q", c.ID, field, path)
				}
			}
		}
		if c.Repeats < 0 {
			return Benchmark{}, fmt.Errorf("benchmark case %q repeats cannot be negative", c.ID)
		}
	}
	return benchmark, nil
}

func validName(name string) bool {
	name = strings.TrimSpace(name)
	return name != "" && name != "." && name != ".." && !strings.ContainsAny(name, `/\\`)
}

func validRelativePath(path string) bool {
	path = strings.TrimSpace(path)
	if path == "" || filepath.IsAbs(path) {
		return false
	}
	clean := filepath.Clean(path)
	return clean != "." && clean != ".." && !strings.HasPrefix(clean, ".."+string(filepath.Separator)) && !strings.HasPrefix(clean, "../") && !strings.HasPrefix(clean, `..\\`)
}

func (s *Store) BenchmarkDir(name string) string {
	return filepath.Join(s.Root, "benchmarks", filepath.Clean(name))
}

func (s *Store) ListBenchmarks() ([]Benchmark, error) {
	root := filepath.Join(s.Root, "benchmarks")
	entries, err := os.ReadDir(root)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []Benchmark
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".toml" {
			continue
		}
		b, err := LoadBenchmark(filepath.Join(root, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("load benchmark %s: %w", entry.Name(), err)
		}
		out = append(out, b)
	}
	return out, nil
}

func (s *Store) AddBenchmark(source string) (Benchmark, error) {
	if err := s.Validate(); err != nil {
		return Benchmark{}, err
	}
	b, err := LoadBenchmark(source)
	if err != nil {
		return Benchmark{}, err
	}
	if filepath.Base(source) == "." || filepath.Base(source) == ".." {
		return Benchmark{}, errors.New("invalid benchmark path")
	}
	if err := os.MkdirAll(filepath.Join(s.Root, "benchmarks"), 0o755); err != nil {
		return Benchmark{}, err
	}
	dest := filepath.Join(s.Root, "benchmarks", b.Name+".toml")
	data, err := os.ReadFile(source)
	if err != nil {
		return Benchmark{}, err
	}
	if err := fileutil.AtomicWriteFile(dest, data, 0o644); err != nil {
		return Benchmark{}, err
	}
	return b, nil
}
