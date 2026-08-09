package cli

import (
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

const shimmerText = "  Vcode…"

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
	runes := []rune(shimmerText)
	n := len(runes)
	if n == 0 {
		return ""
	}
	head := frame % (n + 4)
	radius := 3

	var b strings.Builder
	for i, r := range runes {
		dist := head - i
		if dist >= 0 && dist <= radius {
			b.WriteString(shimmerBright.Render(string(r)))
		} else {
			b.WriteString(shimmerDark.Render(string(r)))
		}
	}
	return b.String()
}
