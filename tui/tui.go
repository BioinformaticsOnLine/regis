package tui

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
)

// Message types for pipeline events
type StepStartMsg struct {
	StepNumber  int
	StepName    string
	Command     string
	Description string
}

type StepProgressMsg struct {
	Progress    float64
	StepETA     time.Duration
	PipelineETA time.Duration
}

type StepCompleteMsg struct {
	StepNumber int
	Success    bool
	Duration   time.Duration
}

type LogEntryMsg struct {
	Level     string
	Timestamp time.Time
	Message   string
	Caller    string
}

type CommandOutputMsg struct {
	Output string
}

type UserCommandTabToggleMsg struct{}

// PipelineMetadataMsg contains metadata about the pipeline configuration
type PipelineMetadataMsg struct {
	StartTime        time.Time
	DataType         string
	Method           string
	Cores            int
	Species          string
	ValidationMode   string // e.g., "CPAT+CPC2", "CPC2-only"
	LncTarMode       string // e.g., "off", "best", "highly", "all"
	IntaRNAMode      string // e.g., "off", "best", "highly", "all"
	SortMeRNAEnabled bool   // rRNA filtering enabled
	OriginalCommand  string // Full command line invocation

	// CPAT custom models
	CPATHexModel   string
	CPATLogitModel string
}

type PipelineCompleteMsg struct {
	Success       bool
	Duration      time.Duration
	FailureReason string
}

type SystemMetricsMsg struct {
	CPUPercent    float64
	MemoryPercent float64
	MemoryUsedGB  float64
	MemoryTotalGB float64
}

type TickMsg time.Time

// QuitMsg signals the TUI to exit
type QuitMsg struct{}

// Model is the main TUI state
type Model struct {
	// Pipeline metadata
	metadata PipelineMetadataMsg

	// Current step info
	currentStep     int
	currentStepName string
	currentCommand  string
	stepETA         time.Duration
	pipelineETA     time.Duration
	stepStartTime   time.Time

	// Progress
	stepProgress   float64
	progressBar    progress.Model
	spinner        spinner.Model
	spinnerRunning bool

	// System metrics
	cpuPercent float64
	memPercent float64
	memUsedGB  float64
	memTotalGB float64

	// Logs and command output
	logs          []LogEntryMsg
	commandOutput []string

	// Viewports for scrolling
	commandViewport viewport.Model
	logViewport     viewport.Model

	// UI state
	commandTabExpanded bool
	focusedPanel       int // 0 = command, 1 = log
	activeTab          int // 1 = Logs, 2 = Summary
	showHelp           bool
	showCommandPopup   bool    // Show full sub-tool command in popup
	showRegisCommand   bool    // Show full regis command (original invocation)
	showFullscreenLogs bool    // Show logs in fullscreen mode
	logoColorIndex     int     // For animated logo
	animationTime      float64 // For smooth animations

	// Harmonica spring physics for smooth transitions
	springPosition float64 // Current spring position
	springVelocity float64 // Current spring velocity

	// Pipeline state
	PipelineRunning bool
	PipelineSuccess bool
	FailureReason   string

	// Step completion tracking
	completedSteps     map[int]time.Duration // Maps step number to duration
	completedStepNames map[int]string        // Maps step number to step name

	// Dimensions
	width  int
	height int

	// Message channel for receiving updates
	msgChan chan tea.Msg
}

// Init initializes the model
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		listenForMessages(m.msgChan),
		m.spinner.Tick,
		tickCmd(),
	)
}

// listenForMessages listens for messages from the pipeline
func listenForMessages(msgChan chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		return <-msgChan
	}
}

// Update handles messages and updates the model
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		// Update viewport sizes when window is resized
		m.updateViewportSizes()

		// Force viewport content refresh to prevent cutoff
		m.updateLogViewport()

		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			// Exit TUI - shutdown countdown will happen in main()
			// Don't kill processes yet - let user cancel if needed
			return m, tea.Quit

		case "t":
			// Terminate - same as quit
			return m, tea.Quit

		case "c":
			// Toggle current sub-tool command popup
			m.showCommandPopup = !m.showCommandPopup
			if m.showCommandPopup {
				m.showRegisCommand = false // Close other popup
			}

		case "e":
			// Toggle entire regis command popup
			m.showRegisCommand = !m.showRegisCommand
			if m.showRegisCommand {
				m.showCommandPopup = false // Close other popup
			}

		case "l":
			// Toggle fullscreen logs view
			m.showFullscreenLogs = !m.showFullscreenLogs
			if m.showFullscreenLogs {
				m.showCommandPopup = false
				m.showRegisCommand = false
				m.showHelp = false
			}

		case "esc":
			// Close any open popups
			m.showCommandPopup = false
			m.showRegisCommand = false
			m.showHelp = false
			m.showFullscreenLogs = false

		case "?":
			m.showHelp = !m.showHelp

		case "1":
			// Switch to Logs tab
			m.activeTab = 1

		case "2":
			// Switch to Summary tab
			m.activeTab = 2

		case "tab":
			m.focusedPanel = (m.focusedPanel + 1) % 2

		case "up", "k":
			if m.focusedPanel == 0 {
				m.commandViewport.LineUp(1)
			} else {
				m.logViewport.LineUp(1)
			}

		case "down", "j":
			if m.focusedPanel == 0 {
				m.commandViewport.LineDown(1)
			} else {
				m.logViewport.LineDown(1)
			}
		}

	case tea.MouseMsg:
		// Handle mouse for viewport scrolling
		if msg.Type == tea.MouseWheelUp {
			if m.focusedPanel == 0 {
				m.commandViewport.LineUp(3)
			} else {
				m.logViewport.LineUp(3)
			}
		} else if msg.Type == tea.MouseWheelDown {
			if m.focusedPanel == 0 {
				m.commandViewport.LineDown(3)
			} else {
				m.logViewport.LineDown(3)
			}
		}

	case TickMsg:
		// Update system metrics every second
		if cpuPercent, err := cpu.Percent(100*time.Millisecond, false); err == nil && len(cpuPercent) > 0 {
			m.cpuPercent = cpuPercent[0]
		}
		if memStats, err := mem.VirtualMemory(); err == nil {
			m.memPercent = memStats.UsedPercent
			m.memUsedGB = float64(memStats.Used) / 1024 / 1024 / 1024
			m.memTotalGB = float64(memStats.Total) / 1024 / 1024 / 1024
		}

		// Harmonica spring physics for smooth color transitions
		// Spring parameters
		const stiffness = 0.3
		const damping = 0.8
		const deltaTime = 0.1

		// Target oscillates to create continuous motion
		target := math.Sin(m.animationTime * 0.5)

		// Calculate spring force
		displacement := target - m.springPosition
		springForce := displacement * stiffness
		dampingForce := -m.springVelocity * damping

		// Update velocity and position using spring physics
		m.springVelocity += (springForce + dampingForce) * deltaTime
		m.springPosition += m.springVelocity * deltaTime

		// Increment animation time for smooth transitions
		m.animationTime += 0.1
		return m, tickCmd()

	case PipelineMetadataMsg:
		m.metadata = msg

	case StepStartMsg:
		m.currentStep = msg.StepNumber
		m.currentStepName = msg.StepName
		m.currentCommand = "" // Reset command for new step
		m.stepProgress = 0
		m.stepStartTime = time.Now()
		m.spinnerRunning = true

		// Add cheerful startup emoji
		m.logs = append(m.logs, LogEntryMsg{
			Timestamp: time.Now(),
			Level:     "info",
			Message:   fmt.Sprintf("(｡◕‿◕｡) Starting %s...", msg.StepName),
		})
		m.updateLogViewport()
		cmds = append(cmds, listenForMessages(m.msgChan))

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		if m.spinnerRunning {
			cmds = append(cmds, cmd)
		}

	case StepProgressMsg:
		// Smooth progress animation using target value
		m.stepProgress = msg.Progress
		m.stepETA = msg.StepETA
		m.pipelineETA = msg.PipelineETA
		cmds = append(cmds, listenForMessages(m.msgChan))

	case StepCompleteMsg:
		m.stepProgress = 1.0
		m.spinnerRunning = false // Stop spinner when step completes

		// Calculate duration from step start time if not provided
		actualDuration := msg.Duration
		if actualDuration == 0 && !m.stepStartTime.IsZero() {
			actualDuration = time.Since(m.stepStartTime)
		}

		// Track completed step duration and name
		// Always track if we have a valid duration (either from msg or calculated)
		if actualDuration > 0 {
			m.completedSteps[msg.StepNumber] = actualDuration
			m.completedStepNames[msg.StepNumber] = m.currentStepName // Store the step name
		}

		// Add motivational emoji
		var emoji string
		if msg.Success {
			emoji = "(ﾉ◕ヮ◕)ﾉ*:･ﾟ✧  " // Celebratory emoji for success
		} else {
			emoji = "(｡•́︿•̀｡)  " // Sad emoji for failure
		}

		duration := ""
		if actualDuration > 0 {
			duration = fmt.Sprintf(" in %s", formatDuration(actualDuration))
		}

		m.logs = append(m.logs, LogEntryMsg{
			Timestamp: time.Now(),
			Level:     "info",
			Message:   emoji + fmt.Sprintf("Step completed%s", duration),
		})
		m.updateLogViewport()
		cmds = append(cmds, listenForMessages(m.msgChan))

	case LogEntryMsg:
		m.logs = append(m.logs, msg)
		m.updateLogViewport()
		cmds = append(cmds, listenForMessages(m.msgChan))

	case CommandOutputMsg:
		// Replace current command (not append)
		m.currentCommand = msg.Output
		m.updateLogViewport() // Update to show command at top
		cmds = append(cmds, listenForMessages(m.msgChan))

	case PipelineCompleteMsg:
		m.PipelineRunning = false
		m.PipelineSuccess = msg.Success
		m.FailureReason = msg.FailureReason
		m.spinnerRunning = false

		// Clear current command and step to remove "Running" display
		m.currentCommand = ""
		m.currentStepName = ""

		// Add motivational emoji based on success
		var emoji string
		var message string
		if msg.Success {
			emoji = "(◕‿◕)  " // Happy face for success
			message = emoji + "Pipeline completed successfully! Great work!"
		} else {
			emoji = "(╥﹏╥)  " // Sad face for failure
			message = emoji + "Pipeline failed. Don't give up - try again!"
		}

		m.logs = append(m.logs, LogEntryMsg{
			Timestamp: time.Now(),
			Level:     "info",
			Message:   message,
		})
		m.updateLogViewport()

		// Auto-exit after 3 seconds
		cmds = append(cmds, listenForMessages(m.msgChan))
		cmds = append(cmds, tea.Tick(3*time.Second, func(_ time.Time) tea.Msg {
			return QuitMsg{}
		}))

	case QuitMsg:
		return m, tea.Quit
	}

	return m, tea.Batch(cmds...)
}

// View renders the TUI
func (m Model) View() string {
	if m.showHelp {
		return renderHelp(m.width, m.height)
	}

	// Show fullscreen logs if active
	if m.showFullscreenLogs {
		return renderFullscreenLogs(m)
	}

	baseView := renderMainView(m)

	// Show regis command popup if active
	if m.showRegisCommand {
		return renderRegisCommandPopup(m, baseView)
	}

	// Show current sub-tool command popup if active
	if m.showCommandPopup && m.currentCommand != "" {
		return renderCommandPopup(m, baseView)
	}

	return baseView
}

// updateViewportSizes updates viewport dimensions based on window size
func (m *Model) updateViewportSizes() {
	if m.width < 40 || m.height < 20 {
		return // Too small to render
	}

	// Calculate available space for log viewport
	// Make reserved height adaptive based on terminal size
	// Logo section (~13 lines) + Step banner (~3) + Running Tool (~3) + Metrics (~1) + LogPanelOverhead (~6) = ~26 lines
	// But for smaller terminals, we need to be more flexible
	reservedHeight := 26

	// Adaptive reserved height based on terminal size
	if m.height < 40 {
		// Small terminal: reduce reserved space to make log viewport usable
		reservedHeight = 20
	} else if m.height < 50 {
		// Medium terminal: moderate reserved space
		reservedHeight = 24
	}

	logHeight := m.height - reservedHeight
	if logHeight < 5 {
		logHeight = 5
	}

	// Command viewport is used for popups, give it reasonable size (half screen)
	// It should not take space from the main log viewport, so we give it a fixed size.
	commandHeight := m.height / 2
	if commandHeight < 10 {
		commandHeight = 10
	}

	m.commandViewport.Width = m.width - 6
	m.commandViewport.Height = commandHeight

	m.logViewport.Width = m.width - 6
	m.logViewport.Height = logHeight
}

// updateCommandViewport updates command viewport content
func (m *Model) updateCommandViewport() {
	content := ""
	for _, line := range m.commandOutput {
		content += line + "\n"
	}
	m.commandViewport.SetContent(content)
	m.commandViewport.GotoBottom()
}

// updateLogViewport updates log viewport content
func (m *Model) updateLogViewport() {
	var content strings.Builder

	// Don't show command in log viewport - it's in the heading now
	// Just show logs with word wrapping
	maxWidth := m.logViewport.Width - 2 // Leave margin
	if maxWidth < 40 {
		maxWidth = 80
	}

	for _, log := range m.logs {
		logLine := formatLogEntry(log)
		wrapped := wrapText(logLine, maxWidth)
		content.WriteString(wrapped)
		content.WriteString("\n")
	}

	m.logViewport.SetContent(content.String())
	m.logViewport.GotoBottom()
}

// wrapText wraps text to fit within maxWidth
func wrapText(text string, maxWidth int) string {
	if len(text) <= maxWidth {
		return text
	}

	var result strings.Builder
	words := strings.Fields(text)
	lineLen := 0

	for i, word := range words {
		wordLen := len(word)

		if lineLen+wordLen+1 > maxWidth {
			result.WriteString("\n")
			result.WriteString(word)
			lineLen = wordLen
		} else {
			if i > 0 {
				result.WriteString(" ")
				lineLen++
			}
			result.WriteString(word)
			lineLen += wordLen
		}
	}

	return result.String()
}

// formatLogEntry formats a log entry with colorful icons
func formatLogEntry(log LogEntryMsg) string {
	// Colorful icons based on log level
	var levelIcon string
	var iconColor string

	switch log.Level {
	case "error":
		levelIcon = "✗"
		iconColor = "#FF5555" // Red
	case "warn":
		levelIcon = "⚠"
		iconColor = "#FFB86C" // Orange/Yellow
	case "debug":
		levelIcon = "⚙"
		iconColor = "#00FFFF" // Cyan
	case "info":
		levelIcon = "✓"
		iconColor = "#50FA7B" // Green
	default:
		levelIcon = "ℹ"
		iconColor = "#BD93F9" // Purple
	}

	coloredIcon := lipgloss.NewStyle().
		Foreground(lipgloss.Color(iconColor)).
		Bold(true).
		Render(levelIcon)

	timestampStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6272A4"))

	return fmt.Sprintf("%s %s %s",
		timestampStyle.Render("["+log.Timestamp.Format("15:04:05")+"]"),
		coloredIcon,
		log.Message,
	)
}

// NewModel creates a new TUI model
func NewModel(msgChan chan tea.Msg) Model {
	// Initialize with default dimensions
	width := 120
	height := 40

	// Create viewports with proper sizes
	cmdVP := viewport.New(width-4, 15)
	cmdVP.Style = lipgloss.NewStyle()

	logVP := viewport.New(width-4, 15)
	logVP.Style = lipgloss.NewStyle()

	// Initialize progress bar
	prog := progress.New(progress.WithDefaultGradient())

	// Initialize spinner
	spin := spinner.New()
	spin.Spinner = spinner.Points                                          // More vibrant style: ⣾⣽⣻⢿⡿⣟⣯⣷
	spin.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#00FFFF")) // Bright cyan

	return Model{
		msgChan:            msgChan,
		commandViewport:    cmdVP,
		logViewport:        logVP,
		progressBar:        prog,
		spinner:            spin,
		spinnerRunning:     false,
		focusedPanel:       1, // Default focus on log panel for scrolling
		activeTab:          1, // Default to Logs tab
		commandTabExpanded: false,
		showHelp:           false,
		PipelineRunning:    true,
		logs:               make([]LogEntryMsg, 0),
		commandOutput:      make([]string, 0),
		completedSteps:     make(map[int]time.Duration), // Initialize step tracking
		completedStepNames: make(map[int]string),        // Initialize step name tracking
		width:              width,
		height:             height,
	}
}

// tickCmd returns a command that sends tick messages for animations and metrics updates
func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return TickMsg(t)
	})
}
