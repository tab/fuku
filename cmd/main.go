package main

import (
	"io"
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
	cfg, err := loadConfig()
	if err != nil {
		os.Exit(1)
	}

	noUI := hasNoUIFlag(os.Args[1:])
	application := createApp(cfg, noUI)
	application.Run()
}

// hasNoUIFlag checks if --no-ui flag is present in args
func hasNoUIFlag(args []string) bool {
	for _, arg := range args {
		if arg == "--no-ui" {
			return true
		}
	}

	return false
}

// loadConfig wraps config.Load for easier testing
func loadConfig() (*config.Config, error) {
	return config.Load()
}

// createApp creates the FX application with the given config
func createApp(cfg *config.Config, noUI bool) *fx.App {
	var logOutput io.Writer
	if !noUI {
		logOutput = io.Discard
	}

	return fx.New(
		fx.WithLogger(createFxLogger(cfg)),
		fx.Supply(cfg),
		fx.Provide(func() logger.Logger {
			return logger.NewLoggerWithOutput(cfg, logOutput)
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
