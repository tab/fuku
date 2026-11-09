package modules

import (
	"net/http"

	"go.uber.org/fx"

	"fuku/internal/adapters/cli"
	"fuku/internal/adapters/config"
	"fuku/internal/adapters/logsocket"
	"fuku/internal/adapters/mcp"
	"fuku/internal/adapters/rest"
	"fuku/internal/adapters/terminal"
	"fuku/internal/bootstrap/lifecycle"
	"fuku/internal/model"
)

// MCP composes the stdio MCP server, which loads the config and finds the running instance on each tool call
func MCP(cmd *cli.Options, telemetry model.Telemetry) fx.Option {
	return fx.Options(
		base,
		standalone(cmd, telemetry),
		fx.Provide(
			func() mcp.Dialer {
				return func(identity model.Instance) mcp.Client { return logsocket.NewClient(identity) }
			},
			rest.NewHTTPClient,
			func(c *http.Client) rest.HTTPDoer { return c },
			func(doer rest.HTTPDoer) mcp.APIDialer {
				return func(address, token string) (mcp.API, error) {
					client, err := rest.NewClient(doer, address, token)
					if err != nil {
						return nil, err
					}

					return client, nil
				}
			},
			func() mcp.Loader { return config.LoadPath },
			func() terminal.Theme { return terminal.NewTheme(terminal.AppearanceDark) },
			func(b *mcp.Binary) mcp.Executor { return b },
			func(s *mcp.Server) lifecycle.Command { return s },
		),
		mcp.Module,
	)
}
