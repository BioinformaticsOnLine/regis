package tui

import (
	"github.com/charmbracelet/lipgloss"
)

// renderTabIndicator renders the tab selection indicator
func renderTabIndicator(m Model) string {
	activeStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#00FFFF")). // Bright cyan
		Bold(true).
		Underline(true)

	inactiveStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6272A4")) // Muted gray

	var tab1, tab2 string
	if m.activeTab == 1 {
		tab1 = activeStyle.Render("[1] Logs")
		tab2 = inactiveStyle.Render("[2] Summary")
	} else {
		tab1 = inactiveStyle.Render("[1] Logs")
		tab2 = activeStyle.Render("[2] Summary")
	}

	separator := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#44475A")).
		Render("  │  ")

	return tab1 + separator + tab2
}
