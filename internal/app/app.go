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
	cli        cli.CLI
	log        logger.Logger
	shutdowner fx.Shutdowner
	exitCode   int
}

// NewApp creates a new application instance with its dependencies
func NewApp(cli cli.CLI, log logger.Logger, shutdowner fx.Shutdowner) *App {
	return &App{
		cli:        cli,
		log:        log,
		shutdowner: shutdowner,
	}
}

// Run executes the application with command line arguments and shuts down the FX app
func (a *App) Run() {
	args := os.Args[1:]
	a.exitCode = a.execute(args)

	if a.exitCode != 0 {
		a.log.Debug().Int("exit_code", a.exitCode).Msg("Application finished with non-zero exit code")
	}

	if err := a.shutdowner.Shutdown(); err != nil {
		a.log.Error().Err(err).Msg("Failed to shutdown FX application")
	}
}

// ExitCode returns the exit code from the last Run execution
func (a *App) ExitCode() int {
	return a.exitCode
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
			go app.Run()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			return nil
		},
	})
}
