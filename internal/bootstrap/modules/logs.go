package modules

import (
	"io"

	"go.uber.org/fx"

	"fuku/internal/adapters/cli"
	"fuku/internal/adapters/instance"
	"fuku/internal/adapters/logsocket"
	"fuku/internal/adapters/terminal"
	"fuku/internal/adapters/tui"
	"fuku/internal/app/logs"
	"fuku/internal/bootstrap/lifecycle"
	"fuku/internal/model"
)

// Logs composes the log stream of the running instance: the session over the socket client, inline or as bare lines
func Logs(cmd *cli.Options, project model.Project) fx.Option {
	return fx.Options(
		base,
		configured(cmd, project),
		fx.Provide(
			fx.Annotate(newLogsView, fx.ParamTags(``, ``, ``, `name:"stdout"`)),
			func(cmd *cli.Options) logs.Request {
				return logs.Request{Profile: cmd.Profile, Services: cmd.Services, ReplayOptions: cmd.ReplayOptions}
			},
			func(c *logsocket.Client) logs.Client { return c },
			func(s *logs.Session) cli.Session { return s },
			func(c *cli.Logs) lifecycle.Command { return c },
			observerParams.participants,
		),
		instance.Module,
		logsocket.Module,
	)
}

// newLogsView selects the view of the logs command: bare lines without a UI, the banner and styled lines otherwise
func newLogsView(cmd *cli.Options, theme func() terminal.Theme, log *terminal.Log, stdout io.Writer) logs.View {
	if cmd.NoUI {
		return cli.NewLogView(log, stdout)
	}

	return tui.NewLogView(theme, log, stdout)
}
