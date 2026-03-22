package main

import (
	"fmt"
	"os"

	"charm.land/lipgloss/v2"
	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"

	"fuku/internal/app"
	"fuku/internal/app/cli"
	"fuku/internal/app/errors"
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

	cfg, topology, err := loadConfig(cmd.ConfigFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)

		return 1
	}

	switch cmd.Type {
	case cli.CommandRun, cli.CommandStop:
		if len(cfg.Services) == 0 {
			fmt.Fprintf(os.Stderr, "Error: %v\n", errors.ErrNoServicesDefined)

			return 1
		}
	default:
	}

	if cfg.Telemetry && cfg.SentryDSN == "" {
		cfg.SentryDSN = sentryDSN
	}

	application := createApp(cfg, topology, cmd)
	application.Run()

	return 0
}

// loadConfig wraps config loading for easier testing
func loadConfig(configFile string) (*config.Config, *config.Topology, error) {
	if configFile != "" {
		return config.LoadFromFile(configFile)
	}

	return config.Load()
}

// createAppWithoutConfig creates a lightweight app for standalone commands (init, version, help)
func createAppWithoutConfig(cmd *cli.Options) *cli.CLI {
	return cli.NewCLI(cmd)
}

// createApp creates the FX application with the given config and topology
func createApp(cfg *config.Config, topology *config.Topology, cmd *cli.Options) *fx.App {
	isDark := lipgloss.HasDarkBackground(os.Stdin, os.Stdout)
	log := render.NewLog(isDark)
	writer := render.NewWriter(cfg, log, os.Stdout)

	if cmd.NoUI || cmd.Type == cli.CommandLogs {
		writer.SetEnabled(true)
	}

	return fx.New(
		fx.WithLogger(createFxLogger(cfg)),
		fx.Supply(cfg, topology, log, cmd),
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
