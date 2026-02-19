package main

import (
	"fmt"
	"os"

	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"

	"fuku/internal/app"
	"fuku/internal/app/cli"
	"fuku/internal/app/logs"
	"fuku/internal/config"
	"fuku/internal/config/logger"
)

// main is the entry point for the application
func main() {
	runApp()
}

// runApp contains the main application logic
func runApp() {
	cmd, err := cli.Parse(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	cfg, topology, err := loadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	application := createApp(cfg, topology, cmd)
	application.Run()
}

// loadConfig wraps config.Load for easier testing
func loadConfig() (*config.Config, *config.Topology, error) {
	return config.Load()
}

// createApp creates the FX application with the given config and topology
func createApp(cfg *config.Config, topology *config.Topology, cmd *cli.Options) *fx.App {
	formatter := logs.NewLogFormatter(cfg)

	if cmd.NoUI || cmd.Type == cli.CommandLogs || cmd.Type == cli.CommandStop {
		formatter.SetEnabled(true)
	}

	return fx.New(
		fx.WithLogger(createFxLogger(cfg)),
		fx.Supply(cfg, topology, formatter, cmd),
		fx.Provide(func() logger.Logger {
			return logger.NewLoggerWithOutput(cfg, formatter)
		}),
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
