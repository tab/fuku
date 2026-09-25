package modules

import (
	"bytes"
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
		Recorder:  &eventlog.Recorder{},
		Collector: &telemetry.Collector{},
		Tracer:    &telemetry.Tracer{},
		Announcer: &cli.Announcer{},
	}
	params := runParams{
		Guard:    &instance.Guard{},
		Runtime:  &services.Runtime{},
		Registry: &registry.Store{},
		Watcher:  &watch.Watcher{},
		Socket:   &logsocket.Server{},
		Sampler:  &resources.Sampler{},
		Checker:  &updater.Checker{},
	}
	server := &rest.Server{}
	store := &environment.Store{}
	bridge := &tui.Bridge{}
	program := &tui.Program{}
	run := &cli.Run{}

	consumers := []lifecycle.Consumer{params.Runtime, params.Registry, params.Watcher, observers.Recorder}
	producers := []lifecycle.Producer{observers.Announcer, params.Socket, params.Watcher, params.Runtime, params.Sampler, params.Checker}

	tests := []struct {
		name      string
		server    *rest.Server
		build     func(observers observerParams, p runParams) lifecycle.Participants
		consumers []lifecycle.Consumer
		producers []lifecycle.Producer
		command   lifecycle.Command
	}{
		{
			name:   "the view with the API: the environment store and the bridge subscribe last, the API server starts last",
			server: server,
			build: func(observers observerParams, p runParams) lifecycle.Participants {
				return newViewParticipants(observers, p, store, bridge, program)
			},
			consumers: []lifecycle.Consumer{params.Runtime, params.Registry, params.Watcher, observers.Recorder, store, bridge},
			producers: []lifecycle.Producer{observers.Announcer, params.Socket, params.Watcher, params.Runtime, params.Sampler, params.Checker, server},
			command:   program,
		},
		{
			name:   "headless without the API: no environment store, the run command waits on the runtime, the update check still runs",
			server: nil,
			build: func(observers observerParams, p runParams) lifecycle.Participants {
				return newHeadlessParticipants(observers, p, run)
			},
			consumers: consumers,
			producers: producers,
			command:   run,
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
			assert.Equal(t, tt.command, participants.Command)
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

func Test_newRunWriter(t *testing.T) {
	theme := func() terminal.Theme { return terminal.NewTheme(terminal.AppearanceLight) }
	log := terminal.NewLog(terminal.Options{Format: logging.FormatConsole}, theme)

	tests := []struct {
		name    string
		cmd     *cli.Options
		enabled bool
	}{
		{
			name:    "with the TUI the writer stays disabled until the view returns the terminal",
			cmd:     &cli.Options{Type: cli.CommandRun},
			enabled: false,
		},
		{
			name:    "without a UI the writer is enabled",
			cmd:     &cli.Options{Type: cli.CommandRun, NoUI: true},
			enabled: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout bytes.Buffer

			writer := newRunWriter(output.Options{Format: logging.FormatConsole}, log, tt.cmd, &stdout)

			_, err := writer.Write([]byte(`{"message":"hello from the writer"}` + "\n"))

			require.NoError(t, err)
			assert.Equal(t, tt.enabled, bytes.Contains(stdout.Bytes(), []byte("hello from the writer")))
		})
	}
}
