package evolution

import "time"

const (
	DefaultAgent   = "build"
	DefaultRounds  = 3
	DefaultRepeats = 2
)

type Config struct {
	Enabled   bool   `toml:"enabled"`
	Agent     string `toml:"agent"`
	Rounds    int    `toml:"rounds"`
	Repeats   int    `toml:"repeats"`
	Benchmark string `toml:"benchmark"`
}

type State struct {
	Agent       string    `json:"agent"`
	Version     int       `json:"version"`
	Status      string    `json:"status"`
	UpdatedAt   time.Time `json:"updated_at"`
	AcceptedRun string    `json:"accepted_run,omitempty"`
}

type Benchmark struct {
	Name    string `toml:"name"`
	Version int    `toml:"version"`
	Cases   []Case `toml:"cases"`
}

type Case struct {
	ID             string   `toml:"id"`
	Task           string   `toml:"task"`
	Fixture        string   `toml:"fixture"`
	AllowedPaths   []string `toml:"allowed_paths"`
	ExpectedFiles  []string `toml:"expected_files"`
	VerifyCommands []string `toml:"verify_commands"`
	Repeats        int      `toml:"repeats"`
}

type Run struct {
	ID         string    `json:"id"`
	Agent      string    `json:"agent"`
	Benchmark  string    `json:"benchmark"`
	Version    int       `json:"version"`
	Status     string    `json:"status"`
	Score      float64   `json:"score"`
	StartedAt  time.Time `json:"started_at"`
	FinishedAt time.Time `json:"finished_at,omitempty"`
	Reason     string    `json:"reason,omitempty"`
	Cases      []CaseRun `json:"cases,omitempty"`
}

type CaseRun struct {
	CaseID       string   `json:"case_id"`
	Repeat       int      `json:"repeat"`
	Workspace    string   `json:"workspace"`
	ExitCode     int      `json:"exit_code"`
	ChangedFiles []string `json:"changed_files,omitempty"`
	Verification string   `json:"verification"`
	Score        Score    `json:"score"`
	Output       string   `json:"output,omitempty"`
}

type Score struct {
	Completion float64 `json:"completion"`
	Verify     float64 `json:"verification"`
	Quality    float64 `json:"quality"`
	Efficiency float64 `json:"efficiency"`
	Total      float64 `json:"total"`
	HardGate   bool    `json:"hard_gate"`
	Reason     string  `json:"reason,omitempty"`
}
