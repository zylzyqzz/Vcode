package cli

import (
	"fmt"
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
	if plain != "⟮─ Build  ─⟯" {
		t.Fatalf("mode tag=%q, want gold capsule frame", plain)
	}
	if ansi.StringWidth(ansi.Strip(renderModeTag("Build"))) != ansi.StringWidth(ansi.Strip(renderModeTag("Plan"))) || ansi.StringWidth(ansi.Strip(renderModeTag("Plan"))) != ansi.StringWidth(ansi.Strip(renderModeTag("Goal"))) {
		t.Fatal("mode frames must keep a stable width while switching")
	}
	if strings.Contains(renderModeTag("Plan"), "48;") {
		t.Fatal("mode tag must not use a background color")
	}
}

func TestMainCLIChromeUsesStableBrandGold(t *testing.T) {
	if vcodeBrandGold.hex != "#b8860b" {
		t.Fatalf("brand gold=%q, want stable dark gold", vcodeBrandGold.hex)
	}
	if got := (chatTUI{goalMode: true}).statusModeColor(); got != vcodeBrandGold {
		t.Fatalf("goal chrome color=%+v, want brand gold %+v", got, vcodeBrandGold)
	}
	if got := (chatTUI{planMode: true}).statusModeColor(); got != vcodeBrandGold {
		t.Fatalf("plan chrome color=%+v, want brand gold %+v", got, vcodeBrandGold)
	}
}

func TestMarkdownAccentUsesBrandGold(t *testing.T) {
	if got := ansi.Strip(brandAccent("标题")); got != "标题" {
		t.Fatalf("brand markdown accent changed content: %q", got)
	}
	if activeCLITheme.accent == vcodeBrandGold {
		t.Skip("active theme already uses brand gold")
	}
	if strings.Contains(brandAccent("标题"), fmt.Sprintf("38;5;%d", activeCLITheme.accent.xterm)) {
		t.Fatal("markdown should not inherit the selectable theme accent")
	}
}

func TestSubmittedMessageUsesBrandGold(t *testing.T) {
	if got := renderUserBubble("你好", 80, false); !strings.Contains(got, "你好") {
		t.Fatalf("submitted message lost content: %q", got)
	}
	if strings.Contains(renderUserBubble("你好", 80, false), fmt.Sprintf("38;5;%d", activeCLITheme.accent.xterm)) && activeCLITheme.accent != vcodeBrandGold {
		t.Fatal("submitted message should not inherit the selectable theme accent")
	}
}
