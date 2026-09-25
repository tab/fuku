package tui

import (
	"context"
	"errors"

	tea "charm.land/bubbletea/v2"
	"go.uber.org/fx"

	"fuku/internal/adapters/terminal"
)

// Output switches the application log output on once the view no longer owns the terminal
type Output interface {
	SetEnabled(enabled bool)
}

// ProgramParams contains the dependencies of the services view
type ProgramParams struct {
	fx.In

	Options     Options
	Bridge      *Bridge
	Control     Control
	Registry    Registry
	Monitor     Monitor
	Environment Environment
	Theme       func() terminal.Theme
	Writer      Output
	Logger      Logger
}

// Program runs the services view as the command of the run
type Program struct {
	params  ProgramParams
	options []tea.ProgramOption // set by a test that runs without a terminal
}

// NewProgram creates the services view command
func NewProgram(params ProgramParams) *Program {
	return &Program{params: params}
}

// Run runs the view on the bridge until it quits or ctx ends, then gives the terminal back to the log
func (p *Program) Run(ctx context.Context) (int, error) {
	model := NewModel(ctx, ModelParams{
		Profile:       p.params.Options.Profile,
		RetryAttempts: p.params.Options.RetryAttempts,
		RetryBackoff:  p.params.Options.RetryBackoff,
		Control:       p.params.Control,
		Registry:      p.params.Registry,
		Monitor:       p.params.Monitor,
		Environment:   p.params.Environment,
		Theme:         p.params.Theme(),
		Logger:        p.params.Logger,
	})

	program := tea.NewProgram(model, append([]tea.ProgramOption{tea.WithContext(ctx), tea.WithoutSignalHandler()}, p.options...)...)

	p.params.Bridge.attach(program)

	_, err := program.Run()

	p.params.Writer.SetEnabled(true)

	if err != nil && !errors.Is(err, context.Canceled) {
		return 1, err
	}

	return 0, nil
}
