package bootstrap

import (
	"fmt"
	"os"

	"fuku/internal/app/cli"
	"fuku/internal/config"
)

// StandaloneCLI handles the commands that run without config or FX container
type StandaloneCLI struct {
	cmd *cli.Options
}

// NewStandaloneCLI creates the CLI for the init, version and help commands
func NewStandaloneCLI(cmd *cli.Options) *StandaloneCLI {
	return &StandaloneCLI{cmd: cmd}
}

// Run executes the standalone command and returns the exit code
func (c *StandaloneCLI) Run() int {
	switch c.cmd.Type {
	case cli.CommandVersion:
		fmt.Printf("Version: %s\n", config.Version)
		return 0
	case cli.CommandHelp:
		fmt.Println(cli.Usage)
		return 0
	case cli.CommandInit:
		exitCode, err := cli.GenerateConfigFile()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}

		return exitCode
	default:
		return 0
	}
}
