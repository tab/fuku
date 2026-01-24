package app

import (
	"context"
	"os"

	"go.uber.org/fx"

	"fuku/internal/app/cli"
)

// App represents the main application container
type App struct {
	cli cli.CLI
}

// NewApp creates a new application instance with its dependencies
func NewApp(cli cli.CLI) *App {
	return &App{
		cli: cli,
	}
}

// Run executes the application
func (a *App) Run() {
	exitCode := a.execute()
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
			return nil
		},
	})
}
