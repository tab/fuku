package cli

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"

	"fuku/internal/app/bus"
	"fuku/internal/app/logs"
	"fuku/internal/app/runner"
	"fuku/internal/app/ui/wire"
	"fuku/internal/app/watcher"
	"fuku/internal/config"
	"fuku/internal/config/logger"
	"fuku/internal/config/template"
)

// Help text constants
const (
	Usage = `Usage:
  fuku                            Run services with default profile (with TUI)

  fuku init                       Generate fuku.yaml template (--init, -i, init, i)

  fuku run <profile>              Run services with specified profile
  fuku --run <profile>            Same as above (--run, -r, run, r)
  fuku run <profile> --no-ui      Run services without TUI

  fuku stop                       Stop services with default profile
  fuku stop <profile>             Stop services with specified profile
  fuku --stop <profile>           Same as above (--stop, -s, stop, s)

  fuku logs [service...]          Stream logs from running services
  fuku --logs                     Same as above (--logs, -l, logs, l)
  fuku logs --profile <name> [service...] Stream logs from specific profile

  fuku help                       Show help (--help, -h, help)
  fuku version                    Show version (--version, -v, version)

Examples:
  fuku                            Run default profile with TUI
  fuku init                       Generate fuku.yaml in current directory
  fuku run core --no-ui           Run core services without TUI
  fuku -r core --no-ui            Same as above using flag
  fuku stop                       Stop all services (default profile)
  fuku stop backend               Stop backend services
  fuku logs                       Stream all logs from running fuku
  fuku logs api auth              Stream logs from api and auth services
  fuku -l                         Stream logs using flag`
)

// CLI defines the interface for cli operations
type CLI interface {
	Execute() (exitCode int, err error)
}

// cli represents the command-line interface for the application
type cli struct {
	cmd      *Options
	bus      bus.Bus
	runner   runner.Runner
	watcher  watcher.Watcher
	streamer logs.Runner
	ui       wire.UI
	log      logger.Logger
}

// NewCLI creates a new cli instance
func NewCLI(
	cmd *Options,
	b bus.Bus,
	runner runner.Runner,
	watcher watcher.Watcher,
	streamer logs.Runner,
	ui wire.UI,
	log logger.Logger,
) CLI {
	return &cli{
		cmd:      cmd,
		bus:      b,
		runner:   runner,
		watcher:  watcher,
		streamer: streamer,
		ui:       ui,
		log:      log.WithComponent("CLI"),
	}
}

// Execute processes the parsed command and executes the appropriate handler
func (c *cli) Execute() (int, error) {
	ctx := context.Background()

	c.bus.Publish(bus.Message{
		Type: bus.EventCommandStarted,
		Data: bus.CommandStarted{
			Command: c.cmd.Type.String(),
			Profile: c.cmd.Profile,
			UI:      !c.cmd.NoUI,
		},
	})

	switch c.cmd.Type {
	case CommandHelp:
		return c.handleHelp()
	case CommandInit:
		return c.handleInit()
	case CommandStop:
		return c.handleStop(ctx, c.cmd.Profile)
	case CommandVersion:
		return c.handleVersion()
	case CommandLogs:
		return c.handleLogs()
	default:
		return c.handleRun(ctx, c.cmd.Profile)
	}
}

// handleRun executes the run command with the specified profile
func (c *cli) handleRun(ctx context.Context, profile string) (int, error) {
	c.log.Debug().Msgf("Running with profile: %s", profile)

	c.watcher.Start(ctx)
	defer c.watcher.Close()

	if !c.cmd.NoUI {
		return c.runWithUI(ctx, profile)
	}

	if err := c.runner.Run(ctx, profile); err != nil {
		c.log.Error().Err(err).Msgf("Failed to run profile '%s'", profile)
		return 1, err
	}

	return 0, nil
}

// handleStop kills processes in service directories for the given profile
func (c *cli) handleStop(ctx context.Context, profile string) (int, error) {
	c.log.Debug().Msgf("Stopping services for profile: %s", profile)

	if err := c.runner.Stop(ctx, profile); err != nil {
		c.log.Error().Err(err).Msgf("Failed to stop profile '%s'", profile)

		return 1, err
	}

	return 0, nil
}

// runWithUI runs the TUI alongside the runner
func (c *cli) runWithUI(ctx context.Context, profile string) (int, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	program, err := c.ui(ctx, profile)
	if err != nil {
		c.log.Error().Err(err).Msg("Failed to create UI")
		return 1, err
	}

	runnerErrChan := make(chan error, 1)

	go func() {
		runnerErrChan <- c.runner.Run(ctx, profile)
	}()

	if _, err := program.Run(); err != nil {
		c.log.Error().Err(err).Msg("UI error")

		cancel()
		<-runnerErrChan

		return 1, err
	}

	cancel()

	if err := <-runnerErrChan; err != nil {
		c.log.Error().Err(err).Msgf("Failed to run profile '%s'", profile)
		return 1, err
	}

	return 0, nil
}

// handleInit generates a fuku.yaml template in the current directory
func (c *cli) handleInit() (int, error) {
	c.log.Debug().Msg("Initializing configuration")

	f, err := os.OpenFile(config.ConfigFile, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if errors.Is(err, fs.ErrExist) {
		fmt.Printf("%s already exists\n", config.ConfigFile)
		return 0, nil
	}

	if err != nil {
		return 1, fmt.Errorf("failed to write %s: %w", config.ConfigFile, err)
	}

	defer f.Close()

	if _, err := f.Write(template.Content); err != nil {
		return 1, fmt.Errorf("failed to write %s: %w", config.ConfigFile, err)
	}

	fmt.Printf("Created %s\n", config.ConfigFile)

	return 0, nil
}

// handleLogs streams logs from a running fuku instance
func (c *cli) handleLogs() (int, error) {
	return c.streamer.Run(c.cmd.Profile, c.cmd.Services), nil
}

// handleHelp displays help information
func (c *cli) handleHelp() (int, error) {
	c.log.Debug().Msg("Displaying help information")
	fmt.Println(Usage)

	return 0, nil
}

// handleVersion displays version information
func (c *cli) handleVersion() (int, error) {
	c.log.Debug().Msg("Displaying version information")
	fmt.Printf("Version: %s\n", config.Version)

	return 0, nil
}
