package bootstrap

import (
	"fmt"
	"os"

	"fuku/internal/app/cli"
)

// Run parses the command line and dispatches the command, returning the process exit code
func Run(args []string, sentryDSN string) int {
	cmd, err := cli.Parse(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)

		return 1
	}

	switch cmd.Type {
	case cli.CommandInit, cli.CommandVersion, cli.CommandHelp:
		return NewStandaloneCLI(cmd).Run()
	case cli.CommandDoctor:
		return NewDoctorCLI(cmd).Run()
	default:
		return NewServiceCLI(cmd, sentryDSN).Run()
	}
}
