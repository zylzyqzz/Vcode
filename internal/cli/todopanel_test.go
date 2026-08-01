package cli

import "testing"

// Todo details are intentionally hidden from the transcript in the CLI-first
// UI; progress is represented by the unified activity line.
func TestRenderTodoPanelIsHidden(t *testing.T) {
	m := newTestChatTUI()
	m.todoArgs = `{"todos":[{"content":"phase","status":"in_progress"}]}`
	if got := m.renderTodoPanel(); got != "" {
		t.Fatalf("todo panel should be hidden, got %q", got)
	}
}
