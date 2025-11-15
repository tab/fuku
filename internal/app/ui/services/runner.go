package services

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"

	"fuku/internal/app/runtime"
	"fuku/internal/config/logger"
)

// Run starts the Bubble Tea program for the services UI
func Run(ctx context.Context, profile string, event runtime.EventBus, command runtime.CommandBus, log logger.Logger) (*tea.Program, Model, error) {
	model := NewModel(ctx, profile, event, command, log)

	p := tea.NewProgram(
		model,
		tea.WithAltScreen(),
		tea.WithContext(ctx),
	)

	log.Debug().Msg("TUI: Program created, returning to CLI")

	return p, model, nil
}
