package main

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

var sentryDSN string

// main is the entry point for the application
func main() {
	os.Exit(runApp())
}

// runApp contains the main application logic
func runApp() (exitCode int) {
	cmd, err := cli.Parse(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)

		return 1
	}

	if cmd.Type.Standalone() {
		return createAppWithoutConfig(cmd).Run()
	}

	if err := cli.ChangeToConfigDir(cmd); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)

		return 1
	}

	if cmd.Type == cli.CommandDoctor {
		return cli.RunDoctor(cmd)
	}

	cfg, topology, err := config.LoadPath(cmd.ConfigFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)

		return 1
	}

	if cmd.Type.RequiresServices() && len(cfg.Services) == 0 {
		fmt.Fprintf(os.Stderr, "Error: %v\n", errors.ErrNoServicesDefined)

		return 1
	}

	if cfg.Telemetry && cfg.SentryDSN == "" {
		cfg.SentryDSN = sentryDSN
	}

	shutdown := app.NewShutdown()

	application := createApp(appOptions{
		cfg:      cfg,
		topology: topology,
		cmd:      cmd,
		shutdown: shutdown,
	})

	return runFxApp(application, shutdown)
}

// runFxApp starts the FX application, waits for its shutdown signal and returns the exit code
func runFxApp(application *fx.App, shutdown *app.Shutdown) int {
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

// createAppWithoutConfig creates a lightweight app for standalone commands (init, version, help)
func createAppWithoutConfig(cmd *cli.Options) *cli.CLI {
	return cli.NewCLI(cmd)
}

// appOptions holds the inputs createApp supplies to the FX container
type appOptions struct {
	cfg      *config.Config
	topology *config.Topology
	cmd      *cli.Options
	shutdown *app.Shutdown
}

// createApp creates the FX application with the given config, topology and instance identity
func createApp(options appOptions) *fx.App {
	isDark := lipgloss.HasDarkBackground(os.Stdin, os.Stdout)
	log := render.NewLog(isDark)
	writer := render.NewWriter(options.cfg, log, os.Stdout)

	if options.cmd.NoUI || options.cmd.Type == cli.CommandLogs {
		writer.SetEnabled(true)
	}

	return fx.New(
		fx.WithLogger(createFxLogger(options.cfg)),
		fx.Supply(options.cfg, options.topology, log, options.cmd, writer, options.shutdown),
		fx.Provide(fx.Annotate(func() io.Writer {
			return os.Stderr
		}, fx.ResultTags(`name:"stderr"`))),
		fx.Provide(func() logger.Logger {
			return logger.NewLoggerWithOutput(options.cfg, writer)
		}),
		fx.Provide(logger.NewEventLogger),
		sentry.Module,
		app.Module,
	)
}

// createFxLogger returns an FX logger based on the config
func createFxLogger(cfg *config.Config) func() fxevent.Logger {
	return func() fxevent.Logger {
		if cfg.Logging.Level == logger.DebugLevel {
			return &fxevent.ConsoleLogger{W: os.Stdout}
		}

		return fxevent.NopLogger
	}
}
