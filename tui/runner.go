package tui

import (
	"context"
	"time"

	"github.com/BioinformaticsOnLine/regis/config"
	tea "github.com/charmbracelet/bubbletea"
)

// RunPipeline executes the pipeline and sends updates to the TUI
func RunPipeline(ctx context.Context, cfg *config.Config, program *tea.Program) error {
	// Send initial metadata
	metadata := PipelineMetadataMsg{
		StartTime:      time.Now(),
		DataType:       cfg.DataType,
		Method:         cfg.Method,
		Cores:          cfg.Threads,
		Species:        cfg.Species,
		ValidationMode: cfg.ValidationMode,
		LncTarMode:     getLncTarMode(cfg),
		IntaRNAMode:    getIntaRNAMode(cfg),
	}
	program.Send(metadata)

	// TODO: Integrate with actual pipeline execution
	// For now, this is a placeholder that will be replaced with actual pipeline integration

	return nil
}

// getLncTarMode returns a human-readable LncTar mode string
func getLncTarMode(cfg *config.Config) string {
	if !cfg.EnableLncTar {
		return "off"
	}
	if cfg.LncTarBestOnly {
		return "best candidates"
	}
	if cfg.LncTarComprehensive {
		return "all lncRNAs"
	}
	return "highly expressed"
}

// getIntaRNAMode returns a human-readable IntaRNA mode string
func getIntaRNAMode(cfg *config.Config) string {
	if !cfg.EnableIntaRNA {
		return "off"
	}
	if cfg.IntaRNABestOnly {
		return "best candidates"
	}
	if cfg.IntaRNAComprehensive {
		return "all lncRNAs"
	}
	return "highly expressed"
}

// SendStepStart sends a step start message to the TUI
func SendStepStart(program *tea.Program, stepNum int, stepName string, command string) {
	program.Send(StepStartMsg{
		StepNumber: stepNum,
		StepName:   stepName,
		Command:    command,
	})
}

// SendStepProgress sends a progress update to the TUI
func SendStepProgress(program *tea.Program, progress float64, stepETA, pipelineETA time.Duration) {
	program.Send(StepProgressMsg{
		Progress:    progress,
		StepETA:     stepETA,
		PipelineETA: pipelineETA,
	})
}

// SendStepComplete sends a step completion message to the TUI
func SendStepComplete(program *tea.Program, stepNum int, success bool, duration time.Duration) {
	program.Send(StepCompleteMsg{
		StepNumber: stepNum,
		Success:    success,
		Duration:   duration,
	})
}

// SendLog sends a log entry to the TUI
func SendLog(program *tea.Program, level, message, caller string) {
	program.Send(LogEntryMsg{
		Level:     level,
		Timestamp: time.Now(),
		Message:   message,
		Caller:    caller,
	})
}

// SendCommandOutput sends command output to the TUI
func SendCommandOutput(program *tea.Program, output string) {
	program.Send(CommandOutputMsg{
		Output: output,
	})
}

// SendPipelineComplete sends pipeline completion message
func SendPipelineComplete(program *tea.Program, success bool, duration time.Duration) {
	program.Send(PipelineCompleteMsg{
		Success:  success,
		Duration: duration,
	})
}
