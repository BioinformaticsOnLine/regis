package tui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

// renderFullscreenLogs renders the logs in fullscreen mode
func renderFullscreenLogs(m Model) string {
	// Build title header
	title := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFB86C")).
		Bold(true).
		Render("📜 REGIS LOGS (Fullscreen)")

	instructions := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6272A4")).
		Italic(true).
		Render("  [Press 'l' or ESC to exit | ↑↓ to scroll | q to quit]")

	header := title + instructions

	// Calculate available height for viewport (full screen minus header and scroll indicator)
	availableHeight := m.height - 4 // Reserve space for header (2 lines) and scroll indicator (2 lines)
	if availableHeight < 10 {
		availableHeight = 10
	}

	// Create a temporary fullscreen viewport with the logs
	fullscreenViewport := m.logViewport
	fullscreenViewport.Height = availableHeight
	fullscreenViewport.Width = m.width - 4

	// Get the content from the resized viewport
	content := fullscreenViewport.View()
	if len(m.logs) == 0 {
		content = "Waiting for logs..."
	}

	// Build scroll indicator
	scrollIndicator := ""
	if m.logViewport.TotalLineCount() > availableHeight {
		scrollPct := int(m.logViewport.ScrollPercent() * 100)

		upArrow := "▲"
		downArrow := "▼"

		if m.logViewport.AtTop() {
			upArrow = " "
		}
		if m.logViewport.AtBottom() {
			downArrow = " "
		}

		scrollIndicator = fmt.Sprintf("\n%s Scroll: %d%% %s", upArrow, scrollPct, downArrow)
	}

	// Render everything together
	result := header + "\n\n" + content + scrollIndicator

	return result
}
