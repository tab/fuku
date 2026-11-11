package modules

import (
	"net/http"
	"os"

	"github.com/charmbracelet/x/term"
	"go.uber.org/fx"

	"fuku/internal/adapters/cli"
	"fuku/internal/adapters/config"
	"fuku/internal/adapters/logsocket"
	"fuku/internal/adapters/mcp"
	"fuku/internal/adapters/rest"
	"fuku/internal/adapters/terminal"
	"fuku/internal/adapters/tui"
	"fuku/internal/bootstrap/lifecycle"
	"fuku/internal/model"
)

// MCP composes the stdio MCP server, which loads the config and finds the running instance on each tool call
func MCP(cmd *cli.Options, telemetry model.Telemetry) fx.Option {
	return fx.Options(
		base,
		standalone(cmd, telemetry),
		fx.Provide(
			func() func(model.Instance) mcp.Instance {
				return func(identity model.Instance) mcp.Instance { return logsocket.NewClient(identity) }
			},
			rest.NewHTTPClient,
			func(c *http.Client) rest.HTTPDoer { return c },
			func(doer rest.HTTPDoer) func(address, token string) (mcp.Client, error) {
				return func(address, token string) (mcp.Client, error) {
					client, err := rest.NewClient(doer, address, token)
					if err != nil {
						return nil, err
					}

					return client, nil
				}
			},
			func() mcp.Loader { return config.LoadDir },
			// ponytail: dark only, since the live console usually shares its terminal with stdin and the reply would reach the protocol; resolve from stderr when stdin is a pipe
			func() terminal.Theme { return terminal.NewTheme(terminal.AppearanceDark) },
			fx.Annotate(tui.NewMCPConsole, fx.ParamTags(``, `name:"stderr"`)),
			newConsole,
			func(b *mcp.Binary) mcp.Executor { return b },
			func(s *mcp.Server) lifecycle.Command { return s },
		),
		mcp.Module,
	)
}

// newConsole picks the console for the process stderr
func newConsole(cmd *cli.Options, plain *mcp.Plain, live *tui.MCPConsole) mcp.Console {
	return pickConsole(cmd.NoUI, term.IsTerminal(os.Stderr.Fd()), plain, live)
}

// pickConsole picks the live console when stderr is a terminal, and plain lines for the host's log or --no-ui
func pickConsole(noUI, isTerminal bool, plain *mcp.Plain, live *tui.MCPConsole) mcp.Console {
	if noUI || !isTerminal {
		return plain
	}

	return live
}
