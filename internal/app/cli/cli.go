package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	"fuku/internal/app/runner"
	"fuku/internal/config"
	"fuku/internal/config/logger"
)

const (
	Usage = `Usage:
  fuku --run=<SCOPE>              Run the fuku with the specified scope
  fuku help                       Show help
  fuku version                    Show version

Examples:
  fuku --run=SAMPLE_SCOPE       Run the fuku with SAMPLE_SCOPE`
)

// CLI defines the interface for cli operations
type CLI interface {
	Run(args []string) error
}

// cli represents the command-line interface for the application
type cli struct {
	cfg    *config.Config
	runner runner.Runner
	log    logger.Logger
}

// NewCLI creates a new cli instance
func NewCLI(
	cfg *config.Config,
	runner runner.Runner,
	log logger.Logger,
) CLI {
	return &cli{
		cfg:    cfg,
		runner: runner,
		log:    log,
	}
}

// Run processes command-line arguments and executes commands
func (c *cli) Run(args []string) error {
	if len(args) == 0 {
		os.Exit(0)
		return nil
	}

	cmd := args[0]

	switch {
	case cmd == "help" || cmd == "--help" || cmd == "-h":
		c.handleHelp()
	case cmd == "version" || cmd == "--version" || cmd == "-v":
		c.handleVersion()
	case cmd == "run" || cmd == "--run" || cmd == "-r":
		scope := "default"
		if len(args) > 1 {
			scope = args[1]
		}
		c.handleRun(scope)
	case strings.HasPrefix(cmd, "--run="):
		scope := strings.TrimPrefix(cmd, "--run=")
		if scope == "" {
			scope = "default"
		}
		c.handleRun(scope)
	default:
		c.handleUnknown()
	}

	os.Exit(0)
	return nil
}

// handleRun executes the run command with the specified scope
func (c *cli) handleRun(scope string) {
	c.log.Debug().Msgf("Running with scope: %s", scope)

	ctx := context.Background()
	if err := c.runner.Run(ctx, scope); err != nil {
		c.log.Error().Err(err).Msgf("Failed to run scope '%s'", scope)
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}

// handleHelp displays help information
func (c *cli) handleHelp() {
	c.log.Debug().Msg("Displaying help information")
	fmt.Println(Usage)
}

// handleVersion displays version information
func (c *cli) handleVersion() {
	c.log.Debug().Msg("Displaying version information")
	fmt.Printf("Version: %s\n", config.Version)
}

// handleUnknown handles unknown commands
func (c *cli) handleUnknown() {
	c.log.Debug().Msg("Unknown command")
	fmt.Println("Unknown command. Use 'fuku help' for more information")
}
