package modules

import (
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"

	"fuku/internal/adapters/cli"
	"fuku/internal/adapters/eventlog"
	"fuku/internal/adapters/github"
	"fuku/internal/adapters/instance"
	"fuku/internal/adapters/logsocket"
	"fuku/internal/adapters/output"
	"fuku/internal/adapters/process"
	"fuku/internal/adapters/readiness"
	"fuku/internal/adapters/resources"
	"fuku/internal/adapters/rest"
	"fuku/internal/adapters/telemetry"
	"fuku/internal/adapters/terminal"
	"fuku/internal/adapters/tui"
	"fuku/internal/adapters/watch"
	"fuku/internal/app/doctor"
	"fuku/internal/app/logs"
	"fuku/internal/app/registry"
	"fuku/internal/app/services"
	"fuku/internal/app/updater"
	"fuku/internal/bootstrap/lifecycle"
	"fuku/internal/model"
	"fuku/internal/platform/buildinfo"
	"fuku/internal/platform/bus"
	"fuku/internal/platform/logging"
	"fuku/internal/platform/worker"
)

func Test_Compositions(t *testing.T) {
	project := model.Project{
		Logging:     model.Logging{Level: logging.LevelInfo, Format: logging.FormatConsole},
		Concurrency: model.Concurrency{Workers: 5},
	}
	served := project
	served.Server.Listen = "127.0.0.1:9876"

	noProject := func(model.Project) {}
	noRuntime := func(*services.Runtime) {}
	noRegistry := func(*registry.Store) {}
	noView := func(*tui.Program) {}
	noHeadless := func(*cli.Run) {}
	noServer := func(*rest.Server) {}

	tests := []struct {
		name   string
		option fx.Option
		absent any
	}{
		{
			name:   "help loads no project",
			option: Help(&cli.Options{Type: cli.CommandHelp}, model.Telemetry{}),
			absent: noProject,
		},
		{
			name:   "version loads no project",
			option: Version(&cli.Options{Type: cli.CommandVersion}, model.Telemetry{}),
			absent: noProject,
		},
		{
			name:   "init loads no project",
			option: Init(&cli.Options{Type: cli.CommandInit}, model.Telemetry{}),
			absent: noProject,
		},
		{
			name:   "doctor runs no services",
			option: Doctor(&cli.Options{Type: cli.CommandDoctor, DoctorFormat: cli.FormatSummary}, model.Config{}),
			absent: noRuntime,
		},
		{
			name:   "stop projects no registry",
			option: Stop(&cli.Options{Type: cli.CommandStop, Profile: model.ProfileDefault}, project),
			absent: noRegistry,
		},
		{
			name:   "stop runs no services",
			option: Stop(&cli.Options{Type: cli.CommandStop, Profile: model.ProfileDefault}, project),
			absent: noRuntime,
		},
		{
			name:   "logs runs no services",
			option: Logs(&cli.Options{Type: cli.CommandLogs}, project),
			absent: noRuntime,
		},
		{
			name:   "run with the TUI has no headless command",
			option: Run(&cli.Options{Type: cli.CommandRun, Profile: model.ProfileDefault}, served),
			absent: noHeadless,
		},
		{
			name:   "headless run has no view",
			option: Run(&cli.Options{Type: cli.CommandRun, Profile: model.ProfileDefault, NoUI: true}, project),
			absent: noView,
		},
		{
			name:   "run without a listen address resolves no *rest.Server",
			option: Run(&cli.Options{Type: cli.CommandRun, Profile: model.ProfileDefault}, project),
			absent: noServer,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := fx.ValidateApp(tt.option, fx.Supply(SentryDSN("")))

			require.NoError(t, err)
			assert.Error(t, fx.ValidateApp(tt.option, fx.Supply(SentryDSN("")), fx.Invoke(tt.absent)))
		})
	}
}

// componentHandler records the component a bound logger carries
type componentHandler struct {
	slog.Handler
	component string
}

// WithAttrs keeps the last component attribute a binding added
func (h componentHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	for _, attr := range attrs {
		if attr.Key == "component" {
			h.component = attr.Value.String()
		}
	}

	return h
}

func Test_Projections(t *testing.T) {
	type options struct {
		fx.In

		Bus       bus.Options
		Telemetry telemetry.Options
		Logs      logs.Options      `optional:"true"`
		Output    output.Options    `optional:"true"`
		Terminal  terminal.Options  `optional:"true"`
		Worker    worker.Options    `optional:"true"`
		Services  services.Options  `optional:"true"`
		Updater   updater.Options   `optional:"true"`
		Resources resources.Options `optional:"true"`
		GitHub    github.Options    `optional:"true"`
		REST      rest.Options      `optional:"true"`
		TUI       tui.Options       `optional:"true"`
		Request   logs.Request      `optional:"true"`
		Doctor    doctor.Options    `optional:"true"`
	}

	type loggers struct {
		fx.In

		Bus       bus.Logger
		Lifecycle lifecycle.Logger
		EventLog  eventlog.Logger  `optional:"true"`
		Logs      logs.Logger      `optional:"true"`
		Process   process.Logger   `optional:"true"`
		Services  services.Logger  `optional:"true"`
		Readiness readiness.Logger `optional:"true"`
		Socket    logsocket.Logger `optional:"true"`
		Watch     watch.Logger     `optional:"true"`
		REST      rest.Logger      `optional:"true"`
		UI        tui.Logger       `optional:"true"`
		Updater   updater.Logger   `optional:"true"`
		GitHub    github.Logger    `optional:"true"`
	}

	project := model.Project{
		Logging:     model.Logging{Level: logging.LevelInfo, Format: logging.FormatJSON},
		Concurrency: model.Concurrency{Workers: 5},
		Retry:       model.Retry{Attempts: 3, Backoff: 2 * time.Second},
		Logs:        model.Logs{Buffer: 64, History: 128},
		Server:      model.Server{Listen: "127.0.0.1:9876", Token: "secret"},
		Telemetry:   model.Telemetry{DSN: "https://env@sentry.io/1", Environment: "test"},
		Updater:     model.Updater{Enabled: true},
	}
	buildDSN := SentryDSN("https://build@sentry.io/2")
	buildTelemetry := telemetry.Options{Enabled: true, DSN: string(buildDSN), Environment: "test"}
	buildProject := project
	buildProject.Telemetry = model.Telemetry{Enabled: true, Environment: "test"}
	profile := "web"
	tail := 10
	replay := model.ReplayOptions{Tail: &tail, NoFollow: true}
	wd, err := os.Getwd()
	require.NoError(t, err)

	dir, err := filepath.EvalSymlinks(wd)
	require.NoError(t, err)

	probe := fx.Options(
		fx.Supply(buildDSN),
		fx.Replace(slog.New(componentHandler{Handler: slog.DiscardHandler})),
	)
	components := func(l loggers) map[string]string {
		bound := map[string]any{
			"bus": l.Bus, "lifecycle": l.Lifecycle, "eventlog": l.EventLog, "logs": l.Logs,
			"process": l.Process, "services": l.Services, "readiness": l.Readiness, "socket": l.Socket,
			"watch": l.Watch, "rest": l.REST, "ui": l.UI, "updater": l.Updater, "github": l.GitHub,
		}
		result := map[string]string{}

		for name, log := range bound {
			logger, exists := log.(*slog.Logger)
			if !exists {
				continue
			}

			result[name] = logger.Handler().(componentHandler).component
		}

		return result
	}

	shared := options{
		Bus:       bus.Options{QueueDepth: bus.QueueDepthDefault},
		Telemetry: telemetry.Options{DSN: "https://env@sentry.io/1", Environment: "test"},
		Logs:      logs.Options{Buffer: 64, History: 128},
		Output:    output.Options{Format: logging.FormatJSON},
		Terminal:  terminal.Options{Format: logging.FormatJSON},
	}
	processed := shared
	processed.Worker = worker.Options{Workers: 5}
	headless := processed
	headless.Services = services.Options{Profile: profile, RetryAttempts: 3, RetryBackoff: 2 * time.Second}
	headless.Updater = updater.Options{Enabled: true, Version: buildinfo.Version}
	headless.GitHub = github.Options{CachePath: github.DefaultCachePath()}
	headless.REST = rest.Options{Listen: "127.0.0.1:9876", Token: "secret"}
	buildHeadless := headless
	buildHeadless.Telemetry = buildTelemetry
	buildHeadless.Resources = resources.Options{Enabled: true}
	view := headless
	view.TUI = tui.Options{Profile: profile, RetryAttempts: 3, RetryBackoff: 2 * time.Second}
	stream := shared
	stream.Request = logs.Request{Profile: profile, Services: []string{"api"}, ReplayOptions: replay}
	bare := options{Bus: shared.Bus, Telemetry: buildTelemetry}
	diagnosed := bare
	diagnosed.Doctor = doctor.Options{Profile: profile, ExplicitConfig: true, Fingerprint: instance.Fingerprint(dir), Version: buildinfo.Version}

	minimal := map[string]string{"bus": "BUS", "lifecycle": "APP"}
	base := maps.Clone(minimal)
	maps.Copy(base, map[string]string{"eventlog": "BUS", "logs": "LOGS"})
	profiled := maps.Clone(base)
	maps.Copy(profiled, map[string]string{
		"process": "PROCESS", "services": "SERVICES", "readiness": "READINESS", "socket": "SERVER", "watch": "WATCHER", "rest": "API",
		"updater": "UPDATER", "github": "UPDATER",
	})
	viewed := maps.Clone(profiled)
	viewed["ui"] = "UI"
	stopped := maps.Clone(base)
	maps.Copy(stopped, map[string]string{"process": "PROCESS", "services": "SERVICES"})

	tests := []struct {
		name       string
		option     fx.Option
		options    options
		components map[string]string
	}{
		{
			name:       "headless run with the API",
			option:     Run(&cli.Options{Type: cli.CommandRun, Profile: profile, NoUI: true}, project),
			options:    headless,
			components: profiled,
		},
		{
			name:       "headless run uses the build DSN without an environment DSN",
			option:     Run(&cli.Options{Type: cli.CommandRun, Profile: profile, NoUI: true}, buildProject),
			options:    buildHeadless,
			components: profiled,
		},
		{
			name:       "run under the view",
			option:     Run(&cli.Options{Type: cli.CommandRun, Profile: profile}, project),
			options:    view,
			components: viewed,
		},
		{
			name:       "stop",
			option:     Stop(&cli.Options{Type: cli.CommandStop, Profile: profile}, project),
			options:    processed,
			components: stopped,
		},
		{
			name:       "logs",
			option:     Logs(&cli.Options{Type: cli.CommandLogs, Profile: profile, Services: []string{"api"}, ReplayOptions: replay}, project),
			options:    stream,
			components: base,
		},
		{
			name:       "doctor",
			option:     Doctor(&cli.Options{Type: cli.CommandDoctor, Profile: profile, ConfigFile: "fuku.yaml"}, model.Config{Project: buildProject}),
			options:    diagnosed,
			components: minimal,
		},
		{
			name:       "help",
			option:     Help(&cli.Options{Type: cli.CommandHelp}, buildProject.Telemetry),
			options:    bare,
			components: minimal,
		},
		{
			name:       "version",
			option:     Version(&cli.Options{Type: cli.CommandVersion}, buildProject.Telemetry),
			options:    bare,
			components: minimal,
		},
		{
			name:       "init",
			option:     Init(&cli.Options{Type: cli.CommandInit}, buildProject.Telemetry),
			options:    bare,
			components: minimal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				populated options
				bound     loggers
			)

			app := fx.New(tt.option, probe, fx.Populate(&populated, &bound))

			require.NoError(t, app.Err())
			assert.Equal(t, tt.options, populated)
			assert.Equal(t, tt.components, components(bound))
		})
	}
}

func Test_newTheme_FallsBackToDarkWithoutATerminal(t *testing.T) {
	theme := newTheme()

	result := theme()

	assert.Equal(t, terminal.AppearanceDark, result.Appearance)
}

func Test_newDiscardLogger(t *testing.T) {
	result := newDiscardLogger()

	assert.Equal(t, slog.DiscardHandler, result.Handler())
}
