package main

import (
	"context"
	"fmt"
	"os"

	"charm.land/lipgloss/v2"
	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"

	"fuku/internal/app"
	"fuku/internal/app/cli"
	"fuku/internal/app/errors"
	"fuku/internal/app/instance"
	"fuku/internal/app/render"
	"fuku/internal/config"
	"fuku/internal/config/logger"
	"fuku/internal/config/sentry"
)

var sentryDSN string

// main is the entry point for the application
func main() {
	os.Exit(runApp())
}

// runApp contains the main application logic
func runApp() (exitCode int) {
	cmd, err := cli.Parse(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)

		return 1
	}

	if cmd.Type.Standalone() {
		return createAppWithoutConfig(cmd).Run()
	}

	if err := cli.ChangeToConfigDir(cmd); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)

		return 1
	}

	if cmd.Type == cli.CommandDoctor {
		return cli.RunDoctor(cmd)
	}

	cfg, topology, err := config.LoadPath(cmd.ConfigFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)

		return 1
	}

	if cmd.Type.RequiresServices() && len(cfg.Services) == 0 {
		fmt.Fprintf(os.Stderr, "Error: %v\n", errors.ErrNoServicesDefined)

		return 1
	}

	if cfg.Telemetry && cfg.SentryDSN == "" {
		cfg.SentryDSN = sentryDSN
	}

	identity, err := instance.New()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)

		return 1
	}

	if code := refuseSecondInstance(cmd, cfg, identity); code != 0 {
		return code
	}

	application := createApp(cfg, topology, identity, cmd)
	application.Run()

	return 0
}

// refuseSecondInstance stops a run that would preflight-kill the services of an instance already
// serving this project (detection needs the API, so an instance without one is not found)
func refuseSecondInstance(cmd *cli.Options, cfg *config.Config, identity instance.Identity) int {
	if cmd.Type != cli.CommandRun {
		return 0
	}

	address, found := instance.Running(context.Background(), cfg.ServerListen(), identity.Fingerprint)
	if !found {
		return 0
	}

	fmt.Fprintf(os.Stderr, "Error: %v (API on %s)\n", errors.ErrInstanceAlreadyRunning, address)
	fmt.Fprintf(os.Stderr, "Read its output with 'fuku logs', or stop it before starting another instance\n")

	return 1
}

// createAppWithoutConfig creates a lightweight app for standalone commands (init, version, help)
func createAppWithoutConfig(cmd *cli.Options) *cli.CLI {
	return cli.NewCLI(cmd)
}

// createApp creates the FX application with the given config, topology and instance identity
func createApp(cfg *config.Config, topology *config.Topology, identity instance.Identity, cmd *cli.Options) *fx.App {
	isDark := lipgloss.HasDarkBackground(os.Stdin, os.Stdout)
	log := render.NewLog(isDark)
	writer := render.NewWriter(cfg, log, os.Stdout)

	if cmd.NoUI || cmd.Type == cli.CommandLogs {
		writer.SetEnabled(true)
	}

	return fx.New(
		fx.WithLogger(createFxLogger(cfg)),
		fx.Supply(cfg, topology, identity, log, cmd, writer),
		fx.Provide(func() logger.Logger {
			return logger.NewLoggerWithOutput(cfg, writer)
		}),
		fx.Provide(logger.NewEventLogger),
		sentry.Module,
		app.Module,
	)
}

// createFxLogger returns an FX logger based on the config
func createFxLogger(cfg *config.Config) func() fxevent.Logger {
	return func() fxevent.Logger {
		if cfg.Logging.Level == logger.DebugLevel {
			return &fxevent.ConsoleLogger{W: os.Stdout}
		}

		return fxevent.NopLogger
	}
}
