package app

import (
	"context"
	"os"

	"go.uber.org/fx"

	"fuku/internal/app/cli"
	"fuku/internal/config/logger"
)

// App represents the main application container
type App struct {
	cli cli.CLI
	log logger.Logger
}

// NewApp creates a new application instance with its dependencies
func NewApp(cli cli.CLI, log logger.Logger) *App {
	return &App{
		cli: cli,
		log: log,
	}
}

// Run executes the application with command line arguments
func (a *App) Run() {
	args := os.Args[1:]
	if err := a.cli.Run(args); err != nil {
		a.log.Error().Err(err).Msg("Application error")
		os.Exit(1)
	}
}

// Register registers the application's lifecycle hooks with fx
func Register(lifecycle fx.Lifecycle, app *App) {
	lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			go app.Run()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			return nil
		},
	})
}
