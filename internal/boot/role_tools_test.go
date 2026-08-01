package boot

import (
	"context"
	"encoding/json"
	"testing"

	"vcode/internal/tool"
)

type roleTestTool struct{ name string }

func (t roleTestTool) Name() string                                             { return t.name }
func (t roleTestTool) Description() string                                      { return t.name }
func (t roleTestTool) Schema() json.RawMessage                                  { return json.RawMessage(`{"type":"object"}`) }
func (t roleTestTool) Execute(context.Context, json.RawMessage) (string, error) { return "", nil }
func (t roleTestTool) ReadOnly() bool                                           { return true }

func TestFilterRegistryHonorsRoleAllowlist(t *testing.T) {
	reg := tool.NewRegistry()
	reg.Add(roleTestTool{name: "read_file"})
	reg.Add(roleTestTool{name: "bash"})
	filtered := filterRegistry(reg, []string{"read_file", "missing"})
	if filtered.Len() != 1 {
		t.Fatalf("filtered tools=%v", filtered.Names())
	}
	if _, ok := filtered.Get("read_file"); !ok {
		t.Fatal("allowlisted tool missing")
	}
	if _, ok := filtered.Get("bash"); ok {
		t.Fatal("non-allowlisted tool leaked")
	}
}
