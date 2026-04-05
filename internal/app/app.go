package app

import (
	"context"
	"time"

	"go.uber.org/fx"

	"fuku/internal/app/cli"
	"fuku/internal/config/sentry"
)

// Root holds the application root context and its cancellation
//
//nolint:containedctx // Root is the designated owner of the app-wide context
type Root struct {
	ctx    context.Context
	cancel context.CancelFunc
}

// NewRoot creates a new application root context
func NewRoot() *Root {
	ctx, cancel := context.WithCancel(context.Background())

	return &Root{ctx: ctx, cancel: cancel}
}

// Context returns the root context
func (r *Root) Context() context.Context {
	return r.ctx
}

// Cancel cancels the root context
func (r *Root) Cancel() {
	r.cancel()
}

// App represents the main application container
type App struct {
	ui         cli.TUI
	sentry     sentry.Sentry
	shutdowner fx.Shutdowner
	done       chan struct{}
}

// NewApp creates a new application instance with its dependencies
func NewApp(ui cli.TUI, sentry sentry.Sentry, shutdowner fx.Shutdowner) *App {
	return &App{
		ui:         ui,
		sentry:     sentry,
		shutdowner: shutdowner,
		done:       make(chan struct{}),
	}
}

// Run executes the application and signals FX to shut down
func (a *App) Run(ctx context.Context) {
	exitCode := a.execute(ctx)
	close(a.done)

	a.sentry.Flush()

	//nolint:errcheck // shutdown is best-effort at exit
	a.shutdowner.Shutdown(fx.ExitCode(exitCode))
}

// execute runs the CLI and returns exit code - extracted for testing
func (a *App) execute(ctx context.Context) int {
	defer func() {
		if r := recover(); r != nil {
			sentry.CurrentHub().Recover(r)
			sentry.FlushSDK(5 * time.Second)
			panic(r)
		}
	}()

	exitCode, _ := a.ui.Execute(ctx)

	return exitCode
}

// Register registers the application's lifecycle hooks with fx
func Register(lifecycle fx.Lifecycle, root *Root, app *App) {
	lifecycle.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			go app.Run(root.Context())

			return nil
		},
		OnStop: func(ctx context.Context) error {
			root.Cancel()

			select {
			case <-app.done:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		},
	})
}
