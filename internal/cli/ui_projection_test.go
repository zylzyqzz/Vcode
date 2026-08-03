package cli

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"vcode/internal/event"
)

func TestActivityLineIsFixedAndCompact(t *testing.T) {
	m := newTestChatTUI()
	m.state = tuiRunning
	plain := strings.TrimSpace(ansi.Strip(m.runningWorkingLine(false, false)))
	if plain != "V....." {
		t.Fatalf("activity line=%q, want V.....", plain)
	}
	if strings.Contains(plain, "V V") || strings.Contains(plain, "0s") {
		t.Fatalf("activity line contains noisy status: %q", plain)
	}
}

func TestNoticeProjectionHidesInternalLifecycleNoise(t *testing.T) {
	for _, text := range []string{"background bash started: bash-1", "task build · node_started", "node_finished", "session changed"} {
		if shouldShowNotice(text, event.LevelInfo) {
			t.Fatalf("internal notice should be hidden: %q", text)
		}
	}
	if !shouldShowNotice("permission required", event.LevelWarn) {
		t.Fatal("permission warning must remain visible")
	}
	if !shouldShowNotice("verification failed: go test ./...", event.LevelInfo) {
		t.Fatal("verification failure must remain visible")
	}
}

func TestModeTagUsesGoldRoundedFrameWithoutBackground(t *testing.T) {
	plain := ansi.Strip(renderModeTag("Build"))
	if plain != "⟮─ Build ─⟯" {
		t.Fatalf("mode tag=%q, want gold capsule frame", plain)
	}
	if strings.Contains(renderModeTag("Plan"), "48;") {
		t.Fatal("mode tag must not use a background color")
	}
}
