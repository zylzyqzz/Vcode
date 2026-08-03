package compat

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeCompatFile(t *testing.T, root, name, body string) {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestScanDiscoversCompatibleAssets(t *testing.T) {
	root := t.TempDir()
	writeCompatFile(t, root, "AGENTS.md", "shared instructions")
	writeCompatFile(t, root, ".vcode/VCODE.md", "canonical instructions")
	writeCompatFile(t, root, ".opencode/skills/review/SKILL.md", "---\nname: review\ndescription: Review changes\n---\nReview the diff.")
	writeCompatFile(t, root, ".claude/agents/tester.md", "---\ndescription: Run tests\nmodel: deepseek-v4-flash\ntools: read_file, bash\nmax_steps: 12\n---\nTest the project.")
	writeCompatFile(t, root, ".mcp.json", `{"mcpServers":{"docs":{"command":"npx","args":["-y","docs-server"],"env":{"TOKEN":"${DOCS_TOKEN}"}}}}`)
	writeCompatFile(t, root, ".claude/settings.json", `{"hooks":{"PreToolUse":[{"matcher":"bash","hooks":[{"type":"command","command":"echo hook"}]}]}}`)

	report, err := Scan(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Instructions) != 2 {
		t.Fatalf("instructions=%d, want 2", len(report.Instructions))
	}
	if len(report.Skills) != 1 || report.Skills[0].Name != "review" {
		t.Fatalf("skills=%+v", report.Skills)
	}
	if len(report.Agents) != 1 || report.Agents[0].MaxSteps != 12 || len(report.Agents[0].Tools) != 2 {
		t.Fatalf("agents=%+v", report.Agents)
	}
	if len(report.MCPServers) != 1 || report.MCPServers[0].Name != "docs" || report.MCPServers[0].Transport != "stdio" {
		t.Fatalf("mcp=%+v", report.MCPServers)
	}
	if report.MCPServers[0].Env["TOKEN"] != "${DOCS_TOKEN}" {
		t.Fatal("MCP environment placeholder was changed")
	}
	if len(report.Hooks) != 1 || report.Hooks[0].Event != "PreToolUse" {
		t.Fatalf("hooks=%+v", report.Hooks)
	}
}

func TestScanOpenCodeMCPAndPrecedence(t *testing.T) {
	root := t.TempDir()
	writeCompatFile(t, root, ".mcp.json", `{"mcpServers":{"shared":{"command":"first"}}}`)
	writeCompatFile(t, root, "opencode.json", `{"mcp":{"servers":{"remote":{"type":"remote","url":"https://example.test/mcp"},"shared":{"command":"second"}}}}`)
	report, err := Scan(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.MCPServers) != 2 {
		t.Fatalf("mcp=%+v", report.MCPServers)
	}
	for _, server := range report.MCPServers {
		if server.Name == "shared" && server.Command != "first" {
			t.Fatalf("precedence lost: %+v", server)
		}
		if server.Name == "remote" && (server.Transport != "remote" || !strings.Contains(server.URL, "example.test")) {
			t.Fatalf("remote=%+v", server)
		}
	}
}

func TestScanJSONCAndShadowDiagnostics(t *testing.T) {
	root := t.TempDir()
	writeCompatFile(t, root, "opencode.jsonc", "{\n  // keep URLs inside strings intact\n  \"mcp\": {\"servers\": {\"remote\": {\"url\": \"https://example.test/mcp\"}}}\n}")
	writeCompatFile(t, root, ".opencode/skills/review/SKILL.md", "---\nname: review\n---\nOpenCode review")
	writeCompatFile(t, root, ".claude/skills/review/SKILL.md", "---\nname: review\n---\nClaude review")
	report, err := Scan(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.MCPServers) != 1 || report.MCPServers[0].URL != "https://example.test/mcp" {
		t.Fatalf("jsonc mcp=%+v warnings=%v", report.MCPServers, report.Warnings)
	}
	if len(report.Skills) != 1 || report.Skills[0].Source.Kind != "claude" {
		t.Fatalf("skills=%+v", report.Skills)
	}
	if len(report.Warnings) == 0 || !strings.Contains(report.Warnings[0], "shadowed") {
		t.Fatalf("warnings=%v", report.Warnings)
	}
}
