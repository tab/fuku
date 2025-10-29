package main

import (
	"os"

	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"

	"fuku/internal/app"
	"fuku/internal/config"
	"fuku/internal/config/logger"
)

// main is the entry point for the application
func main() {
	runApp()
}

// runApp contains the main application logic
func runApp() {
	cfg, err := config.Load()
	if err != nil {
		os.Exit(1)
	}

	application := createApp(cfg)
	application.Run()
}

// createApp creates the FX application with the given config
func createApp(cfg *config.Config) *fx.App {
	return fx.New(
		fx.WithLogger(createFxLogger(cfg)),
		fx.Supply(cfg),
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
