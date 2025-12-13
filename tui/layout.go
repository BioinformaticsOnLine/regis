package tui

import (
	"fmt"
	"math"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/BioinformaticsOnLine/regis/version"
)

// Color palette
var (
	colorPrimary   = lipgloss.Color("#00D9FF")
	colorSecondary = lipgloss.Color("#BD93F9")
	colorSuccess   = lipgloss.Color("#50FA7B")
	colorError     = lipgloss.Color("#FF5555")
	colorWarning   = lipgloss.Color("#FFB86C")
	colorMuted     = lipgloss.Color("#6272A4")
	colorBorder    = lipgloss.Color("#44475A")
)

// Styles
var (
	logoStyle = lipgloss.NewStyle().
			Foreground(colorPrimary).
			Bold(true)

	subtitleStyle = lipgloss.NewStyle().
			Foreground(colorMuted).
			Italic(true)

	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorBorder).
			Padding(1, 2)

	stepBannerStyle = lipgloss.NewStyle().
			Foreground(colorPrimary).
			Bold(true).
			Padding(0, 2).
			Border(lipgloss.DoubleBorder(), false, false, true, false).
			BorderForeground(colorBorder)

	panelTitleStyle = lipgloss.NewStyle().
			Foreground(colorSecondary).
			Bold(true)

	commandStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F8F8F2"))

	logStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F8F8F2"))

	helpStyle = lipgloss.NewStyle().
			Foreground(colorMuted).
			Italic(true)
)

// renderMainView renders the main TUI view layout
func renderMainView(m Model) string {
	var sections []string

	// Top header: Logo on left, USER COMMAND TAB on right
	logo := renderLogo(m)
	commandTab := renderCommandTab(m)

	// Calculate adaptive spacer based on window width
	// Logo ASCII art is actually 45 chars wide (including the leading spaces)
	// Command tab box is roughly 35-40 chars
	logoWidth := 45       // Corrected from 41 to match actual ASCII art width
	commandTabWidth := 40 // Approximate with box borders
	usedWidth := logoWidth + commandTabWidth

	// Calculate spacer to fill remaining space (but keep a minimum of 2)
	spacerWidth := 2
	if m.width > usedWidth+10 {
		// We have extra space, distribute it
		spacerWidth = m.width - usedWidth - 8 // -8 for margins and safety
		// But cap it at a reasonable maximum
		if spacerWidth > 20 {
			spacerWidth = 20
		}
	}

	spacer := strings.Repeat(" ", spacerWidth)

	// Join horizontally with adaptive gap
	topHeader := lipgloss.JoinHorizontal(
		lipgloss.Top,
		logo,
		spacer,
		commandTab,
	)
	sections = append(sections, topHeader)

	// Step banner with inline tab indicator
	stepBannerText := renderStepBanner(m)
	tabIndicatorText := renderTabIndicator(m)

	// Join them horizontally with spacing
	spacer = strings.Repeat(" ", 5)
	combinedHeader := lipgloss.JoinHorizontal(lipgloss.Left, stepBannerText, spacer, tabIndicatorText)
	sections = append(sections, combinedHeader)

	// Divider removed to save vertical space for log viewport
	// dividerWidth := m.width - 4
	// if dividerWidth < 1 {
	// 	dividerWidth = 80
	// }
	// sections = append(sections, strings.Repeat("─", dividerWidth))

	// Content based on active tab
	var contentSection string
	if m.activeTab == 1 {
		// Tab 1: Logs view
		if m.currentCommand != "" {
			contentSection = renderRunningToolHeading(m) + renderLogPanel(m)
		} else {
			contentSection = renderLogPanel(m)
		}
	} else if m.activeTab == 2 {
		// Tab 2: Summary view
		contentSection = renderSummaryView(m)
	}

	sections = append(sections, contentSection)

	// System metrics at bottom left
	sections = append(sections, renderSystemMetrics(m))

	// Manually join sections to eliminate gaps from JoinVertical
	// JoinVertical adds automatic spacing, so we concatenate directly
	var result strings.Builder
	for i, section := range sections {
		result.WriteString(section)
		// Only add newline between sections, not after the last one
		if i < len(sections)-1 {
			result.WriteString("\n")
		}
	}

	// Add minimal top padding to prevent logo cutoff while maximizing space
	return "\n" + result.String()
}

// renderLogo renders the REGIS ASCII logo
func renderLogo(m Model) string {
	logo := `██████╗ ███████╗ ██████╗ ██╗███████╗
██╔══██╗██╔════╝██╔════╝ ██║██╔════╝
██████╔╝█████╗  ██║  ███╗██║███████╗
██╔══██╗██╔══╝  ██║   ██║██║╚════██║
██║  ██║███████╗╚██████╔╝██║███████║
╚═╝  ╚═╝╚══════╝ ╚═════╝ ╚═╝╚══════╝`

	// Static cyan color - clean and professional
	staticLogo := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#00D9FF")).
		Bold(true).
		Render(logo)

	// Gold/yellow subtitle
	subtitle1 := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFD700")).
		Italic(true).
		Render("RNA-seq Guided Identification System")

	subtitle2 := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFD700")).
		Italic(true).
		Render("lncRNA Discovery Pipeline v" + version.Version)

	// Contact and attribution info - compact
	bugs := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6272A4")).
		Render("Dev and Bugs: github.com/pranjalpruthi")

	organization := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6272A4")).
		Render("Regis Team • CNRS-Sorbonne • CSIR-IGIB")

	return staticLogo + "\n" + subtitle1 + "\n" + subtitle2 + "\n" + bugs + "\n" + organization
}

// renderCommandTab renders the USER COMMAND TAB with full details
func renderCommandTab(m Model) string {
	var content strings.Builder

	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FF79C6")). // Bright pink
		Bold(true).
		Underline(true)

	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#8BE9FD")) // Bright cyan

	valueStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#50FA7B")). // Bright green
		Bold(true)

	highlightStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFB86C")). // Bright orange
		Bold(true)

	content.WriteString(titleStyle.Render("⚙ USER COMMAND TAB"))
	content.WriteString("\n")

	// Time and cores
	content.WriteString(labelStyle.Render("🕐 Started: "))
	content.WriteString(valueStyle.Render(m.metadata.StartTime.Format("15:04:05")))
	content.WriteString("\n")
	content.WriteString(labelStyle.Render("⚡ CPU cores: "))
	content.WriteString(valueStyle.Render(fmt.Sprintf("%d", m.metadata.Cores)))
	content.WriteString("\n")

	// Data type and method
	content.WriteString(labelStyle.Render("📊 Data type: "))
	content.WriteString(highlightStyle.Render(m.metadata.DataType))
	content.WriteString("\n")
	content.WriteString(labelStyle.Render("🔬 Method: "))
	content.WriteString(highlightStyle.Render(m.metadata.Method))
	content.WriteString("\n")

	// rRNA filtering status
	if m.metadata.SortMeRNAEnabled {
		content.WriteString(labelStyle.Render("🧹 rRNA filter: "))
		content.WriteString(valueStyle.Render("enabled"))
		content.WriteString("\n")
	}

	// Validation mode with details
	if m.metadata.ValidationMode != "" {
		content.WriteString(labelStyle.Render("✓ Validation: "))
		content.WriteString(highlightStyle.Render(m.metadata.ValidationMode))
		content.WriteString("\n")

		// CPAT custom models if provided
		if m.metadata.CPATHexModel != "" {
			content.WriteString(labelStyle.Render("  • Hex: "))
			content.WriteString(valueStyle.Render(m.metadata.CPATHexModel))
			content.WriteString("\n")
		}
		if m.metadata.CPATLogitModel != "" {
			content.WriteString(labelStyle.Render("  • Logit: "))
			content.WriteString(valueStyle.Render(m.metadata.CPATLogitModel))
			content.WriteString("\n")
		}
	}

	// LncTar mode with details
	if m.metadata.LncTarMode != "off" && m.metadata.LncTarMode != "" {
		content.WriteString(labelStyle.Render("🎯 LncTar: "))
		content.WriteString(valueStyle.Render(m.metadata.LncTarMode))
		content.WriteString("\n")
	}

	// IntaRNA mode with details
	if m.metadata.IntaRNAMode != "off" && m.metadata.IntaRNAMode != "" {
		content.WriteString(labelStyle.Render("🧬 IntaRNA: "))
		content.WriteString(valueStyle.Render(m.metadata.IntaRNAMode))
		content.WriteString("\n")
	}

	hintStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#BD93F9")). // Purple
		Italic(true)

	content.WriteString(hintStyle.Render("💡 'e' for full command"))

	// Return content without box border/padding for compact layout
	return content.String()
}

// renderStepBanner renders the current step banner with smooth color transitions and pulsing effect
func renderStepBanner(m Model) string {
	if m.currentStepName == "" {
		return ""
	}

	// Gradient colors for smooth transitions
	colors := []string{"#00D9FF", "#00FFFF", "#00FF80", "#50FA7B", "#FFB86C", "#FF79C6"}

	// Get current and next color for smooth transition
	// Use currentStep directly to handle Step 0 (Dependency Check) correctly
	colorIndex := m.currentStep % len(colors)
	nextColorIndex := (colorIndex + 1) % len(colors)

	currentColor := colors[colorIndex]
	nextColor := colors[nextColorIndex]

	// Smooth breathing/pulsing effect using sine wave
	pulse := math.Sin(m.animationTime)*0.5 + 0.5 // Oscillates between 0 and 1

	// Use harmonica spring physics for smooth color transition
	// Spring position oscillates smoothly with natural physics
	transitionProgress := (m.springPosition + 1.0) / 2.0 // Normalize to 0-1
	displayColor := interpolateColor(currentColor, nextColor, transitionProgress)

	bannerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(displayColor)).
		Bold(true).
		Padding(0, 2).
		Border(lipgloss.DoubleBorder(), false, false, true, false).
		BorderForeground(lipgloss.Color(displayColor))

	// Add subtle glow effect with pulse
	glowChar := "●"
	if pulse > 0.7 {
		glowChar = "◉"
	} else if pulse > 0.3 {
		glowChar = "◎"
	}

	// Combine glow and step info (no emoji here)
	banner := bannerStyle.Render(fmt.Sprintf("%s Step %d: %s", glowChar, m.currentStep, m.currentStepName))

	// Add progress bar if step is in progress
	var progressSection string
	if m.stepProgress > 0 && m.stepProgress < 1.0 {
		progressSection = "\n" + m.progressBar.ViewAs(m.stepProgress)
	}

	// Add time elapsed
	var timeInfo string
	if !m.stepStartTime.IsZero() {
		elapsed := time.Since(m.stepStartTime)
		timeStr := formatDuration(elapsed)
		timeStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#BD93F9")).
			Italic(true)
		timeInfo = " " + timeStyle.Render(fmt.Sprintf("(Step Time: %s)", timeStr))
	}

	return banner + timeInfo + progressSection
}

// interpolateColor smoothly interpolates between two hex colors
func interpolateColor(color1, color2 string, t float64) string {
	// Parse hex colors
	r1, g1, b1 := parseHexColor(color1)
	r2, g2, b2 := parseHexColor(color2)

	// Interpolate
	r := int(float64(r1)*(1-t) + float64(r2)*t)
	g := int(float64(g1)*(1-t) + float64(g2)*t)
	b := int(float64(b1)*(1-t) + float64(b2)*t)

	return fmt.Sprintf("#%02X%02X%02X", r, g, b)
}

// parseHexColor parses a hex color string to RGB values
func parseHexColor(hex string) (int, int, int) {
	hex = strings.TrimPrefix(hex, "#")
	var r, g, b int
	fmt.Sscanf(hex, "%02x%02x%02x", &r, &g, &b)
	return r, g, b
}

// renderSystemMetrics renders CPU and memory usage with colors
func renderSystemMetrics(m Model) string {
	// Color thresholds
	cpuColor := "#50FA7B" // Green
	if m.cpuPercent > 70 {
		cpuColor = "#FFB86C" // Orange
	}
	if m.cpuPercent > 90 {
		cpuColor = "#FF5555" // Red
	}

	memColor := "#50FA7B" // Green
	if m.memPercent > 70 {
		memColor = "#FFB86C" // Orange
	}
	if m.memPercent > 90 {
		memColor = "#FF5555" // Red
	}

	cpuText := lipgloss.NewStyle().
		Foreground(lipgloss.Color(cpuColor)).
		Bold(true).
		Render(fmt.Sprintf("CPU: %.1f%%", m.cpuPercent))

	memText := lipgloss.NewStyle().
		Foreground(lipgloss.Color(memColor)).
		Bold(true).
		Render(fmt.Sprintf("MEM: %.1f%%", m.memPercent))

	separator := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6272A4")).
		Render("  |  ")

	// Add elapsed time (total pipeline time)
	timeText := ""
	if !m.metadata.StartTime.IsZero() {
		elapsed := time.Since(m.metadata.StartTime)
		timeStr := formatDuration(elapsed)
		timeText = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#BD93F9")).
			Italic(true).
			Render(fmt.Sprintf("  |  Total Time: %s", timeStr))
	}

	// Add hint for fullscreen logs
	hintText := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6272A4")). // Muted gray
		Italic(true).
		Render("  |  Press 'l' for fullscreen logs")

	metricsStyle := lipgloss.NewStyle().PaddingLeft(2)

	return metricsStyle.Render(cpuText + separator + memText + timeText + hintText)
}

// renderRunningToolHeading renders a compact heading showing the current running tool with animated emoji
func renderRunningToolHeading(m Model) string {
	parts := strings.Fields(m.currentCommand)
	toolName := "unknown"
	if len(parts) > 0 {
		toolName = parts[0]
		if idx := strings.LastIndex(toolName, "/"); idx >= 0 {
			toolName = toolName[idx+1:]
		}
	}

	spinnerText := ""
	if m.spinnerRunning {
		spinnerText = m.spinner.View() + " "
	}

	// Animated motivational emoji frames (13 frames for smooth animation)
	emojiFrames := []string{
		"(｡◕‿◕｡)",  // Frame 1: Happy
		"(｡◕‿‿◕｡)", // Frame 2: Very happy
		"(◕‿◕)",    // Frame 3: Cheerful
		"(◕ω◕)",    // Frame 4: Content
		"(´･_･`)",  // Frame 5: Worried/working hard
		"(◕‿◕✿)",   // Frame 6: Flower
		"(◕‿◕)っ",   // Frame 7: Reaching
		"(◕‿◕)ノ",   // Frame 8: Waving
		"ヽ(◕‿◕)ノ",  // Frame 9: Celebrating
		"(◕ᴗ◕✿)",   // Frame 10: Blooming
		"(◕‿◕)♡",   // Frame 11: Love
		"(◕‿◕)☆",   // Frame 12: Star
		"(◕‿◕)✧",   // Frame 13: Sparkle
	}

	// Cycle through emoji frames based on animation time (13 frames)
	frameIndex := int(m.animationTime*3) % len(emojiFrames) // Change every ~0.33 seconds for smoother animation
	currentEmoji := emojiFrames[frameIndex]

	// Colorful gradient for tool status
	toolStatus := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#00FFFF")). // Bright cyan
		Bold(true).
		Render(spinnerText+currentEmoji+" ▶ Running: ") +
		lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFB86C")). // Orange for tool name
			Bold(true).
			Render(toolName)

	// Responsive hint - shorten on small screens
	hintText := "[Press 'c' for full sub-tool command | 't' to terminate]"
	if m.width < 100 {
		hintText = "['c' command | 't' quit]"
	}

	hint := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6272A4")).
		Italic(true).
		Render("  " + hintText)

	// Only leading newline, no trailing newline to reduce gaps
	return "\n" + toolStatus + hint
}

// renderCommandPanel renders the sub tool command panel
func renderCommandPanel(m Model) string {
	// Determine dynamic title based on command
	titleText := "Sub Tool Command"
	if m.currentCommand != "" {
		parts := strings.Fields(m.currentCommand)
		if len(parts) > 0 {
			toolName := filepath.Base(parts[0])
			titleText = fmt.Sprintf("Running: %s", toolName)
		}
	}
	title := panelTitleStyle.Render(titleText)

	etaText := ""
	if m.stepETA > 0 {
		etaText = fmt.Sprintf("ETA: %s", formatDuration(m.stepETA))
	}

	// Calculate spacing, ensure it's not negative
	spacing := m.width - lipgloss.Width(title) - lipgloss.Width(etaText) - 10
	if spacing < 0 {
		spacing = 0
	}

	header := lipgloss.JoinHorizontal(
		lipgloss.Top,
		title,
		strings.Repeat(" ", spacing),
		etaText,
	)

	// Show current command
	commandText := m.currentCommand
	if commandText == "" {
		commandText = "Waiting for command..."
	}

	// Build scroll indicator
	scrollIndicator := ""
	if m.commandViewport.TotalLineCount() > m.commandViewport.Height {
		// Content is scrollable
		scrollPct := int(m.commandViewport.ScrollPercent() * 100)

		upArrow := "▲"
		downArrow := "▼"

		if m.commandViewport.AtTop() {
			upArrow = " "
		}
		if m.commandViewport.AtBottom() {
			downArrow = " "
		}

		scrollIndicator = fmt.Sprintf("\n%s Scroll: %d%% %s (use ↑↓ or Tab+arrows)",
			upArrow, scrollPct, downArrow)
	}

	content := commandStyle.Render(commandText) + "\n\n" + m.commandViewport.View() + scrollIndicator

	border := lipgloss.RoundedBorder()
	if m.focusedPanel == 0 {
		border = lipgloss.ThickBorder()
	}

	panel := lipgloss.NewStyle().
		Border(border).
		BorderForeground(colorBorder).
		Padding(1, 2).
		Width(m.width - 4).
		Render(header + "\n" + content)

	return panel
}

// renderLogPanel renders the log viewer panel
func renderLogPanel(m Model) string {
	title := panelTitleStyle.Render("Regis Log")

	content := m.logViewport.View()
	if len(m.logs) == 0 {
		content = logStyle.Render("Waiting for logs...")
	}

	// Build scroll indicator
	scrollIndicator := ""
	if m.logViewport.TotalLineCount() > m.logViewport.Height {
		// Content is scrollable
		scrollPct := int(m.logViewport.ScrollPercent() * 100)

		upArrow := "▲"
		downArrow := "▼"

		if m.logViewport.AtTop() {
			upArrow = " "
		}
		if m.logViewport.AtBottom() {
			downArrow = " "
		}

		scrollIndicator = fmt.Sprintf("\n%s Scroll: %d%% %s (use ↑↓ arrows or mouse wheel)",
			upArrow, scrollPct, downArrow)
	}

	border := lipgloss.RoundedBorder()
	if m.focusedPanel == 1 {
		border = lipgloss.ThickBorder()
	}

	// Don't set a fixed height - let the panel size naturally based on content
	// This prevents issues when resizing the terminal
	panel := lipgloss.NewStyle().
		Border(border).
		BorderForeground(colorBorder).
		Padding(1, 2).
		Width(m.width - 4).
		Render(title + "\n" + content + scrollIndicator)

	return panel
}

// renderHelp renders the help screen
func renderHelp(width, height int) string {
	helpText := `
REGIS TUI - Keyboard Shortcuts

Navigation:
  ↑/↓         Scroll current panel
  PgUp/PgDn   Page scroll
  Home/End    Jump to top/bottom
  Tab         Switch between command and log panels

Actions:
  c           Toggle USER COMMAND TAB (expand/collapse)
  l           View logs in fullscreen mode
  ?           Toggle this help screen
  q / Ctrl+C  Quit (with confirmation if pipeline running)

Panels:
  • Command Panel: Shows currently executing command
  • Log Panel: Shows real-time JSON-formatted logs

Press ? to close this help screen
`

	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorPrimary).
		Padding(1, 2).
		Width(width-4).
		Height(height-4).
		Align(lipgloss.Center, lipgloss.Center)

	return style.Render(helpText)
}

// formatDuration formats a duration in human-readable format
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}

	minutes := int(d.Minutes())
	seconds := int(d.Seconds()) % 60

	if minutes < 60 {
		return fmt.Sprintf("%dm %ds", minutes, seconds)
	}

	hours := minutes / 60
	minutes = minutes % 60

	return fmt.Sprintf("%dh %dm", hours, minutes)
}
