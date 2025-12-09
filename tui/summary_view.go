package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// renderSummaryView renders the full summary tab view
func renderSummaryView(m Model) string {
	var content strings.Builder

	// Title
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#BD93F9")). // Purple
		Bold(true).
		Underline(true)

	content.WriteString(titleStyle.Render("📊 PIPELINE SUMMARY"))
	content.WriteString("\n\n")

	// If no steps completed yet
	if len(m.completedSteps) == 0 {
		emptyStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6272A4")). // Muted gray
			Italic(true)
		content.WriteString(emptyStyle.Render("No steps completed yet. Switch to [1] Logs to see real-time progress."))
		content.WriteString("\n")
		return content.String()
	}

	// Get sorted step numbers
	stepNumbers := make([]int, 0, len(m.completedSteps))
	for stepNum := range m.completedSteps {
		stepNumbers = append(stepNumbers, stepNum)
	}
	sort.Ints(stepNumbers)

	// Calculate total duration
	var totalDuration time.Duration
	for _, duration := range m.completedSteps {
		totalDuration += duration
	}

	// Summary header
	summaryHeaderStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#50FA7B")). // Green
		Bold(true)

	content.WriteString(summaryHeaderStyle.Render(fmt.Sprintf("✓ Completed Steps: %d", len(m.completedSteps))))
	content.WriteString("\n")

	totalStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#F1FA8C")) // Yellow
	content.WriteString(totalStyle.Render(fmt.Sprintf("⏱  Total Time: %s", formatDuration(totalDuration))))
	content.WriteString("\n\n")

	// Separator line
	content.WriteString(strings.Repeat("─", 60))
	content.WriteString("\n")

	// Render each completed step
	for _, stepNum := range stepNumbers {
		duration := m.completedSteps[stepNum]
		stepName := m.completedStepNames[stepNum]

		// If no name is stored, use generic name
		if stepName == "" {
			stepName = fmt.Sprintf("Step %d", stepNum)
		}

		// Step number and name
		stepStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFB86C")) // Orange
		stepText := stepStyle.Render(fmt.Sprintf("S%-2d: %s", stepNum, stepName))

		// Duration
		durationStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F1FA8C")) // Yellow
		durationText := durationStyle.Render(formatDuration(duration))

		// Status (always success for now, could be enhanced later)
		statusStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#50FA7B")) // Green
		statusText := statusStyle.Render("✓")

		content.WriteString(fmt.Sprintf("%s  %s  %s\n", stepText, durationText, statusText))
	}

	// Footer hint
	content.WriteString("\n")
	hintStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6272A4")). // Muted gray
		Italic(true)
	content.WriteString(hintStyle.Render("Press [1] to return to Logs view"))

	return content.String()
}
