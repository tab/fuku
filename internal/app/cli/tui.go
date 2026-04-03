package cli

import (
	"context"

	"go.uber.org/fx"

	"fuku/internal/app/bus"
	"fuku/internal/app/logs"
	"fuku/internal/app/runner"
	"fuku/internal/app/ui/wire"
	"fuku/internal/app/watcher"
	"fuku/internal/config/logger"
)

// TUI defines the interface for terminal UI operations
type TUI interface {
	Execute() (exitCode int, err error)
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
		log:      p.Logger.WithComponent("TUI"),
	}
}

// Execute processes the parsed command and executes the appropriate handler
func (t *tui) Execute() (int, error) {
	ctx := context.Background()

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
		return t.handleLogs()
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
		t.log.Error().Err(err).Msg("Failed to create UI")
		return 1, err
	}

	runnerErrChan := make(chan error, 1)

	go func() {
		runnerErrChan <- t.runner.Run(ctx, profile)
	}()

	if _, err := program.Run(); err != nil {
		t.log.Error().Err(err).Msg("UI error")

		cancel()
		<-runnerErrChan

		return 1, err
	}

	cancel()

	if err := <-runnerErrChan; err != nil {
		t.log.Error().Err(err).Msgf("Failed to run profile '%s'", profile)
		return 1, err
	}

	return 0, nil
}

// handleLogs streams logs from a running fuku instance
func (t *tui) handleLogs() (int, error) {
	return t.streamer.Run(t.cmd.Profile, t.cmd.Services), nil
}
