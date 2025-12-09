package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// renderStepSummary renders a compact summary of completed steps with their durations
func renderStepSummary(m Model) string {
	if len(m.completedSteps) == 0 {
		return ""
	}

	var summary strings.Builder

	// Title
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#BD93F9")). // Purple
		Bold(true)

	summary.WriteString(titleStyle.Render("⏱  STEP TIMES"))
	summary.WriteString("\n")

	// Get sorted step numbers
	stepNumbers := make([]int, 0, len(m.completedSteps))
	for stepNum := range m.completedSteps {
		stepNumbers = append(stepNumbers, stepNum)
	}
	sort.Ints(stepNumbers)

	// Render each completed step
	for _, stepNum := range stepNumbers {
		duration := m.completedSteps[stepNum]

		// Format: S1: 1m20s
		stepLabel := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#50FA7B")). // Green
			Render(fmt.Sprintf("✓ S%-2d:", stepNum))

		durationText := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F1FA8C")). // Yellow
			Render(formatDuration(duration))

		summary.WriteString(fmt.Sprintf("%s %s\n", stepLabel, durationText))
	}

	return summary.String()
}
