package bootstrap

import (
	"bytes"
	"context"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"

	"fuku/internal/app"
	"fuku/internal/app/cli"
	"fuku/internal/app/errors"
	"fuku/internal/app/relay"
	"fuku/internal/app/render"
	"fuku/internal/config"
	"fuku/internal/config/logger"
)

func Test_ServiceCLI_Run(t *testing.T) {
	tests := []struct {
		name   string
		cmd    *cli.Options
		expect int
	}{
		{
			name:   "config directory missing",
			cmd:    &cli.Options{Type: cli.CommandRun, Profile: config.Default, ConfigFile: "missing/fuku.yaml"},
			expect: 1,
		},
		{
			name:   "config file missing",
			cmd:    &cli.Options{Type: cli.CommandRun, Profile: config.Default, ConfigFile: "missing.yaml"},
			expect: 1,
		},
		{
			name:   "run without services",
			cmd:    &cli.Options{Type: cli.CommandRun, Profile: config.Default},
			expect: 1,
		},
		{
			name: "logs without an instance runs the container",
			cmd: &cli.Options{
				Type:          cli.CommandLogs,
				Profile:       "bootstrap-no-such-profile",
				NoUI:          true,
				ReplayOptions: relay.ReplayOptions{NoFollow: true},
			},
			expect: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			t.Setenv("SENTRY_DSN", "")
			t.Setenv("FUKU_UPDATER_DISABLED", "1")

			result := NewServiceCLI(tt.cmd, "").Run()

			assert.Equal(t, tt.expect, result)
		})
	}
}

func Test_newContainer(t *testing.T) {
	topology := &config.Topology{
		Order:        []string{},
		TierServices: make(map[string][]string),
	}

	tests := []struct {
		name  string
		level string
		cmd   *cli.Options
	}{
		{
			name:  "info level with TUI",
			level: logger.InfoLevel,
			cmd:   &cli.Options{Type: cli.CommandRun, Profile: config.Default},
		},
		{
			name:  "debug level without UI",
			level: logger.DebugLevel,
			cmd:   &cli.Options{Type: cli.CommandRun, Profile: config.Default, NoUI: true},
		},
		{
			name:  "error level",
			level: logger.ErrorLevel,
			cmd:   &cli.Options{Type: cli.CommandRun, Profile: config.Default},
		},
		{
			name:  "warn level in logs mode",
			level: logger.WarnLevel,
			cmd:   &cli.Options{Type: cli.CommandLogs},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.DefaultConfig()
			cfg.Logging.Level = tt.level

			application, shutdown := newContainer(containerOptions{
				cfg:      cfg,
				topology: topology,
				cmd:      tt.cmd,
			})

			require.NoError(t, application.Err())
			assert.NotNil(t, shutdown)
		})
	}
}

func Test_runContainer(t *testing.T) {
	tests := []struct {
		name   string
		option fx.Option
		expect int
	}{
		{
			name: "returns the exit code the application shuts down with",
			option: fx.Invoke(func(shutdowner fx.Shutdowner) {
				go func() {
					shutdowner.Shutdown(fx.ExitCode(3))
				}()
			}),
			expect: 3,
		},
		{
			name: "start error",
			option: fx.Invoke(func(lc fx.Lifecycle) {
				lc.Append(fx.Hook{OnStart: func(context.Context) error { return errors.ErrNoServicesDefined }})
			}),
			expect: 1,
		},
		{
			name: "another instance exits silently",
			option: fx.Invoke(func(lc fx.Lifecycle) {
				lc.Append(fx.Hook{OnStart: func(context.Context) error { return errors.ErrInstanceAlreadyRunning }})
			}),
			expect: 1,
		},
		{
			name: "stop error",
			option: fx.Invoke(func(lc fx.Lifecycle, shutdowner fx.Shutdowner) {
				lc.Append(fx.Hook{OnStop: func(context.Context) error { return errors.ErrNoServicesDefined }})

				go func() {
					shutdowner.Shutdown(fx.ExitCode(3))
				}()
			}),
			expect: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var shutdown *app.Shutdown

			application := fx.New(fx.NopLogger, fx.Provide(app.NewShutdown), fx.Populate(&shutdown), tt.option)
			require.NoError(t, application.Err())

			result := runContainer(application, shutdown)

			assert.Equal(t, tt.expect, result)
		})
	}
}

func Test_newWriter(t *testing.T) {
	tests := []struct {
		name    string
		cmd     *cli.Options
		enabled bool
	}{
		{
			name:    "run with TUI stays disabled until the TUI enables it",
			cmd:     &cli.Options{Type: cli.CommandRun},
			enabled: false,
		},
		{
			name:    "run without UI is enabled",
			cmd:     &cli.Options{Type: cli.CommandRun, NoUI: true},
			enabled: true,
		},
		{
			name:    "logs is enabled",
			cmd:     &cli.Options{Type: cli.CommandLogs},
			enabled: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldStdout := os.Stdout
			r, w, err := os.Pipe()
			require.NoError(t, err)

			os.Stdout = w

			t.Cleanup(func() { os.Stdout = oldStdout })

			writer := newWriter(config.DefaultConfig(), render.NewLog(false), tt.cmd)

			_, err = writer.Write([]byte(`{"message":"hello from the writer"}` + "\n"))
			require.NoError(t, err)

			w.Close()

			var buf bytes.Buffer

			_, _ = io.Copy(&buf, r)

			assert.Equal(t, tt.enabled, bytes.Contains(buf.Bytes(), []byte("hello from the writer")))
		})
	}
}

func Test_fxLogger(t *testing.T) {
	tests := []struct {
		name   string
		level  string
		expect fxevent.Logger
	}{
		{
			name:   "debug level returns the console logger",
			level:  logger.DebugLevel,
			expect: &fxevent.ConsoleLogger{W: os.Stdout},
		},
		{
			name:   "info level returns the nop logger",
			level:  logger.InfoLevel,
			expect: fxevent.NopLogger,
		},
		{
			name:   "warn level returns the nop logger",
			level:  logger.WarnLevel,
			expect: fxevent.NopLogger,
		},
		{
			name:   "error level returns the nop logger",
			level:  logger.ErrorLevel,
			expect: fxevent.NopLogger,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.DefaultConfig()
			cfg.Logging.Level = tt.level

			result := fxLogger(cfg)()

			assert.Equal(t, tt.expect, result)
		})
	}
}
