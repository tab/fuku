package modules

import (
	"io"
	"log/slog"
	"net/http"

	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"

	"fuku/internal/adapters/cli"
	"fuku/internal/adapters/eventlog"
	"fuku/internal/adapters/github"
	"fuku/internal/adapters/instance"
	"fuku/internal/adapters/logsocket"
	"fuku/internal/adapters/output"
	"fuku/internal/adapters/process"
	"fuku/internal/adapters/readiness"
	"fuku/internal/adapters/resources"
	"fuku/internal/adapters/telemetry"
	"fuku/internal/adapters/terminal"
	"fuku/internal/adapters/watch"
	"fuku/internal/app/logs"
	"fuku/internal/app/profiles"
	"fuku/internal/app/registry"
	"fuku/internal/app/services"
	"fuku/internal/app/updater"
	"fuku/internal/bootstrap/lifecycle"
	"fuku/internal/model"
	"fuku/internal/platform/buildinfo"
	"fuku/internal/platform/logging"
	"fuku/internal/platform/worker"
)

// configured is what the commands of a loaded project share: the project, the command, the logger and the observers
func configured(cmd *cli.Options, project model.Project) fx.Option {
	return fx.Options(
		fx.WithLogger(fxLogger(project)),
		fx.Supply(cmd, project),
		fx.Provide(
			newLogger,
			fx.Annotate(newWriter, fx.ParamTags(``, ``, `name:"stdout"`)),
			func(p model.Project) model.Telemetry { return p.Telemetry },
			func(log *slog.Logger) eventlog.Logger { return log.With("component", "BUS") },
			func(log *slog.Logger) logs.Logger { return log.With("component", "LOGS") },
			func(p model.Project) logs.Options {
				return logs.Options{Buffer: p.Logs.Buffer, History: p.Logs.History}
			},
			func(p model.Project) output.Options { return output.Options{Format: p.Logging.Format} },
			func(p model.Project) terminal.Options { return terminal.Options{Format: p.Logging.Format} },
			func(h *logs.Hub) eventlog.Broadcaster { return h },
		),
		cli.Module,
		eventlog.Module,
		logs.Module,
		terminal.Module,
	)
}

// telemetryParams are what every command runs: the command, the announcer and, with telemetry, the collector and tracer
type telemetryParams struct {
	fx.In

	Telemetry telemetry.Options
	Collector *telemetry.Collector
	Tracer    *telemetry.Tracer
	Announcer *cli.Announcer
	Command   lifecycle.Command
}

// participants seeds the participants of a command
func (p telemetryParams) participants() lifecycle.Participants {
	participants := lifecycle.Participants{
		Producers: []lifecycle.Producer{p.Announcer},
		Command:   p.Command,
	}

	if p.Telemetry.Enabled {
		participants.Consumers = []lifecycle.Consumer{p.Collector, p.Tracer}
	}

	return participants
}

// observerParams are the participants of every configured command: the event log ahead of the telemetry participants
type observerParams struct {
	fx.In

	Base     telemetryParams
	Recorder *eventlog.Recorder
}

// participants seeds the participants of a configured command
func (p observerParams) participants() lifecycle.Participants {
	participants := p.Base.participants()
	participants.Consumers = append([]lifecycle.Consumer{p.Recorder}, participants.Consumers...)

	return participants
}

// processes reaches the children of a profile: resolution, the process adapter, the worker pool, the services package
var processes = fx.Options(
	fx.Provide(
		func(log *slog.Logger) process.Logger { return log.With("component", "PROCESS") },
		func(log *slog.Logger) services.Logger { return log.With("component", "SERVICES") },
		func(p model.Project) worker.Options { return worker.Options{Workers: p.Concurrency.Workers} },
		func(p *process.Preflight) services.Preflight { return p },
		func(p *worker.Pool) process.Pool { return p },
		func(r *profiles.Resolver) services.ProfileResolver { return r },
		func(h *logs.Hub) process.LogSink { return h },
	),
	process.Module,
	profiles.Module,
	services.Module,
	worker.Module,
)

// profile is what runs a profile: the processes, the services runtime with its guard and control, readiness and workers
var profile = fx.Options(
	processes,
	fx.Provide(
		func(log *slog.Logger) readiness.Logger { return log.With("component", "READINESS") },
		func(cmd *cli.Options, p model.Project) services.Options {
			return services.Options{Profile: cmd.Profile, RetryAttempts: p.Retry.Attempts, RetryBackoff: p.Retry.Backoff}
		},
		func(a *lifecycle.Arbiter) services.Reporter { return a },
		func(f *process.Factory) services.Launcher { return f },
		func(t *process.Tracker) services.Tracker { return t },
		func(c *readiness.Checker) services.Readiness { return c },
		func(p *worker.Pool) services.Pool { return p },
	),
	readiness.Module,
)

// runtime is the shared runtime of a run: profile, registry, watcher, sampler, socket server, update check
var runtime = fx.Options(
	profile,
	fx.Provide(
		func(log *slog.Logger) github.Logger { return log.With("component", "UPDATER") },
		func(log *slog.Logger) logsocket.Logger { return log.With("component", "SERVER") },
		func(log *slog.Logger) updater.Logger { return log.With("component", "UPDATER") },
		func(log *slog.Logger) watch.Logger { return log.With("component", "WATCHER") },
		func(p model.Project) updater.Options {
			return updater.Options{Enabled: p.Updater.Enabled, Version: buildinfo.Version}
		},
		func(t telemetry.Options) resources.Options { return resources.Options{Enabled: t.Enabled} },
		func() github.Options {
			return github.Options{CachePath: github.DefaultCachePath()}
		},
		func(c *http.Client) github.HTTPDoer { return c },
		func(c *github.Client) updater.ReleaseSource { return c },
		func(h *logs.Hub) logsocket.Hub { return h },
		func(s *registry.Store) logsocket.Registry { return s },
		func(c *services.Control) logsocket.Control { return c },
		func(s *registry.Store) resources.Registry { return s },
		fx.Annotate(
			func(m *resources.ProcessMonitor) resources.Monitor { return m },
			fx.ParamTags(`name:"sampler"`),
		),
	),
	github.Module,
	instance.Module,
	logsocket.Module,
	registry.Module,
	resources.Module,
	updater.Module,
	watch.Module,
)

// newLogger creates the application logger that writes through the log writer
func newLogger(project model.Project, writer *output.Writer) *slog.Logger {
	return slog.New(logging.NewHandler(logging.Options{Level: project.Logging.Level, Version: buildinfo.Version}, writer))
}

// newWriter creates the application log writer, enabled up front
func newWriter(options output.Options, log *terminal.Log, stdout io.Writer) *output.Writer {
	writer := output.NewWriter(options, log, stdout)
	writer.SetEnabled(true)

	return writer
}

// newTelemetryOptions projects the telemetry config, falling back to the DSN built into the binary
func newTelemetryOptions(cfg model.Telemetry, sentryDSN SentryDSN) telemetry.Options {
	dsn := cfg.DSN
	if dsn == "" {
		dsn = string(sentryDSN)
	}

	return telemetry.Options{
		Enabled:     cfg.Enabled && dsn != "",
		DSN:         dsn,
		Environment: cfg.Environment,
	}
}

// fxLogger returns the Fx event logger: the application logger at debug level, silence otherwise
func fxLogger(project model.Project) func(*slog.Logger) fxevent.Logger {
	return func(log *slog.Logger) fxevent.Logger {
		if project.Logging.Level != logging.LevelDebug {
			return fxevent.NopLogger
		}

		logger := &fxevent.SlogLogger{Logger: log.With("component", "FX")}
		logger.UseLogLevel(slog.LevelDebug)

		return logger
	}
}
