package modules

import (
	"bytes"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx/fxevent"

	"fuku/internal/adapters/cli"
	"fuku/internal/adapters/eventlog"
	"fuku/internal/adapters/output"
	"fuku/internal/adapters/telemetry"
	"fuku/internal/adapters/terminal"
	"fuku/internal/bootstrap/lifecycle"
	"fuku/internal/model"
	"fuku/internal/platform/logging"
)

func Test_observerParams_participants(t *testing.T) {
	recorder := &eventlog.Recorder{}
	collector := &telemetry.Collector{}
	tracer := &telemetry.Tracer{}
	announcer := &cli.Announcer{}

	tests := []struct {
		name      string
		telemetry telemetry.Options
		consumers []lifecycle.Consumer
	}{
		{
			name:      "with telemetry disabled only the event log observes, the announcer produces",
			telemetry: telemetry.Options{},
			consumers: []lifecycle.Consumer{recorder},
		},
		{
			name:      "the collector and the tracer join with telemetry enabled",
			telemetry: telemetry.Options{Enabled: true},
			consumers: []lifecycle.Consumer{recorder, collector, tracer},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := observerParams{Telemetry: tt.telemetry, Recorder: recorder, Collector: collector, Tracer: tracer, Announcer: announcer}

			participants := p.participants()

			assert.Nil(t, participants.Guard)
			assert.Equal(t, tt.consumers, participants.Consumers)
			assert.Equal(t, []lifecycle.Producer{announcer}, participants.Producers)
			assert.Nil(t, participants.Command)
		})
	}
}

func Test_newWriter(t *testing.T) {
	log := terminal.NewLog(terminal.Options{Format: logging.FormatConsole}, terminal.NewTheme(terminal.AppearanceLight))

	var stdout bytes.Buffer

	writer := newWriter(output.Options{Format: logging.FormatConsole}, log, &stdout)

	_, err := writer.Write([]byte(`{"message":"hello from the writer"}` + "\n"))

	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "hello from the writer")
}

func Test_newTelemetryOptions(t *testing.T) {
	tests := []struct {
		name      string
		telemetry model.Telemetry
		sentryDSN SentryDSN
		expected  telemetry.Options
	}{
		{
			name:      "environment DSN wins over the build DSN",
			telemetry: model.Telemetry{Enabled: true, DSN: "https://env@sentry.io/1", Environment: "development"},
			sentryDSN: "https://build@sentry.io/2",
			expected:  telemetry.Options{Enabled: true, DSN: "https://env@sentry.io/1", Environment: "development"},
		},
		{
			name:      "build DSN fills an empty environment DSN",
			telemetry: model.Telemetry{Enabled: true, Environment: "production"},
			sentryDSN: "https://build@sentry.io/2",
			expected:  telemetry.Options{Enabled: true, DSN: "https://build@sentry.io/2", Environment: "production"},
		},
		{
			name:      "no DSN at all disables telemetry",
			telemetry: model.Telemetry{Enabled: true, Environment: "production"},
			expected:  telemetry.Options{Environment: "production"},
		},
		{
			name:      "opt-out stays disabled with a DSN",
			telemetry: model.Telemetry{DSN: "https://env@sentry.io/1", Environment: "test"},
			sentryDSN: "https://build@sentry.io/2",
			expected:  telemetry.Options{DSN: "https://env@sentry.io/1", Environment: "test"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := newTelemetryOptions(tt.telemetry, tt.sentryDSN)

			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_fxLogger(t *testing.T) {
	log := slog.New(slog.DiscardHandler)
	debug := &fxevent.SlogLogger{Logger: log.With("component", "FX")}
	debug.UseLogLevel(slog.LevelDebug)

	tests := []struct {
		name   string
		level  string
		expect fxevent.Logger
	}{
		{
			name:   "debug level returns the application logger at debug",
			level:  logging.LevelDebug,
			expect: debug,
		},
		{
			name:   "info level returns the nop logger",
			level:  logging.LevelInfo,
			expect: fxevent.NopLogger,
		},
		{
			name:   "warn level returns the nop logger",
			level:  logging.LevelWarn,
			expect: fxevent.NopLogger,
		},
		{
			name:   "error level returns the nop logger",
			level:  logging.LevelError,
			expect: fxevent.NopLogger,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			project := model.Project{Logging: model.Logging{Level: tt.level}}

			result := fxLogger(project)(log)

			assert.Equal(t, tt.expect, result)
		})
	}
}
