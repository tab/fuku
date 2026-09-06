package app

import (
	"context"
	"time"

	"go.uber.org/fx"

	"fuku/internal/app/api"
	"fuku/internal/app/bus"
	"fuku/internal/app/cli"
	"fuku/internal/app/instance"
	"fuku/internal/config"
	"fuku/internal/config/logger"
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
	bus        bus.Bus
	shutdown   *Shutdown
	log        logger.Logger
	done       chan struct{}
}

// NewApp creates a new application instance with its dependencies
func NewApp(ui cli.TUI, sentry sentry.Sentry, shutdowner fx.Shutdowner, b bus.Bus, shutdown *Shutdown, log logger.Logger) *App {
	return &App{
		ui:         ui,
		sentry:     sentry,
		shutdowner: shutdowner,
		bus:        b,
		shutdown:   shutdown,
		log:        log.WithComponent("APP"),
		done:       make(chan struct{}),
	}
}

// Run executes the application and signals FX to shut down
func (a *App) Run(ctx context.Context) {
	exitCode := a.execute(ctx)

	a.sentry.Flush()
	a.shutdown.Self()
	close(a.done)

	//nolint:errcheck // shutdown is best-effort at exit
	a.shutdowner.Shutdown(fx.ExitCode(exitCode))
}

// PublishSignal announces the OS signal that initiated the shutdown on the bus
func (a *App) PublishSignal() {
	sig := a.shutdown.Signal()
	if sig == nil {
		return
	}

	a.log.Info().Msgf("Received signal %s, shutting down services...", sig)

	a.bus.Publish(bus.Message{
		Type:     bus.EventSignal,
		Data:     bus.Signal{Name: sig.String()},
		Critical: true,
	})
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
			app.PublishSignal()
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

// RegisterGuard registers the instance guard lifecycle hook when server config is present
func RegisterGuard(lc fx.Lifecycle, cmd *cli.Options, cfg *config.Config, guard instance.Guard) {
	if cmd.Type != cli.CommandRun {
		return
	}

	if cfg.Server.Listen == "" {
		return
	}

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			return guard.Check(ctx)
		},
	})
}

// RegisterAPI registers the API server lifecycle hooks when server config is present
func RegisterAPI(lc fx.Lifecycle, cmd *cli.Options, cfg *config.Config, server *api.Server) {
	if cmd.Type != cli.CommandRun {
		return
	}

	if cfg.Server.Listen == "" {
		return
	}

	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			server.Start()

			return nil
		},
		OnStop: func(ctx context.Context) error {
			server.Shutdown(ctx)

			return nil
		},
	})
}
