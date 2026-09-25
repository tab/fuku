package modules

import (
	"io"
	"log/slog"
	"os"
	"sync"

	"go.uber.org/fx"

	"fuku/internal/adapters/cli"
	"fuku/internal/adapters/telemetry"
	"fuku/internal/adapters/terminal"
	"fuku/internal/bootstrap/lifecycle"
	"fuku/internal/contracts"
	"fuku/internal/model"
	"fuku/internal/platform/bus"
)

// SentryDSN is the build-time DSN used when the environment supplies none
type SentryDSN string

// base is in every composition: the bus, the coordinator and its arbiter, telemetry, the theme, the process streams
var base = fx.Options(
	fx.Supply(bus.Options{QueueDepth: bus.QueueDepthDefault}),
	fx.Provide(
		newTelemetryOptions,
		newTheme,
		lifecycle.NewArbiter,
		lifecycle.NewCoordinator,
		fx.Annotate(newStdout, fx.ResultTags(`name:"stdout"`)),
		fx.Annotate(newStderr, fx.ResultTags(`name:"stderr"`)),
		func(a *lifecycle.Arbiter) bus.FailureReporter { return a },
		func(b *bus.Bus) contracts.Publisher { return b },
		func(b *bus.Bus) contracts.Subscriber { return b },
		func(b *bus.Bus) lifecycle.Closer { return b },
		func(c *telemetry.Client) lifecycle.Telemetry { return c },
		func(log *slog.Logger) bus.Logger { return log.With("component", "BUS") },
		func(log *slog.Logger) lifecycle.Logger { return log.With("component", "APP") },
	),
	bus.Module,
	telemetry.Module,
	fx.Invoke(lifecycle.Register),
)

// standalone serves the commands that load no project: the command, telemetry from the environment, no log, a quiet Fx
func standalone(cmd *cli.Options, telemetry model.Telemetry) fx.Option {
	return fx.Options(
		fx.NopLogger,
		fx.Supply(cmd, telemetry),
		fx.Provide(newDiscardLogger, telemetryParams.participants),
		cli.Module,
	)
}

// newTheme returns the theme for the terminal's background, detected once on the first call
func newTheme() func() terminal.Theme {
	return sync.OnceValue(func() terminal.Theme {
		return terminal.NewTheme(terminal.AppearanceSystem.Resolve(os.Stdin, os.Stdout))
	})
}

// newDiscardLogger creates the logger of a composition that has no log output
func newDiscardLogger() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}

// newStdout exposes the process stdout for the commands that write their output directly
func newStdout() io.Writer {
	return os.Stdout
}

// newStderr exposes the process stderr for the components that write before the logger exists
func newStderr() io.Writer {
	return os.Stderr
}
