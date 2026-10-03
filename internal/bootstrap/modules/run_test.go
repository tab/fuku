package modules

import (
	"bytes"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"

	"fuku/internal/adapters/cli"
	"fuku/internal/adapters/eventlog"
	"fuku/internal/adapters/instance"
	"fuku/internal/adapters/logsocket"
	"fuku/internal/adapters/output"
	"fuku/internal/adapters/process"
	"fuku/internal/adapters/resources"
	"fuku/internal/adapters/rest"
	"fuku/internal/adapters/telemetry"
	"fuku/internal/adapters/terminal"
	"fuku/internal/adapters/tui"
	"fuku/internal/adapters/watch"
	"fuku/internal/app/environment"
	"fuku/internal/app/registry"
	"fuku/internal/app/services"
	"fuku/internal/app/updater"
	"fuku/internal/bootstrap/lifecycle"
	"fuku/internal/model"
	"fuku/internal/platform/logging"
)

func Test_runParams_participants(t *testing.T) {
	observers := observerParams{
		Base: telemetryParams{
			Collector: &telemetry.Collector{},
			Tracer:    &telemetry.Tracer{},
			Announcer: &cli.Announcer{},
			Command:   &cli.Run{},
		},
		Recorder: &eventlog.Recorder{},
	}
	params := runParams{
		Guard:    &instance.Guard{},
		Runtime:  &services.Runtime{},
		Registry: &registry.Store{},
		Watcher:  &watch.Watcher{},
		Socket:   &logsocket.Server{},
		Sampler:  &resources.Sampler{},
	}
	server := &rest.Server{}
	store := &environment.Store{}
	bridge := &tui.Bridge{}
	checker := &updater.Checker{}

	consumers := []lifecycle.Consumer{params.Runtime, params.Registry, params.Watcher, observers.Recorder}
	producers := []lifecycle.Producer{observers.Base.Announcer, params.Socket, params.Watcher, params.Runtime, params.Sampler}

	tests := []struct {
		name      string
		server    *rest.Server
		build     func(observers observerParams, p runParams) lifecycle.Participants
		consumers []lifecycle.Consumer
		producers []lifecycle.Producer
	}{
		{
			name:   "the view with the API: the environment store and the bridge subscribe last, the update check starts last",
			server: server,
			build: func(observers observerParams, p runParams) lifecycle.Participants {
				return newViewParticipants(observers, p, store, bridge, checker)
			},
			consumers: []lifecycle.Consumer{params.Runtime, params.Registry, params.Watcher, observers.Recorder, store, bridge},
			producers: []lifecycle.Producer{observers.Base.Announcer, params.Socket, params.Watcher, params.Runtime, params.Sampler, server, checker},
		},
		{
			name:   "headless without the API: no environment store, the run command waits on the runtime, no update check",
			server: nil,
			build: func(observers observerParams, p runParams) lifecycle.Participants {
				return p.participants(observers)
			},
			consumers: consumers,
			producers: producers,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := params
			p.Server = tt.server

			participants := tt.build(observers, p)

			assert.Equal(t, params.Guard, participants.Guard)
			assert.Equal(t, tt.consumers, participants.Consumers)
			assert.Equal(t, tt.producers, participants.Producers)
			assert.Equal(t, observers.Base.Command, participants.Command)
		})
	}
}

func Test_stopTimeout(t *testing.T) {
	tests := []struct {
		name     string
		project  model.Project
		expected time.Duration
	}{
		{
			name:     "a project without services keeps the default budget",
			project:  model.Project{},
			expected: fx.DefaultTimeout,
		},
		{
			name:     "every service adds the graceful shutdown of its child",
			project:  model.Project{Services: []model.Service{{Name: "api"}, {Name: "db"}, {Name: "web"}}},
			expected: fx.DefaultTimeout + 3*process.ShutdownTimeout,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			timeout := stopTimeout(tt.project)

			assert.Equal(t, tt.expected, timeout)
		})
	}
}

func Test_disableWriter(t *testing.T) {
	theme := func() terminal.Theme { return terminal.NewTheme(terminal.AppearanceLight) }
	log := terminal.NewLog(terminal.Options{Format: logging.FormatConsole}, theme)

	var stdout bytes.Buffer

	enabled := newWriter(output.Options{Format: logging.FormatConsole}, log, &stdout)

	writer := disableWriter(enabled)
	_, err := writer.Write([]byte(`{"message":"hello from the writer"}` + "\n"))

	require.NoError(t, err)
	assert.Same(t, enabled, writer)
	assert.Empty(t, stdout.String())
}

func Test_Run_Writer(t *testing.T) {
	project := model.Project{
		Logging:     model.Logging{Level: logging.LevelInfo, Format: logging.FormatJSON},
		Concurrency: model.Concurrency{Workers: 5},
	}

	var stdout bytes.Buffer

	write := func(writer *output.Writer) error {
		_, err := writer.Write([]byte(`{"message":"hello from the writer"}` + "\n"))

		return err
	}
	probe := fx.Options(
		fx.Supply(SentryDSN("")),
		fx.Replace(fx.Annotate(&stdout, fx.As(new(io.Writer)), fx.ResultTags(`name:"stdout"`))),
		fx.Invoke(write),
	)

	tests := []struct {
		name    string
		before  func()
		cmd     *cli.Options
		written bool
	}{
		{
			name:    "with the TUI the writer drops the line",
			before:  stdout.Reset,
			cmd:     &cli.Options{Type: cli.CommandRun, Profile: model.ProfileDefault},
			written: false,
		},
		{
			name:    "the detached child drops the line",
			before:  stdout.Reset,
			cmd:     &cli.Options{Type: cli.CommandRun, Profile: model.ProfileDefault, NoUI: true, DetachedChild: true},
			written: false,
		},
		{
			name:    "without a UI the writer prints the line",
			before:  stdout.Reset,
			cmd:     &cli.Options{Type: cli.CommandRun, Profile: model.ProfileDefault, NoUI: true},
			written: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.before()

			app := fx.New(Run(tt.cmd, project), probe)

			require.NoError(t, app.Err())
			assert.Equal(t, tt.written, bytes.Contains(stdout.Bytes(), []byte("hello from the writer")))
		})
	}
}
