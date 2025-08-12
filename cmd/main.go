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
	cfg, err := config.Load()
	if err != nil {
		os.Exit(1)
	}

	fx.New(
		fx.WithLogger(
			func() fxevent.Logger {
				if cfg.Logging.Level == logger.DebugLevel {
					return &fxevent.ConsoleLogger{W: os.Stdout}
				}
				return fxevent.NopLogger
			},
		),
		fx.Supply(cfg),
		app.Module,
	).Run()
}
