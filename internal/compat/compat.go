// Package compat discovers and normalizes agent assets authored for Vcode and
// compatible coding agents. It deliberately stops at data and policy
// translation: execution remains owned by Vcode's MCP, Skill, Hook, and Agent
// runtimes.
package compat

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"vcode/internal/frontmatter"
)

type Source struct {
	Kind     string `json:"kind"`
	Path     string `json:"path"`
	Priority int    `json:"priority"`
}

type Instruction struct {
	Source Source `json:"source"`
	Name   string `json:"name"`
	Body   string `json:"body"`
}

type SkillSpec struct {
	Source      Source `json:"source"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Path        string `json:"path"`
}

type Rule struct {
	Action   string `json:"action"`
	Resource string `json:"resource"`
	Effect   string `json:"effect"`
}

type AgentSpec struct {
	Source      Source   `json:"source"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Model       string   `json:"model,omitempty"`
	Tools       []string `json:"tools,omitempty"`
	Permissions []Rule   `json:"permissions,omitempty"`
	MaxSteps    int      `json:"max_steps,omitempty"`
	Body        string   `json:"body,omitempty"`
}

type MCPSource struct {
	Source    Source            `json:"source"`
	Name      string            `json:"name"`
	Transport string            `json:"transport"`
	Command   string            `json:"command,omitempty"`
	Args      []string          `json:"args,omitempty"`
	URL       string            `json:"url,omitempty"`
	Headers   map[string]string `json:"headers,omitempty"`
	Env       map[string]string `json:"env,omitempty"`
}

type HookSpec struct {
	Source  Source `json:"source"`
	Event   string `json:"event"`
	Match   string `json:"match,omitempty"`
	Command string `json:"command"`
}

type Report struct {
	Root         string        `json:"root"`
	Instructions []Instruction `json:"instructions,omitempty"`
	Skills       []SkillSpec   `json:"skills,omitempty"`
	Agents       []AgentSpec   `json:"agents,omitempty"`
	MCPServers   []MCPSource   `json:"mcp_servers,omitempty"`
	Hooks        []HookSpec    `json:"hooks,omitempty"`
	Warnings     []string      `json:"warnings,omitempty"`
}

type assetRoot struct {
	Path     string
	Kind     string
	Priority int
}

// Scan discovers project-local compatibility assets. The order is explicit:
// Vcode wins over Codex, Claude, OpenCode, and generic agent directories when
// two assets use the same name.
func Scan(root string) (Report, error) {
	if strings.TrimSpace(root) == "" {
		root = "."
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return Report{}, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return Report{}, err
	}
	if !info.IsDir() {
		return Report{}, fmt.Errorf("compat root is not a directory: %s", abs)
	}

	r := Report{Root: abs}
	roots := []assetRoot{
		{abs, "project", 120},
		{filepath.Join(abs, ".vcode"), "vcode", 110},
		{filepath.Join(abs, ".codex"), "codex", 100},
		{filepath.Join(abs, ".claude"), "claude", 90},
		{filepath.Join(abs, ".opencode"), "opencode", 80},
		{filepath.Join(abs, ".agents"), "agents", 70},
		{filepath.Join(abs, ".agent"), "agent", 60},
	}
	seenInstructions := map[string]bool{}
	seenSkills := map[string]bool{}
	seenAgents := map[string]bool{}
	seenMCP := map[string]bool{}
	for _, ar := range roots {
		discoverInstructions(&r, ar, seenInstructions)
		discoverSkills(&r, ar, seenSkills)
		discoverAgents(&r, ar, seenAgents)
		discoverMCP(&r, ar, seenMCP)
		discoverHooks(&r, ar)
	}
	sort.Slice(r.Instructions, func(i, j int) bool { return r.Instructions[i].Source.Priority > r.Instructions[j].Source.Priority })
	sort.Slice(r.Skills, func(i, j int) bool { return r.Skills[i].Name < r.Skills[j].Name })
	sort.Slice(r.Agents, func(i, j int) bool { return r.Agents[i].Name < r.Agents[j].Name })
	sort.Slice(r.MCPServers, func(i, j int) bool { return r.MCPServers[i].Name < r.MCPServers[j].Name })
	sort.Slice(r.Hooks, func(i, j int) bool { return r.Hooks[i].Event < r.Hooks[j].Event })
	return r, nil
}

func source(ar assetRoot, path string) Source {
	return Source{Kind: ar.Kind, Path: path, Priority: ar.Priority}
}

func discoverInstructions(r *Report, ar assetRoot, seen map[string]bool) {
	names := []string{"VCODE.md", "AGENTS.md", "CLAUDE.md"}
	dirs := []string{ar.Path}
	if ar.Kind != "project" {
		dirs = append(dirs, filepath.Dir(ar.Path))
	}
	for _, dir := range dirs {
		for _, name := range names {
			path := filepath.Join(dir, name)
			if seen[path] {
				continue
			}
			body, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			seen[path] = true
			r.Instructions = append(r.Instructions, Instruction{Source: source(ar, path), Name: name, Body: string(body)})
		}
	}
}

func discoverSkills(r *Report, ar assetRoot, seen map[string]bool) {
	dir := filepath.Join(ar.Path, "skills")
	_ = filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if entry.IsDir() || !strings.EqualFold(filepath.Base(path), "SKILL.md") {
			return nil
		}
		if seen[path] {
			return nil
		}
		body, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}
		fm, _ := frontmatter.Split(string(body))
		name := strings.TrimSpace(fm["name"])
		if name == "" {
			name = filepath.Base(filepath.Dir(path))
		}
		if name == "" || strings.ContainsAny(name, `/\\`) {
			return nil
		}
		if seen[name] {
			r.Warnings = append(r.Warnings, fmt.Sprintf("skill %q shadowed by %s", name, path))
			return nil
		}
		seen[path] = true
		seen[name] = true
		r.Skills = append(r.Skills, SkillSpec{Source: source(ar, path), Name: name, Description: strings.TrimSpace(fm["description"]), Path: path})
		return nil
	})
}

func discoverAgents(r *Report, ar assetRoot, seen map[string]bool) {
	dirs := []string{filepath.Join(ar.Path, "agents"), filepath.Join(ar.Path, "agent")}
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".md") {
				continue
			}
			path := filepath.Join(dir, entry.Name())
			if seen[path] {
				continue
			}
			body, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			fm, content := frontmatter.Split(string(body))
			name := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
			spec := AgentSpec{Source: source(ar, path), Name: name, Description: fm["description"], Model: fm["model"], Body: strings.TrimSpace(content)}
			spec.Tools = splitList(fm["tools"])
			spec.MaxSteps = atoi(fm["max_steps"])
			if p := strings.TrimSpace(fm["permissions"]); p != "" {
				spec.Permissions = []Rule{{Action: "*", Resource: "*", Effect: p}}
			}
			if seen[name] {
				r.Warnings = append(r.Warnings, fmt.Sprintf("agent %q shadowed by %s", name, path))
				continue
			}
			seen[path] = true
			seen[name] = true
			r.Agents = append(r.Agents, spec)
		}
	}
}

func discoverMCP(r *Report, ar assetRoot, seen map[string]bool) {
	paths := []string{}
	if ar.Kind == "project" {
		paths = append(paths, filepath.Join(ar.Path, ".mcp.json"), filepath.Join(ar.Path, "opencode.json"), filepath.Join(ar.Path, "opencode.jsonc"))
	}
	for _, path := range paths {
		if seen[path] {
			continue
		}
		b, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		seen[path] = true
		var doc map[string]json.RawMessage
		if err := json.Unmarshal(b, &doc); err != nil {
			if errJSONC := json.Unmarshal(stripJSONComments(b), &doc); errJSONC != nil {
				r.Warnings = append(r.Warnings, fmt.Sprintf("%s: invalid JSON/JSONC: %v", path, errJSONC))
				continue
			}
		}
		var servers map[string]json.RawMessage
		for _, key := range []string{"mcpServers", "mcp"} {
			if raw, ok := doc[key]; ok {
				var nested struct {
					Servers map[string]json.RawMessage `json:"servers"`
				}
				if key == "mcp" && json.Unmarshal(raw, &nested) == nil && nested.Servers != nil {
					servers = nested.Servers
				} else {
					_ = json.Unmarshal(raw, &servers)
				}
				if servers != nil {
					break
				}
			}
		}
		for name, raw := range servers {
			var spec struct {
				Type    string            `json:"type"`
				Command string            `json:"command"`
				Args    []string          `json:"args"`
				URL     string            `json:"url"`
				Headers map[string]string `json:"headers"`
				Env     map[string]string `json:"env"`
			}
			if err := json.Unmarshal(raw, &spec); err != nil {
				r.Warnings = append(r.Warnings, fmt.Sprintf("%s server %s: %v", path, name, err))
				continue
			}
			if seen[name] {
				continue
			}
			transport := spec.Type
			if transport == "" {
				if spec.URL != "" {
					transport = "http"
				} else {
					transport = "stdio"
				}
			}
			seen[name] = true
			r.MCPServers = append(r.MCPServers, MCPSource{Source: source(ar, path), Name: name, Transport: transport, Command: spec.Command, Args: spec.Args, URL: spec.URL, Headers: spec.Headers, Env: spec.Env})
		}
	}
}

func discoverHooks(r *Report, ar assetRoot) {
	for _, path := range []string{filepath.Join(ar.Path, "settings.json"), filepath.Join(ar.Path, "settings.local.json")} {
		b, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var doc struct {
			Hooks map[string]json.RawMessage `json:"hooks"`
		}
		if err := json.Unmarshal(b, &doc); err != nil {
			r.Warnings = append(r.Warnings, fmt.Sprintf("%s: invalid JSON: %v", path, err))
			continue
		}
		for event, raw := range doc.Hooks {
			var groups []struct {
				Matcher string `json:"matcher"`
				Hooks   []struct {
					Type    string `json:"type"`
					Command string `json:"command"`
				} `json:"hooks"`
			}
			if json.Unmarshal(raw, &groups) == nil {
				for _, group := range groups {
					for _, h := range group.Hooks {
						if h.Command != "" {
							r.Hooks = append(r.Hooks, HookSpec{Source: source(ar, path), Event: event, Match: group.Matcher, Command: h.Command})
						}
					}
				}
			}
		}
	}
}

func splitList(v string) []string {
	var out []string
	for _, item := range strings.FieldsFunc(v, func(r rune) bool { return r == ',' || r == ' ' || r == '\t' }) {
		if item != "" {
			out = append(out, item)
		}
	}
	return out
}
func atoi(v string) int { var n int; _, _ = fmt.Sscanf(strings.TrimSpace(v), "%d", &n); return n }
func stripJSONComments(b []byte) []byte {
	out := make([]byte, 0, len(b))
	inString, escaped, lineComment, blockComment := false, false, false, false
	for i := 0; i < len(b); i++ {
		c := b[i]
		if lineComment {
			if c == '\n' {
				lineComment = false
				out = append(out, c)
			} else {
				out = append(out, ' ')
			}
			continue
		}
		if blockComment {
			if c == '*' && i+1 < len(b) && b[i+1] == '/' {
				out = append(out, ' ', ' ')
				i++
			} else if c == '\n' {
				out = append(out, '\n')
			} else {
				out = append(out, ' ')
			}
			continue
		}
		if inString {
			out = append(out, c)
			if escaped {
				escaped = false
			} else if c == '\\' {
				escaped = true
			} else if c == '"' {
				inString = false
			}
			continue
		}
		if c == '"' {
			inString = true
			out = append(out, c)
		} else if c == '/' && i+1 < len(b) && b[i+1] == '/' {
			lineComment = true
			out = append(out, ' ', ' ')
			i++
		} else if c == '/' && i+1 < len(b) && b[i+1] == '*' {
			blockComment = true
			out = append(out, ' ', ' ')
			i++
		} else {
			out = append(out, c)
		}
	}
	return out
}
