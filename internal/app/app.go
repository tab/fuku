package app

import (
	"context"
	"os"

	"go.uber.org/fx"

	"fuku/internal/app/cli"
	"fuku/internal/app/runtime"
	"fuku/internal/config/logger"
)

// App represents the main application container
type App struct {
	cli       cli.CLI
	logWriter runtime.LogWriter
	log       logger.Logger
}

// NewApp creates a new application instance with its dependencies
func NewApp(cli cli.CLI, logWriter runtime.LogWriter, log logger.Logger) *App {
	return &App{
		cli:       cli,
		logWriter: logWriter,
		log:       log,
	}
}

// Run executes the application with command line arguments
func (a *App) Run() {
	args := os.Args[1:]
	exitCode := a.execute(args)
	os.Exit(exitCode)
}

// execute runs the CLI with given args and handles errors - extracted for testing
func (a *App) execute(args []string) int {
	exitCode, err := a.cli.Run(args)
	if err != nil {
		a.log.Error().Err(err).Msg("Application error")
	}

	return exitCode
}

// Register registers the application's lifecycle hooks with fx
func Register(lifecycle fx.Lifecycle, app *App) {
	lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			app.logWriter.Start(ctx)

			go app.Run()

			return nil
		},
		OnStop: func(ctx context.Context) error {
			return app.logWriter.Close()
		},
	})
}
