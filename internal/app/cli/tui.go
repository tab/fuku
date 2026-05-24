package cli

import (
	"context"

	"go.uber.org/fx"

	"fuku/internal/app/bus"
	"fuku/internal/app/errors"
	"fuku/internal/app/logs"
	"fuku/internal/app/render"
	"fuku/internal/app/runner"
	"fuku/internal/app/ui/wire"
	"fuku/internal/app/watcher"
	"fuku/internal/config/logger"
)

// TUI defines the interface for terminal UI operations
type TUI interface {
	Execute(ctx context.Context) (exitCode int, err error)
}

// TUIParams contains dependencies for creating a TUI
type TUIParams struct {
	fx.In

	Cmd      *Options
	Bus      bus.Bus
	Runner   runner.Runner
	Watcher  watcher.Watcher
	Streamer logs.Screen
	UI       wire.UI
	Writer   *render.Writer
	Logger   logger.Logger
}

// tui represents the terminal UI for the application
type tui struct {
	cmd      *Options
	bus      bus.Bus
	runner   runner.Runner
	watcher  watcher.Watcher
	streamer logs.Screen
	ui       wire.UI
	writer   *render.Writer
	log      logger.Logger
}

// NewTUI creates a new TUI instance
func NewTUI(p TUIParams) TUI {
	return &tui{
		cmd:      p.Cmd,
		bus:      p.Bus,
		runner:   p.Runner,
		watcher:  p.Watcher,
		streamer: p.Streamer,
		ui:       p.UI,
		writer:   p.Writer,
		log:      p.Logger.WithComponent("TUI"),
	}
}

// Execute processes the parsed command and executes the appropriate handler
func (t *tui) Execute(ctx context.Context) (int, error) {
	t.bus.Publish(bus.Message{
		Type: bus.EventCommandStarted,
		Data: bus.CommandStarted{
			Command: t.cmd.Type.String(),
			Profile: t.cmd.Profile,
			UI:      !t.cmd.NoUI,
		},
	})

	switch t.cmd.Type {
	case CommandStop:
		return t.handleStop(ctx, t.cmd.Profile)
	case CommandLogs:
		return t.handleLogs(ctx)
	default:
		return t.handleRun(ctx, t.cmd.Profile)
	}
}

// handleRun executes the run command with the specified profile
func (t *tui) handleRun(ctx context.Context, profile string) (int, error) {
	t.log.Debug().Msgf("Running with profile: %s", profile)

	t.watcher.Start(ctx)
	defer t.watcher.Close()

	if !t.cmd.NoUI {
		return t.runWithUI(ctx, profile)
	}

	if err := t.runner.Run(ctx, profile); err != nil {
		t.log.Error().Err(err).Msgf("Failed to run profile '%s'", profile)
		return 1, err
	}

	return 0, nil
}

// handleStop kills processes in service directories for the given profile
func (t *tui) handleStop(ctx context.Context, profile string) (int, error) {
	t.log.Debug().Msgf("Stopping services for profile: %s", profile)

	if err := t.runner.Stop(ctx, profile); err != nil {
		t.log.Error().Err(err).Msgf("Failed to stop profile '%s'", profile)

		return 1, err
	}

	return 0, nil
}

// runWithUI runs the TUI alongside the runner
func (t *tui) runWithUI(ctx context.Context, profile string) (int, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	program, err := t.ui(ctx, profile)
	if err != nil {
		t.writer.SetEnabled(true)
		t.log.Error().Err(err).Msg("Failed to create UI")

		return 1, err
	}

	runnerErrChan := make(chan error, 1)

	go func() {
		runnerErrChan <- t.runner.Run(ctx, profile)
	}()

	uiErrChan := make(chan error, 1)

	go func() {
		_, err := program.Run()
		uiErrChan <- err
	}()

	var runnerErr, uiErr error

	select {
	case runnerErr = <-runnerErrChan:
		cancel()
		program.Quit()

		uiErr = <-uiErrChan
	case uiErr = <-uiErrChan:
		cancel()

		runnerErr = <-runnerErrChan
	}

	t.writer.SetEnabled(true)

	if runnerErr != nil {
		t.log.Error().Err(runnerErr).Msgf("Failed to run profile '%s'", profile)
		return 1, runnerErr
	}

	if uiErr != nil && !errors.Is(uiErr, context.Canceled) {
		t.log.Error().Err(uiErr).Msg("UI error")
		return 1, uiErr
	}

	return 0, nil
}

// handleLogs streams logs from a running fuku instance
func (t *tui) handleLogs(ctx context.Context) (int, error) {
	return t.streamer.Run(ctx, t.cmd.Profile, t.cmd.Services), nil
}
