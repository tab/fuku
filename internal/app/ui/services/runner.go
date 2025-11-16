package services

import (
    "context"

    "github.com/charmbracelet/bubbles/spinner"
    tea "github.com/charmbracelet/bubbletea"

    "fuku/internal/app/monitor"
    "fuku/internal/app/runtime"
    "fuku/internal/app/ui"
    "fuku/internal/config/logger"
)

// Run starts the Bubble Tea program for the services UI
func Run(
    ctx context.Context,
    profile string,
    event runtime.EventBus,
    command runtime.CommandBus,
    controller Controller,
    monitor monitor.Monitor,
    filter ui.LogFilter,
    log logger.Logger,
) (*tea.Program, Model, error) {
    s := spinner.New()
    s.Spinner = spinner.Dot
    s.Style = spinnerStyle

    loader := &Loader{
        Model:  s,
        Active: false,
        queue:  make([]LoaderItem, 0),
    }

    model := NewModel(ctx, profile, event, command, controller, monitor, filter, loader, log)

    p := tea.NewProgram(
        model,
        tea.WithAltScreen(),
        tea.WithContext(ctx),
    )

    log.Debug().Msg("TUI: Program created, returning to CLI")

    return p, model, nil
}
