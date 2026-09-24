package modules

import (
	"io"
	"log/slog"
	"time"

	"go.uber.org/fx"

	"fuku/internal/adapters/cli"
	"fuku/internal/adapters/envfiles"
	"fuku/internal/adapters/instance"
	"fuku/internal/adapters/logsocket"
	"fuku/internal/adapters/output"
	"fuku/internal/adapters/process"
	"fuku/internal/adapters/resources"
	"fuku/internal/adapters/rest"
	"fuku/internal/adapters/terminal"
	"fuku/internal/adapters/tui"
	"fuku/internal/adapters/watch"
	"fuku/internal/app/environment"
	"fuku/internal/app/registry"
	"fuku/internal/app/services"
	"fuku/internal/app/updater"
	"fuku/internal/bootstrap/lifecycle"
	"fuku/internal/model"
)

// Run composes a run: the runtime under the view or headless, with the API when the project sets a listen address
func Run(cmd *cli.Options, project model.Project) fx.Option {
	command := headless
	if !cmd.NoUI {
		command = view
	}

	server := fx.Options()
	if project.Server.Listen != "" {
		server = api
	}

	return fx.Options(
		fx.StopTimeout(stopTimeout(project)),
		base,
		configured(cmd, project),
		runtime,
		server,
		fx.Provide(fx.Annotate(newRunWriter, fx.ParamTags(``, ``, ``, `name:"stdout"`))),
		command,
	)
}

// stopTimeout lets the stop wait out every child's graceful shutdown on top of the default budget
func stopTimeout(project model.Project) time.Duration {
	return fx.DefaultTimeout + time.Duration(len(project.Services))*process.ShutdownTimeout
}

// api serves the registry and the service control over HTTP
var api = fx.Options(
	fx.Provide(
		func(log *slog.Logger) rest.Logger { return log.With("component", "API") },
		func(p model.Project) rest.Options {
			return rest.Options{Listen: p.Server.Listen, Token: p.Server.Token}
		},
		func(s *registry.Store) rest.Registry { return s },
		func(c *services.Control) rest.Control { return c },
	),
	rest.Module,
)

// headless waits on the runtime as the command
var headless = fx.Options(
	fx.Provide(
		func(r *services.Runtime) cli.Runtime { return r },
		newHeadlessParticipants,
	),
)

// view runs the services view as the command, fed by the bridge, with the environment store the aside reads
var view = fx.Options(
	fx.Provide(
		func(log *slog.Logger) tui.Logger { return log.With("component", "UI") },
		func(cmd *cli.Options) tui.Options { return tui.Options{Profile: cmd.Profile} },
		func(w *output.Writer) tui.Output { return w },
		func(m *resources.ProcessMonitor) tui.Monitor { return m },
		func(r *envfiles.Reader) environment.Reader { return r },
		func(s *environment.Store) tui.Environment { return s },
		func(s *registry.Store) tui.Registry { return s },
		func(c *services.Control) tui.Control { return c },
		newViewParticipants,
	),
	envfiles.Module,
	environment.Module,
	tui.Module,
)

// runParams are the participants of the shared runtime (the API server exists only with a listen address)
type runParams struct {
	fx.In

	Guard    *instance.Guard
	Runtime  *services.Runtime
	Registry *registry.Store
	Watcher  *watch.Watcher
	Socket   *logsocket.Server
	Sampler  *resources.Sampler
	Checker  *updater.Checker
	Server   *rest.Server `optional:"true"`
}

// participants orders the runtime around the observers: consumers, producers after the announcer, then the API
func (p runParams) participants(observers observerParams) lifecycle.Participants {
	participants := observers.participants()
	participants.Guard = p.Guard
	participants.Consumers = append([]lifecycle.Consumer{p.Runtime, p.Registry, p.Watcher}, participants.Consumers...)
	participants.Producers = append(participants.Producers, p.Socket, p.Watcher, p.Runtime, p.Sampler, p.Checker)

	if p.Server != nil {
		participants.Producers = append(participants.Producers, p.Server)
	}

	return participants
}

// newHeadlessParticipants runs the profile with the headless command
func newHeadlessParticipants(observers observerParams, p runParams, run *cli.Run) lifecycle.Participants {
	participants := p.participants(observers)
	participants.Command = run

	return participants
}

// newViewParticipants runs the profile with the services view, whose store and bridge subscribe before any publish
func newViewParticipants(observers observerParams, p runParams, store *environment.Store, bridge *tui.Bridge, program *tui.Program) lifecycle.Participants {
	participants := p.participants(observers)
	participants.Consumers = append(participants.Consumers, store, bridge)
	participants.Command = program

	return participants
}

// newRunWriter creates the application log writer, enabled up front only when no TUI takes over the terminal
func newRunWriter(options output.Options, log *terminal.Log, cmd *cli.Options, stdout io.Writer) *output.Writer {
	writer := output.NewWriter(options, log, stdout)
	writer.SetEnabled(cmd.NoUI)

	return writer
}
