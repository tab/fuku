package modules

import (
	"go.uber.org/fx"

	"fuku/internal/adapters/cli"
	"fuku/internal/bootstrap/lifecycle"
)

// Help composes the usage text
func Help(cmd *cli.Options) fx.Option {
	return fx.Options(
		base,
		standalone(cmd),
		fx.Provide(func(h *cli.Help) lifecycle.Command { return h }),
	)
}
