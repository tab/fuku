package bootstrap

import (
	"fmt"
	"os"

	"go.uber.org/dig"
	"go.uber.org/fx"

	"fuku/internal/app/cli"
	"fuku/internal/app/instance"
)

// DoctorCLI handles the doctor command, which needs the project directory and the instance identity but no config
type DoctorCLI struct {
	cmd *cli.Options
}

// NewDoctorCLI creates the CLI for the doctor command
func NewDoctorCLI(cmd *cli.Options) *DoctorCLI {
	return &DoctorCLI{cmd: cmd}
}

// Run resolves the instance identity inside a small FX container, writes the doctor report and returns the exit code
func (c *DoctorCLI) Run() int {
	if err := cli.ChangeToConfigDir(c.cmd); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)

		return 1
	}

	exitCode := 0

	application := fx.New(
		fx.NopLogger,
		fx.Provide(instance.NewInstance),
		fx.Invoke(func(identity instance.Identity) {
			exitCode = cli.RunDoctor(c.cmd, identity)
		}),
	)

	if err := application.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", dig.RootCause(err))

		return 1
	}

	return exitCode
}
