package cli

import (
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

const shimmerText = "  Vcode....."

const activityText = "  V....."

var (
	shimmerDark   = lipgloss.NewStyle().Foreground(lipgloss.Color("#b8860b"))
	shimmerBright = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFD700"))
)

type shimmerTickMsg struct{}

func shimmerTickCmd() tea.Cmd {
	return tea.Tick(120*time.Millisecond, func(_ time.Time) tea.Msg { return shimmerTickMsg{} })
}

// renderShimmer applies a left-to-right golden sweep highlight on the text.
func renderShimmer(frame int) string {
	return renderShimmerText(shimmerText, frame)
}

func renderActivity(frame int) string {
	return renderShimmerText(activityText, frame)
}

func renderShimmerText(text string, frame int) string {
	runes := []rune(text)
	n := len(runes)
	if n == 0 {
		return ""
	}
	head := frame % (n + 4)
	radius := 3

	// Keep the text fixed and render contiguous spans. Styling each rune
	// separately produces visible spacing artifacts in Windows Terminal.
	brightStart := head - radius
	brightEnd := head + 1
	if brightStart < 0 {
		brightStart = 0
	}
	if brightEnd > n {
		brightEnd = n
	}
	var b strings.Builder
	if brightStart > 0 {
		b.WriteString(shimmerDark.Render(string(runes[:brightStart])))
	}
	if brightStart < brightEnd {
		b.WriteString(shimmerBright.Render(string(runes[brightStart:brightEnd])))
	}
	if brightEnd < n {
		b.WriteString(shimmerDark.Render(string(runes[brightEnd:])))
	}
	return b.String()
}
