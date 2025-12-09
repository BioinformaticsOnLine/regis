package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// renderCommandPopup renders a popup dialog showing the full command
func renderCommandPopup(m Model, baseView string) string {
	// Create popup style
	popupStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#BD93F9")).
		Padding(1, 2).
		Background(lipgloss.Color("#282A36")).
		Foreground(lipgloss.Color("#F8F8F2"))

	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#BD93F9")).
		Bold(true)

	// Wrap command at 80 chars for readability
	wrappedCmd := wrapText(m.currentCommand, 80)

	// Build popup content
	var content strings.Builder
	content.WriteString(titleStyle.Render("Current Sub-Tool Command"))
	content.WriteString("\n\n")
	content.WriteString(wrappedCmd)
	content.WriteString("\n\n")
	content.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6272A4")).
		Italic(true).
		Render("Press 'c' or 'Esc' to close"))

	popup := popupStyle.Render(content.String())

	// Place popup in center of screen using lipgloss.Place
	return lipgloss.Place(
		m.width,
		m.height,
		lipgloss.Center,
		lipgloss.Center,
		popup,
		lipgloss.WithWhitespaceChars(" "),
		lipgloss.WithWhitespaceForeground(lipgloss.Color("#44475A")),
	)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
