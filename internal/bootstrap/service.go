package bootstrap

import (
	"context"
	"fmt"
	"io"
	"os"

	"charm.land/lipgloss/v2"
	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"

	"fuku/internal/app"
	"fuku/internal/app/cli"
	"fuku/internal/app/errors"
	"fuku/internal/app/render"
	"fuku/internal/config"
	"fuku/internal/config/logger"
	"fuku/internal/config/sentry"
)

// ServiceCLI handles the run, stop and logs commands, which need the project config and the application container
type ServiceCLI struct {
	cmd       *cli.Options
	sentryDSN string
}

// NewServiceCLI creates the CLI for the service commands with the build-time Sentry DSN as the telemetry default
func NewServiceCLI(cmd *cli.Options, sentryDSN string) *ServiceCLI {
	return &ServiceCLI{cmd: cmd, sentryDSN: sentryDSN}
}

// Run loads the project config, runs the command inside the application container and returns the exit code
func (c *ServiceCLI) Run() int {
	if err := cli.ChangeToConfigDir(c.cmd); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)

		return 1
	}

	cfg, topology, err := config.LoadPath(c.cmd.ConfigFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)

		return 1
	}

	if c.cmd.Type.RequiresServices() && len(cfg.Services) == 0 {
		fmt.Fprintf(os.Stderr, "Error: %v\n", errors.ErrNoServicesDefined)

		return 1
	}

	if cfg.Telemetry && cfg.SentryDSN == "" {
		cfg.SentryDSN = c.sentryDSN
	}

	application, shutdown := newContainer(containerOptions{
		cfg:      cfg,
		topology: topology,
		cmd:      c.cmd,
	})

	return runContainer(application, shutdown)
}

// containerOptions holds the runtime values the container is built around
type containerOptions struct {
	cfg      *config.Config
	topology *config.Topology
	cmd      *cli.Options
}

// newContainer builds the FX container for a command and returns it with the shutdown record it provides
func newContainer(options containerOptions) (*fx.App, *app.Shutdown) {
	var shutdown *app.Shutdown

	application := fx.New(
		fx.WithLogger(fxLogger(options.cfg)),
		fx.Supply(options.cfg, options.topology, options.cmd),
		fx.Provide(
			newLog,
			newWriter,
			newLogger,
			logger.NewEventLogger,
			app.NewShutdown,
			fx.Annotate(newStderr, fx.ResultTags(`name:"stderr"`)),
		),
		fx.Populate(&shutdown),
		sentry.Module,
		app.Module,
	)

	return application, shutdown
}

// runContainer starts the container, waits for its shutdown signal and returns the exit code
func runContainer(application *fx.App, shutdown *app.Shutdown) int {
	startCtx, cancelStart := context.WithTimeout(context.Background(), application.StartTimeout())
	defer cancelStart()

	err := application.Start(startCtx)

	if errors.Is(err, errors.ErrInstanceAlreadyRunning) {
		return 1
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)

		return 1
	}

	sig := <-application.Wait()
	shutdown.Observe(sig.Signal)

	stopCtx, cancelStop := context.WithTimeout(context.Background(), application.StopTimeout())
	defer cancelStop()

	if err := application.Stop(stopCtx); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)

		return 1
	}

	return sig.ExitCode
}

// newLog creates the log renderer for the terminal's background
func newLog() *render.Log {
	return render.NewLog(lipgloss.HasDarkBackground(os.Stdin, os.Stdout))
}

// newWriter creates the application log writer, enabled up front when no TUI will take over the terminal
func newWriter(cfg *config.Config, log *render.Log, cmd *cli.Options) *render.Writer {
	writer := render.NewWriter(cfg, log, os.Stdout)
	writer.SetEnabled(cmd.NoUI || cmd.Type == cli.CommandLogs)

	return writer
}

// newLogger creates the application logger that writes through the log writer
func newLogger(cfg *config.Config, writer *render.Writer) logger.Logger {
	return logger.NewLoggerWithOutput(cfg, writer)
}

// newStderr exposes the process stderr for components that write before the logger exists
func newStderr() io.Writer {
	return os.Stderr
}

// fxLogger returns the FX event logger for the configured log level
func fxLogger(cfg *config.Config) func() fxevent.Logger {
	return func() fxevent.Logger {
		if cfg.Logging.Level == logger.DebugLevel {
			return &fxevent.ConsoleLogger{W: os.Stdout}
		}

		return fxevent.NopLogger
	}
}
