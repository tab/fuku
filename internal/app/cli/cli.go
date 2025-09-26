package cli

import (
	"context"
	"fmt"
	"strings"

	"fuku/internal/app/runner"
	"fuku/internal/config"
	"fuku/internal/config/logger"
)

const (
	Usage = `Usage:
  fuku --run=<PROFILE>            Run the fuku with the specified profile
  fuku help                       Show help
  fuku version                    Show version

Examples:
  fuku --run=default              Run all services
  fuku --run=core                 Run core services
  fuku --run=minimal              Run minimal services`
)

// CLI defines the interface for cli operations
type CLI interface {
	// Run processes command-line arguments and returns an exit code and error
	Run(args []string) (exitCode int, err error)
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
func (c *cli) Run(args []string) (int, error) {
	if len(args) == 0 {
		return c.handleRun(config.DefaultProfile)
	}

	cmd := args[0]

	switch {
	case cmd == "help" || cmd == "--help" || cmd == "-h":
		return c.handleHelp()
	case cmd == "version" || cmd == "--version" || cmd == "-v":
		return c.handleVersion()
	case cmd == "run" || cmd == "--run" || cmd == "-r":
		profile := config.DefaultProfile
		if len(args) > 1 {
			profile = args[1]
		}
		return c.handleRun(profile)
	case strings.HasPrefix(cmd, "--run="):
		profile := strings.TrimPrefix(cmd, "--run=")
		if profile == "" {
			profile = config.DefaultProfile
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
	if err := c.runner.Run(ctx, profile); err != nil {
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
