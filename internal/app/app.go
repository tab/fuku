package app

import (
	"context"
	"os"

	"go.uber.org/fx"

	"fuku/internal/app/cli"
)

// App represents the main application container
type App struct {
	cli  cli.CLI
	done chan struct{}
}

// NewApp creates a new application instance with its dependencies
func NewApp(cli cli.CLI) *App {
	return &App{
		cli:  cli,
		done: make(chan struct{}),
	}
}

// Run executes the application
func (a *App) Run() {
	exitCode := a.execute()
	close(a.done)

	os.Exit(exitCode)
}

// execute runs the CLI and returns exit code - extracted for testing
func (a *App) execute() int {
	exitCode, _ := a.cli.Execute()

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
			select {
			case <-app.done:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		},
	})
}
