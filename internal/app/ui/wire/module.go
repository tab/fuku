package wire

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	"go.uber.org/fx"

	"fuku/internal/app/bus"
	"fuku/internal/app/monitor"
	"fuku/internal/app/ui/services"
	"fuku/internal/config/logger"
)

// UI creates a Bubble Tea program for the TUI
type UI func(ctx context.Context, profile string) (*tea.Program, error)

// Module aggregates all UI modules and provides the UI factory
var Module = fx.Options(
	services.Module,
	fx.Provide(NewUI),
)

// UIParams contains dependencies for creating the UI factory
type UIParams struct {
	fx.In

	Bus        bus.Bus
	Controller services.Controller
	Monitor    monitor.Monitor
	Loader     *services.Loader
	Logger     logger.Logger
}

// NewUI creates a factory function for constructing Bubble Tea programs
func NewUI(params UIParams) UI {
	return func(ctx context.Context, profile string) (*tea.Program, error) {
		model := services.NewModel(
			ctx,
			profile,
			params.Bus,
			params.Controller,
			params.Monitor,
			params.Loader,
			params.Logger,
		)

		p := tea.NewProgram(
			model,
			tea.WithAltScreen(),
			tea.WithContext(ctx),
		)

		params.Logger.Debug().Msg("TUI: Program created via factory")

		return p, nil
	}
}
