package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	"fuku/internal/app/logs"
	"fuku/internal/app/runner"
	"fuku/internal/app/ui/wire"
	"fuku/internal/config"
	"fuku/internal/config/logger"
)

const (
	Usage = `Usage:
  fuku --run=<PROFILE>            Run services with specified profile (with TUI)
  fuku --run=<PROFILE> --no-ui    Run services without TUI
  fuku --logs [service...]        Stream logs from running services
  fuku help                       Show help
  fuku version                    Show version

Examples:
  fuku --run=default              Run all services with TUI
  fuku --run=core --no-ui         Run core services without TUI
  fuku --run=minimal              Run minimal services with TUI
  fuku --logs                     Stream all logs from running fuku
  fuku --logs api auth            Stream logs from api and auth services

TUI Controls:
  ↑/↓ or k/j                      Navigate services
  pgup/pgdn/home/end              Scroll viewport
  r                               Restart selected service
  s                               Stop/start selected service
  q                               Quit (stops all services)`
)

// CLI defines the interface for cli operations
type CLI interface {
	Run(args []string) (exitCode int, err error)
}

// cli represents the command-line interface for the application
type cli struct {
	cfg        *config.Config
	options    config.Options
	runner     runner.Runner
	logsRunner logs.Runner
	ui         wire.UI
	log        logger.Logger
}

// NewCLI creates a new cli instance
func NewCLI(
	cfg *config.Config,
	options config.Options,
	runner runner.Runner,
	logsRunner logs.Runner,
	ui wire.UI,
	log logger.Logger,
) CLI {
	return &cli{
		cfg:        cfg,
		options:    options,
		runner:     runner,
		logsRunner: logsRunner,
		ui:         ui,
		log:        log,
	}
}

// Run processes command-line arguments and executes commands
func (c *cli) Run(args []string) (int, error) {
	if c.options.Logs {
		return c.handleLogs(args)
	}

	profile := config.DefaultProfile

	var remainingArgs []string

	for _, arg := range args {
		switch {
		case arg == "--no-ui":
			// already handled via options
		case strings.HasPrefix(arg, "--run="):
			profile = strings.TrimPrefix(arg, "--run=")
			if profile == "" {
				profile = config.DefaultProfile
			}
		default:
			remainingArgs = append(remainingArgs, arg)
		}
	}

	if len(remainingArgs) == 0 {
		return c.handleRun(profile)
	}

	cmd := remainingArgs[0]

	switch cmd {
	case "help", "--help", "-h":
		return c.handleHelp()
	case "version", "--version", "-v":
		return c.handleVersion()
	case "run", "-r":
		if len(remainingArgs) > 1 {
			profile = remainingArgs[1]
		}

		return c.handleRun(profile)
	default:
		return c.handleUnknown()
	}
}

// handleRun executes the run command with the specified profile
func (c *cli) handleRun(profile string) (int, error) {
	c.log.Debug().Msgf("Running with profile: %s", profile)

	ctx := context.Background()

	if c.options.NoUI {
		if err := c.runner.Run(ctx, profile); err != nil {
			c.log.Error().Err(err).Msgf("Failed to run profile '%s'", profile)
			fmt.Printf("Error: %v\n", err)

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
		fmt.Fprintf(os.Stderr, "Failed to create UI: %v\n", err)

		return 1, err
	}

	if _, err := p.Run(); err != nil {
		c.log.Error().Err(err).Msg("UI error")
		fmt.Fprintf(os.Stderr, "UI error: %v\n", err)

		return 1, err
	}

	cancel()

	if err := <-runnerErrChan; err != nil {
		c.log.Error().Err(err).Msgf("Failed to run profile '%s'", profile)
		fmt.Printf("Error: %v\n", err)

		return 1, err
	}

	return 0, nil
}

// handleLogs streams logs from a running fuku instance
func (c *cli) handleLogs(args []string) (int, error) {
	c.log.Debug().Msg("Running logs mode")

	return c.logsRunner.Run(args), nil
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

// handleUnknown handles unknown commands
func (c *cli) handleUnknown() (int, error) {
	c.log.Debug().Msg("Unknown command")
	fmt.Println("Unknown command. Use 'fuku help' for more information")

	return 1, nil
}
