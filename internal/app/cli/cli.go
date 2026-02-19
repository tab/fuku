package cli

import (
	"context"
	"fmt"

	"fuku/internal/app/errors"
	"fuku/internal/app/logs"
	"fuku/internal/app/runner"
	"fuku/internal/app/session"
	"fuku/internal/app/ui/wire"
	"fuku/internal/app/watcher"
	"fuku/internal/config"
	"fuku/internal/config/logger"
)

// Help text constants
const (
	Usage = `Usage:
  fuku                            Run services with default profile (with TUI)
  fuku run <profile>              Run services with specified profile
  fuku --run <profile>            Same as above (--run, -r, run, r)
  fuku run <profile> --no-ui      Run services without TUI

  fuku logs [service...]          Stream logs from running services
  fuku --logs                     Same as above (--logs, -l, logs, l)
  fuku logs --profile <name> [service...] Stream logs from specific profile

  fuku stop                       Stop orphaned processes (stop, s)

  fuku help                       Show help (--help, -h, help)
  fuku version                    Show version (--version, -v, version)

Examples:
  fuku                            Run default profile with TUI

  fuku run core --no-ui           Run core services without TUI
  fuku -r core --no-ui            Same as above using flag

  fuku logs                       Stream all logs from running fuku
  fuku logs api auth              Stream logs from api and auth services
  fuku -l                         Stream logs using flag

  fuku stop                       Kill orphaned processes from crashed session`
)

// CLI defines the interface for cli operations
type CLI interface {
	Execute() (exitCode int, err error)
}

// cli represents the command-line interface for the application
type cli struct {
	cmd      *Options
	runner   runner.Runner
	watcher  watcher.Watcher
	listener session.Listener
	session  session.Session
	streamer logs.Runner
	ui       wire.UI
	log      logger.Logger
}

// NewCLI creates a new cli instance
func NewCLI(
	cmd *Options,
	runner runner.Runner,
	watcher watcher.Watcher,
	listener session.Listener,
	session session.Session,
	streamer logs.Runner,
	ui wire.UI,
	log logger.Logger,
) CLI {
	return &cli{
		cmd:      cmd,
		runner:   runner,
		watcher:  watcher,
		listener: listener,
		session:  session,
		streamer: streamer,
		ui:       ui,
		log:      log.WithComponent("CLI"),
	}
}

// Execute processes the parsed command and executes the appropriate handler
func (c *cli) Execute() (int, error) {
	switch c.cmd.Type {
	case CommandHelp:
		return c.handleHelp()
	case CommandVersion:
		return c.handleVersion()
	case CommandLogs:
		return c.handleLogs()
	case CommandStop:
		return c.handleStop()
	default:
		return c.handleRun(c.cmd.Profile)
	}
}

// handleRun executes the run command with the specified profile
func (c *cli) handleRun(profile string) (int, error) {
	c.log.Debug().Msgf("Running with profile: %s", profile)

	ctx := context.Background()

	c.listener.Start(ctx)

	c.watcher.Start(ctx)
	defer c.watcher.Close()

	if c.cmd.NoUI {
		if err := c.runner.Run(ctx, profile); err != nil {
			c.log.Error().Err(err).Msgf("Failed to run profile '%s'", profile)
			return 1, err
		}

		return 0, nil
	}

	return c.runWithUI(ctx, profile)
}

// runWithUI runs the TUI alongside the runner
func (c *cli) runWithUI(ctx context.Context, profile string) (int, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	runnerErrChan := make(chan error, 1)

	go func() {
		runnerErrChan <- c.runner.Run(ctx, profile)
	}()

	p, err := c.ui(ctx, profile)
	if err != nil {
		c.log.Error().Err(err).Msg("Failed to create UI")
		return 1, err
	}

	if _, err := p.Run(); err != nil {
		c.log.Error().Err(err).Msg("UI error")
		return 1, err
	}

	cancel()

	if err := <-runnerErrChan; err != nil {
		c.log.Error().Err(err).Msgf("Failed to run profile '%s'", profile)
		return 1, err
	}

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

// handleStop kills orphaned processes from a stale session
func (c *cli) handleStop() (int, error) {
	state, err := c.session.Load()
	if err != nil {
		if errors.Is(err, errors.ErrSessionNotFound) {
			fmt.Println("No active session found")

			return 0, nil
		}

		c.log.Error().Err(err).Msg("Failed to load session")

		return 1, err
	}

	killed := session.KillOrphans(state, c.log)

	if err := c.session.Delete(); err != nil {
		c.log.Warn().Err(err).Msg("Failed to delete session file")
	}

	if killed == 0 {
		fmt.Println("No orphaned processes found")
	} else {
		fmt.Printf("Stopped %d orphaned process(es)\n", killed)
	}

	return 0, nil
}

// handleVersion displays version information
func (c *cli) handleVersion() (int, error) {
	c.log.Debug().Msg("Displaying version information")
	fmt.Printf("Version: %s\n", config.Version)

	return 0, nil
}
