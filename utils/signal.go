package utils

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"go.uber.org/zap"
)

// ShutdownMsg is sent to TUI when shutdown is initiated
type ShutdownMsg struct {
	Reason string
}

// SetupSignalHandler creates a context that cancels on SIGINT/SIGTERM
func SetupSignalHandler() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		Warn("Received interrupt signal, shutting down gracefully...",
			zap.String("signal", sig.String()))

		// Send shutdown message to TUI if available
		if tuiMode && tuiProgram != nil {
			if prog, ok := tuiProgram.(interface{ Send(tea.Msg) }); ok {
				prog.Send(ShutdownMsg{
					Reason: "User interrupted (Ctrl+C)",
				})
			}
		}

		// Cancel context to stop all running commands
		cancel()

		// Give processes 2 seconds to clean up gracefully
		time.Sleep(2 * time.Second)

		// Force exit if still running
		Info("Shutdown complete")
		if Logger != nil {
			Logger.Sync()
		}
	}()

	return ctx, cancel
}
