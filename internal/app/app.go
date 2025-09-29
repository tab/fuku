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
	shutdowner fx.Shutdowner
	errChan    chan error
	log        logger.Logger
}

// NewApp creates a new application instance with its dependencies
func NewApp(cli cli.CLI, shutdowner fx.Shutdowner, log logger.Logger) *App {
	return &App{
		cli:        cli,
		shutdowner: shutdowner,
		errChan:    make(chan error, 1),
		log:        log,
	}
}

// Run executes the application with command line arguments and shuts down the FX app
func (a *App) Run() {
	args := os.Args[1:]
	err := a.execute(args)

	if err != nil {
		a.log.Debug().Err(err).Msg("Application finished with error")
	}

	if shutdownErr := a.shutdowner.Shutdown(); shutdownErr != nil {
		a.log.Error().Err(shutdownErr).Msg("Failed to shutdown FX application")
	}

	a.errChan <- err
}

// Wait waits for the application to complete and returns the error result
func (a *App) Wait() error {
	return <-a.errChan
}

// execute runs the CLI with given args and handles errors - extracted for testing
func (a *App) execute(args []string) error {
	err := a.cli.Run(args)
	if err != nil {
		a.log.Error().Err(err).Msg("Application error")
	}
	return err
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
