package agent

import (
	"context"
	"encoding/json"
	"strings"
	"sync/atomic"
	"testing"

	"vcode/internal/event"
	"vcode/internal/provider"
	"vcode/internal/tool"
)

type changingReadTool struct {
	calls int32
}

func (t *changingReadTool) Name() string            { return "read_file" }
func (t *changingReadTool) Description() string     { return "returns changing content" }
func (t *changingReadTool) Schema() json.RawMessage { return json.RawMessage(`{"type":"object"}`) }
func (t *changingReadTool) ReadOnly() bool          { return true }
func (t *changingReadTool) Execute(context.Context, json.RawMessage) (string, error) {
	return "content revision " + string(rune('1'+atomic.AddInt32(&t.calls, 1))), nil
}

func TestReadOnlyNoProgressGuardNudgesIdenticalSuccessfulReads(t *testing.T) {
	var calls int32
	reg := tool.NewRegistry()
	reg.Add(fakeTool{name: "read_file", readOnly: true, calls: &calls})
	args := `{"path":"status.txt"}`
	turns := make([][]provider.Chunk, 0, readOnlyNoProgressBreakThreshold+1)
	for i := 0; i < readOnlyNoProgressBreakThreshold; i++ {
		turns = append(turns, []provider.Chunk{toolCallChunk("c", "read_file", args), {Type: provider.ChunkDone}})
	}
	turns = append(turns, []provider.Chunk{{Type: provider.ChunkText, Text: "done"}, {Type: provider.ChunkDone}})
	prov := &scriptedProvider{name: "p", turns: turns}
	a := New(prov, reg, NewSession(""), Options{}, event.Discard)

	if err := a.Run(context.Background(), "keep checking"); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := atomic.LoadInt32(&calls); got != readOnlyNoProgressBreakThreshold {
		t.Fatalf("read calls = %d, want %d", got, readOnlyNoProgressBreakThreshold)
	}
	if last := lastToolResult(a.session, "read_file"); !strings.Contains(last, "[loop guard]") || !strings.Contains(last, "same result") {
		t.Fatalf("last identical read should carry a progress nudge, got %q", last)
	}
}

func TestReadOnlyNoProgressGuardAllowsChangedResults(t *testing.T) {
	reg := tool.NewRegistry()
	reader := &changingReadTool{}
	reg.Add(reader)
	args := `{"path":"status.txt"}`
	turns := make([][]provider.Chunk, 0, readOnlyNoProgressBreakThreshold+1)
	for i := 0; i < readOnlyNoProgressBreakThreshold+1; i++ {
		turns = append(turns, []provider.Chunk{toolCallChunk("c", "read_file", args), {Type: provider.ChunkDone}})
	}
	turns = append(turns, []provider.Chunk{{Type: provider.ChunkText, Text: "done"}, {Type: provider.ChunkDone}})
	prov := &scriptedProvider{name: "p", turns: turns}
	a := New(prov, reg, NewSession(""), Options{}, event.Discard)

	if err := a.Run(context.Background(), "watch for progress"); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if last := lastToolResult(a.session, "read_file"); strings.Contains(last, "[loop guard]") {
		t.Fatalf("changed read results must not trip the no-progress guard, got %q", last)
	}
}
