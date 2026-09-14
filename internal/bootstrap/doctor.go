package bootstrap

import (
	"fmt"
	"os"

	"fuku/internal/app/cli"
)

// DoctorCLI handles the doctor command, which needs the project directory but no config
type DoctorCLI struct {
	cmd *cli.Options
}

// NewDoctorCLI creates the CLI for the doctor command
func NewDoctorCLI(cmd *cli.Options) *DoctorCLI {
	return &DoctorCLI{cmd: cmd}
}

// Run writes the doctor report and returns the exit code
func (c *DoctorCLI) Run() int {
	if err := cli.ChangeToConfigDir(c.cmd); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)

		return 1
	}

	return cli.RunDoctor(c.cmd)
}
