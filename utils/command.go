package utils

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/BioinformaticsOnLine/regis/tui"
	tea "github.com/charmbracelet/bubbletea"
	"go.uber.org/zap"
)

// RunCommand executes a command and logs output
func RunCommand(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Build full command string for display
	fullCmd := fmt.Sprintf("%s %s", name, strings.Join(args, " "))
	Info(fmt.Sprintf("Running: %s", fullCmd))

	// Send command to TUI if available
	if tuiMode && tuiProgram != nil {
		if prog, ok := tuiProgram.(interface{ Send(tea.Msg) }); ok {
			prog.Send(tui.CommandOutputMsg{
				Output: fullCmd,
			})
		}
	}

	err := cmd.Run()

	// Log stdout if present - send each line to TUI
	if stdout.Len() > 0 {
		output := stdout.String()
		// Split into lines and send each as info for real-time display
		lines := strings.Split(strings.TrimSpace(output), "\n")
		for _, line := range lines {
			if line != "" {
				Info(line) // Send directly to TUI log
			}
		}
	}

	// Log stderr if present
	if stderr.Len() > 0 {
		output := stderr.String()
		lines := strings.Split(strings.TrimSpace(output), "\n")
		for _, line := range lines {
			if line != "" {
				Warn(line) // Send as warning to TUI log
			}
		}
	}

	if err != nil {
		Error("Command failed",
			zap.String("cmd", name),
			zap.Strings("args", args),
			zap.Error(err),
			zap.String("stderr", stderr.String()))
		return fmt.Errorf("%s failed: %w\nStderr: %s", name, err, stderr.String())
	}

	Info(fmt.Sprintf("✓ %s completed successfully", name))
	return nil
}

// RunCommandWithOutput executes a command and returns stdout
func RunCommandWithOutput(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	Info(fmt.Sprintf("Running: %s %s", name, strings.Join(args, " ")))

	err := cmd.Run()

	if stderr.Len() > 0 {
		Debug("Command stderr", zap.String("output", stderr.String()))
	}

	if err != nil {
		Error("Command failed",
			zap.String("cmd", name),
			zap.Strings("args", args),
			zap.Error(err),
			zap.String("stderr", stderr.String()))
		return "", fmt.Errorf("%s failed: %w\nStderr: %s", name, err, stderr.String())
	}

	Info(fmt.Sprintf("✓ %s completed successfully", name))
	return stdout.String(), nil
}

// RunCommandInDir executes a command in a specific directory
func RunCommandInDir(ctx context.Context, dir, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	Info(fmt.Sprintf("Running in %s: %s %s", dir, name, strings.Join(args, " ")))

	err := cmd.Run()

	if stdout.Len() > 0 {
		Debug("Command stdout", zap.String("output", stdout.String()))
	}

	if stderr.Len() > 0 {
		Debug("Command stderr", zap.String("output", stderr.String()))
	}

	if err != nil {
		Error("Command failed",
			zap.String("cmd", name),
			zap.String("dir", dir),
			zap.Strings("args", args),
			zap.Error(err),
			zap.String("stderr", stderr.String()))
		return fmt.Errorf("%s failed: %w\nStderr: %s", name, err, stderr.String())
	}

	Info(fmt.Sprintf("✓ %s completed successfully", name))
	return nil
}
