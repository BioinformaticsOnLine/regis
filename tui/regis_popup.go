package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// renderRegisCommandPopup renders a popup showing the entire regis command invocation
func renderRegisCommandPopup(m Model, baseView string) string {
	// Create popup style
	popupStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#50FA7B")). // Green for regis command
		Padding(1, 2).
		Background(lipgloss.Color("#282A36")).
		Foreground(lipgloss.Color("#F8F8F2"))

	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#50FA7B")).
		Bold(true)

	// Get original command from metadata
	fullCommand := m.metadata.OriginalCommand
	if fullCommand == "" {
		fullCommand = "./regis (command not available)"
	}

	// Wrap for display
	wrappedCmd := wrapText(fullCommand, 80)

	// Build popup content
	var content strings.Builder
	content.WriteString(titleStyle.Render("Original REGIS Command"))
	content.WriteString("\n\n")
	content.WriteString(wrappedCmd)
	content.WriteString("\n\n")
	content.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6272A4")).
		Italic(true).
		Render("Press 'e' or 'Esc' to close"))

	popup := popupStyle.Render(content.String())

	// Place popup in center
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
