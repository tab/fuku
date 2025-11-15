package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	"fuku/internal/app/runner"
	"fuku/internal/app/runtime"
	"fuku/internal/app/ui/services"
	"fuku/internal/config"
	"fuku/internal/config/logger"
)

const (
	Usage = `Usage:
  fuku --run=<PROFILE>            Run services with specified profile (with TUI)
  fuku --run=<PROFILE> --no-ui    Run services without TUI
  fuku help                       Show help
  fuku version                    Show version

Examples:
  fuku --run=default              Run all services with TUI
  fuku --run=core --no-ui         Run core services without TUI
  fuku --run=minimal              Run minimal services with TUI

TUI Controls (Services View):
  ↑/↓ or k/j                      Navigate services
  pgup/pgdn/home/end              Scroll viewport
  r                               Restart selected service
  s                               Stop/start selected service
  space                           Toggle logs for selected service
  l                               Switch to logs view
  q                               Quit (stops all services)

TUI Controls (Logs View):
  ↑/↓ or k/j                      Scroll logs
  pgup/pgdn/home/end              Scroll viewport
  l                               Switch back to services view
  q                               Quit (stops all services)`
)

// CLI defines the interface for cli operations
type CLI interface {
	// Run processes command-line arguments and returns an exit code and error
	Run(args []string) (exitCode int, err error)
}

// cli represents the command-line interface for the application
type cli struct {
	cfg     *config.Config
	runner  runner.Runner
	log     logger.Logger
	event   runtime.EventBus
	command runtime.CommandBus
}

// NewCLI creates a new cli instance
func NewCLI(
	cfg *config.Config,
	runner runner.Runner,
	log logger.Logger,
	event runtime.EventBus,
	command runtime.CommandBus,
) CLI {
	return &cli{
		cfg:     cfg,
		runner:  runner,
		log:     log,
		event:   event,
		command: command,
	}
}

// Run processes command-line arguments and executes commands
func (c *cli) Run(args []string) (int, error) {
	noUI := false
	profile := config.DefaultProfile

	var remainingArgs []string

	for _, arg := range args {
		switch {
		case arg == "--no-ui":
			noUI = true
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
		return c.handleRun(profile, noUI)
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

		return c.handleRun(profile, noUI)
	default:
		return c.handleUnknown()
	}
}

// handleRun executes the run command with the specified profile
func (c *cli) handleRun(profile string, noUI bool) (int, error) {
	c.log.Debug().Msgf("Running with profile: %s", profile)

	ctx := context.Background()

	if noUI {
		if err := c.runner.Run(ctx, profile); err != nil {
			c.log.Error().Err(err).Msgf("Failed to run profile '%s'", profile)
			fmt.Printf("Error: %v\n", err)

			return 1, err
		}

		return 0, nil
	}

	p, _, err := services.Run(ctx, profile, c.event, c.command, c.log)
	if err != nil {
		c.log.Error().Err(err).Msg("Failed to create UI")
		fmt.Fprintf(os.Stderr, "Failed to create UI: %v\n", err)

		return 1, err
	}

	errChan := make(chan error, 1)

	go func() {
		errChan <- c.runner.Run(ctx, profile)
	}()

	if _, err := p.Run(); err != nil {
		c.log.Error().Err(err).Msg("UI error")
		fmt.Fprintf(os.Stderr, "UI error: %v\n", err)

		return 1, err
	}

	if err := <-errChan; err != nil {
		c.log.Error().Err(err).Msgf("Failed to run profile '%s'", profile)
		fmt.Printf("Error: %v\n", err)

		return 1, err
	}

	return 0, nil
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
