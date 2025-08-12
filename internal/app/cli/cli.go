package cli

import (
	"fmt"
	"os"

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
	log logger.Logger
}

// NewCLI creates a new cli instance
func NewCLI(
	log logger.Logger,
) CLI {
	return &cli{
		log: log,
	}
}

// Run processes command-line arguments and executes commands
func (c *cli) Run(args []string) error {
	if len(args) == 0 {
		os.Exit(0)
		return nil
	}

	cmd := args[0]

	switch cmd {
	case "run", "--run", "-r":
		c.handleRun()
	case "help", "--help", "-h":
		c.handleHelp()
	case "version", "--version", "-v":
		c.handleVersion()
	default:
		c.handleUnknown()
	}

	os.Exit(0)
	return nil
}

// handleRun executes the run command with the specified scope
func (c *cli) handleRun() {
	c.log.Debug().Msg("Call run command")
	fmt.Println("Run is not implemented yet")
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
