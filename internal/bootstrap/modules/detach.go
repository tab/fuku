package modules

import (
	"os"

	"github.com/charmbracelet/x/term"
	"go.uber.org/fx"

	"fuku/internal/adapters/cli"
	"fuku/internal/adapters/detach"
	"fuku/internal/adapters/tui"
	"fuku/internal/bootstrap/lifecycle"
	"fuku/internal/model"
)

// exitInterrupted is the exit code of a detached start that a signal aborted, as a shell reports SIGINT
const exitInterrupted = 130

// Detach composes the parent of a detached run: it launches the child and renders its startup
func Detach(cmd *cli.Options, project model.Project) fx.Option {
	return fx.Options(
		fx.StopTimeout(stopTimeout(project)+fx.DefaultTimeout),
		base,
		configured(cmd, project),
		fx.Provide(
			func(cmd *cli.Options) detach.Options {
				return detach.Options{Profile: cmd.Profile, ConfigFile: cmd.ConfigFile}
			},
			func(l *detach.Launcher) detach.Starter { return l },
			func(cmd *cli.Options) tui.Options { return tui.Options{Profile: cmd.Profile} },
			fx.Annotate(tui.NewStartup, fx.ParamTags(``, ``, `name:"stdout"`)),
			newView,
			func(c *detach.Command) lifecycle.Command { return c },
			observerParams.participants,
		),
		fx.Decorate(disableWriter, interruptible),
		detach.Module,
	)
}

// newView picks the live view on a terminal, and plain lines for a pipe or --no-ui
func newView(cmd *cli.Options, plain *detach.Plain, live *tui.Startup) detach.View {
	if cmd.NoUI || !term.IsTerminal(os.Stdout.Fd()) {
		return plain
	}

	return live
}

// interruptible makes a signal that aborts the detached start exit like an interrupted shell command
func interruptible(arbiter *lifecycle.Arbiter) *lifecycle.Arbiter {
	arbiter.SetSignalCode(exitInterrupted)

	return arbiter
}
