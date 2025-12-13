package utils

import (
	"fmt"
	"os"
	"time"

	"github.com/BioinformaticsOnLine/regis/tui"
	tea "github.com/charmbracelet/bubbletea"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var Logger *zap.Logger
var tuiMode bool           // Flag to suppress stdout when TUI is active
var tuiProgram interface{} // Reference to TUI program for sending log messages

// SetTUIMode enables or disables TUI mode
func SetTUIMode(enabled bool) {
	tuiMode = enabled
}

// SetTUIProgram sets the TUI program reference for log forwarding
func SetTUIProgram(program interface{}) {
	tuiProgram = program
}

// InitLogger initializes the global logger with output to both console and file
func InitLogger(logFile string) error {
	// Create log file
	file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}

	// Console encoder (human-readable) - only if not in TUI mode
	consoleEncoder := zapcore.NewConsoleEncoder(zap.NewDevelopmentEncoderConfig())

	// File encoder (JSON for structured logs)
	fileEncoderConfig := zap.NewProductionEncoderConfig()
	fileEncoderConfig.TimeKey = "timestamp"
	fileEncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	fileEncoder := zapcore.NewJSONEncoder(fileEncoderConfig)

	// Create cores based on TUI mode
	var cores []zapcore.Core

	// Always log to file
	cores = append(cores, zapcore.NewCore(fileEncoder, zapcore.AddSync(file), zapcore.DebugLevel))

	// Only log to stdout if not in TUI mode
	if !tuiMode {
		cores = append(cores, zapcore.NewCore(consoleEncoder, zapcore.AddSync(os.Stdout), zapcore.InfoLevel))
	}

	// Create multi-output core
	// Create multi-output core
	core := zapcore.NewTee(cores...)

	// Add CallerSkip(1) so logs show the actual caller (e.g., modules/step_01.go) instead of utils/logger.go
	Logger = zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1))
	return nil
}

// Info logs an info message
func Info(msg string, fields ...zap.Field) {
	if Logger != nil {
		Logger.Info(msg, fields...)
		sendToTUI("info", msg)
	}
}

// Error logs an error message
func Error(msg string, fields ...zap.Field) {
	if Logger != nil {
		Logger.Error(msg, fields...)
		sendToTUI("error", msg)
	}
}

// Warn logs a warning message
func Warn(msg string, fields ...zap.Field) {
	if Logger != nil {
		Logger.Warn(msg, fields...)
		sendToTUI("warn", msg)
	}
}

// Debug logs a debug message
func Debug(msg string, fields ...zap.Field) {
	if Logger != nil {
		Logger.Debug(msg, fields...)
		sendToTUI("debug", msg)
	}
}

// sendToTUI is a helper to clean up duplication
func sendToTUI(level, msg string) {
	if tuiMode && tuiProgram != nil {
		if prog, ok := tuiProgram.(interface{ Send(tea.Msg) }); ok {
			prog.Send(tui.LogEntryMsg{
				Level:     level,
				Timestamp: time.Now(),
				Message:   msg,
				Caller:    "",
			})
		}
	}
}

// Sync flushes any buffered log entries
func Sync() {
	if Logger != nil {
		Logger.Sync()
	}
}

// StepHeader prints a step header banner
func StepHeader(stepNum int, title string) {
	// Only print to stdout if not in TUI mode
	if !tuiMode {
		fmt.Printf("\n====================================================================\n")
		fmt.Printf("Step %d: %s\n", stepNum, title)
		fmt.Printf("====================================================================\n\n")
	}
	
	// Log directly to Logger to respect AddCallerSkip(1)
	if Logger != nil {
		Logger.Info(fmt.Sprintf("Starting Step %d: %s", stepNum, title))
	}

	// Send to TUI if available
	if tuiMode && tuiProgram != nil {
		if prog, ok := tuiProgram.(interface{ Send(tea.Msg) }); ok {
			prog.Send(tui.StepStartMsg{
				StepNumber: stepNum,
				StepName:   title,
			})
		}
	}
}

// StepHeaderR is an alias for StepHeader (for backwards compatibility)
func StepHeaderR(stepNum int, title string) {
	StepHeader(stepNum, title)
}

// ShowProgress prints a progress message
func ShowProgress(msg string) {
	// Only print to stdout if not in TUI mode
	if !tuiMode {
		fmt.Printf("  → %s...\n", msg)
	}
}

// StepComplete prints step completion message with duration
func StepComplete(stepNum int, title string, startTime time.Time) {
	duration := time.Since(startTime)
	minutes := int(duration.Minutes())
	seconds := int(duration.Seconds()) % 60

	// Only print to stdout if not in TUI mode
	if !tuiMode {
		fmt.Printf("\n✓ Step %d completed in %d minutes and %d seconds\n", stepNum, minutes, seconds)
	}
	
	// Log directly to Logger
	if Logger != nil {
		Logger.Info("Step completed", zap.String("step", title), zap.Duration("duration", duration))
	}

	// Send step completion to TUI if available
	if tuiMode && tuiProgram != nil {
		if prog, ok := tuiProgram.(interface{ Send(tea.Msg) }); ok {
			prog.Send(tui.StepCompleteMsg{
				StepNumber: stepNum,
				Success:    true,
				Duration:   duration,
			})
		}
	}
}
