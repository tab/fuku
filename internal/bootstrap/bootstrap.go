package bootstrap

import (
	"fmt"
	"os"

	"go.uber.org/fx"

	"fuku/internal/adapters/cli"
	"fuku/internal/adapters/config"
	"fuku/internal/bootstrap/lifecycle"
	"fuku/internal/bootstrap/modules"
	"fuku/internal/contracts"
	"fuku/internal/model"
)

// Run parses the command line, composes the container of the command and runs it, returning the process exit code
func Run(args []string, sentryDSN string) int {
	cmd, err := cli.Parse(args)
	if err != nil {
		return refuse(err)
	}

	if err := cli.ChangeToConfigDir(cmd); err != nil {
		return refuse(err)
	}

	option, err := compose(cmd)
	if err != nil {
		return refuse(err)
	}

	var arbiter *lifecycle.Arbiter

	app := fx.New(option, fx.Supply(modules.SentryDSN(sentryDSN)), fx.Populate(&arbiter))

	return lifecycle.Run(app, arbiter, os.Stderr)
}

// compose selects the composition of the command
func compose(cmd *cli.Options) (fx.Option, error) {
	switch cmd.Type {
	case cli.CommandHelp:
		return modules.Help(cmd), nil
	case cli.CommandVersion:
		return modules.Version(cmd), nil
	case cli.CommandInit:
		return modules.Init(cmd), nil
	case cli.CommandDoctor:
		return modules.Doctor(cmd, config.LoadPath(cmd.ConfigFile)), nil
	case cli.CommandStop:
		return load(cmd, modules.Stop)
	case cli.CommandLogs:
		return load(cmd, modules.Logs)
	default:
		return load(cmd, modules.Run)
	}
}

// load composes a command over the loaded project, refusing an invalid config and a run or stop without services
func load(cmd *cli.Options, compose func(*cli.Options, model.Project) fx.Option) (fx.Option, error) {
	loaded := config.LoadPath(cmd.ConfigFile)
	if loaded.Error != nil {
		return nil, loaded.Error
	}

	if cmd.Type.RequiresServices() && len(loaded.Project.Services) == 0 {
		return nil, contracts.ErrNoServicesDefined
	}

	return compose(cmd, loaded.Project), nil
}

// refuse prints the error that keeps the command from running and returns its exit code
func refuse(err error) int {
	fmt.Fprintf(os.Stderr, "Error: %v\n", err)

	return 1
}
