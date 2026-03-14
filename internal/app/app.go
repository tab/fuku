package app

import (
	"context"
	"os"
	"time"

	"go.uber.org/fx"

	"fuku/internal/app/cli"
	"fuku/internal/config/sentry"
)

// App represents the main application container
type App struct {
	ui     cli.TUI
	sentry sentry.Sentry
	done   chan struct{}
}

// NewApp creates a new application instance with its dependencies
func NewApp(ui cli.TUI, s sentry.Sentry) *App {
	return &App{
		ui:     ui,
		sentry: s,
		done:   make(chan struct{}),
	}
}

// Run executes the application
func (a *App) Run() {
	exitCode := a.execute()
	close(a.done)

	a.sentry.Flush()

	os.Exit(exitCode)
}

// execute runs the CLI and returns exit code - extracted for testing
func (a *App) execute() int {
	defer func() {
		if r := recover(); r != nil {
			sentry.CurrentHub().Recover(r)
			sentry.FlushSDK(5 * time.Second)
			panic(r)
		}
	}()

	exitCode, _ := a.ui.Execute()

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
