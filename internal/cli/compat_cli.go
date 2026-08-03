package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"vcode/internal/compat"
	"vcode/internal/config"
	"vcode/internal/skill"
)

func compatCommand(args []string) int {
	if len(args) == 0 {
		compatUsage()
		return 2
	}
	switch args[0] {
	case "doctor", "check":
		return compatDoctor(args[1:])
	case "list", "ls":
		return compatList()
	case "import":
		return compatImport()
	case "help", "-h", "--help":
		compatUsage()
		return 0
	default:
		fmt.Fprintf(os.Stderr, "unknown compat subcommand %q\n", args[0])
		compatUsage()
		return 2
	}
}

func scanCompat() (compat.Report, error) {
	root, err := os.Getwd()
	if err != nil {
		return compat.Report{}, err
	}
	return compat.Scan(root)
}

func compatDoctor(args []string) int {
	report, err := scanCompat()
	if err != nil {
		fmt.Fprintln(os.Stderr, "compat doctor:", err)
		return 1
	}
	for _, arg := range args {
		if arg == "--json" {
			data, marshalErr := json.MarshalIndent(report, "", "  ")
			if marshalErr != nil {
				fmt.Fprintln(os.Stderr, marshalErr)
				return 1
			}
			fmt.Println(string(data))
			return 0
		}
	}
	fmt.Printf("compat root: %s\n", report.Root)
	fmt.Printf("instructions: %d\nskills: %d\nagents: %d\nMCP servers: %d\nhooks: %d\n", len(report.Instructions), len(report.Skills), len(report.Agents), len(report.MCPServers), len(report.Hooks))
	for _, warning := range report.Warnings {
		fmt.Printf("warning: %s\n", warning)
	}
	return 0
}

func compatList() int {
	report, err := scanCompat()
	if err != nil {
		fmt.Fprintln(os.Stderr, "compat list:", err)
		return 1
	}
	for _, item := range report.Instructions {
		fmt.Printf("instruction %-10s %s\n", item.Name, item.Source.Path)
	}
	for _, item := range report.Skills {
		fmt.Printf("skill       %-20s %s\n", item.Name, item.Source.Path)
	}
	for _, item := range report.Agents {
		fmt.Printf("agent       %-20s %s\n", item.Name, item.Source.Path)
	}
	for _, item := range report.MCPServers {
		fmt.Printf("mcp         %-20s %-8s %s\n", item.Name, item.Transport, item.Source.Path)
	}
	for _, item := range report.Hooks {
		fmt.Printf("hook        %-20s %s\n", item.Event, item.Source.Path)
	}
	if len(report.Instructions)+len(report.Skills)+len(report.Agents)+len(report.MCPServers)+len(report.Hooks) == 0 {
		fmt.Println("no compatible assets found")
	}
	return 0
}

func compatImport() int {
	report, err := scanCompat()
	if err != nil {
		fmt.Fprintln(os.Stderr, "compat import:", err)
		return 1
	}
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	imported, added := 0, 0
	for _, server := range report.MCPServers {
		entry := config.PluginEntry{Name: server.Name, Type: server.Transport, Command: server.Command, Args: server.Args, URL: server.URL, Headers: server.Headers, Env: server.Env}
		before := len(cfg.Plugins)
		if err := cfg.UpsertPlugin(entry); err != nil {
			fmt.Fprintf(os.Stderr, "MCP %s: %v\n", server.Name, err)
			continue
		}
		imported++
		if len(cfg.Plugins) > before {
			added++
		}
	}
	if err := cfg.Save(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Printf("imported %d MCP servers into %s (%d new)\n", imported, filepath.Join(".", "vcode.toml"), added)
	fmt.Printf("discovered %d instructions, %d skills, %d agents, %d hooks; these remain source files and load automatically\n", len(report.Instructions), len(report.Skills), len(report.Agents), len(report.Hooks))
	return 0
}

func compatSkillCommand(args []string) int {
	if len(args) == 0 || args[0] == "list" || args[0] == "ls" {
		cfg, _ := config.Load()
		root, _ := os.Getwd()
		store := skill.New(skill.Options{ProjectRoot: root, CustomPaths: cfg.SkillCustomPaths(), ExcludedPaths: cfg.SkillExcludedPaths(), DisabledNames: cfg.DisabledSkillNames(), MaxDepth: cfg.SkillMaxDepth(), Stderr: os.Stderr})
		items := store.List()
		for _, item := range items {
			fmt.Printf("%-24s %-10s %s\n", item.Name, item.Scope, strings.TrimSpace(item.Description))
		}
		if len(items) == 0 {
			fmt.Println("no skills found")
		}
		return 0
	}
	fmt.Println("usage: vcode skill list")
	return 2
}

func compatAgentCommand(args []string) int {
	if len(args) == 0 || args[0] == "list" || args[0] == "ls" {
		report, err := scanCompat()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		for _, item := range report.Agents {
			fmt.Printf("%-24s %-10s %s\n", item.Name, item.Model, strings.TrimSpace(item.Description))
		}
		if len(report.Agents) == 0 {
			fmt.Println("no compatible agents found")
		}
		return 0
	}
	fmt.Println("usage: vcode agent list")
	return 2
}

func compatUsage() {
	fmt.Println(`Compatibility assets from OpenCode, Claude Code, Codex, and Vcode.

Usage:
  vcode compat doctor [--json]
  vcode compat list
  vcode compat import
  vcode skill list
  vcode agent list

compat import imports discovered MCP servers into vcode.toml. Instructions,
Skills, Agents, and Hooks remain in their source locations and are discovered
without copying or executing foreign runtime code.`)
}
