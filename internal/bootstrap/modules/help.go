package modules

import (
	"go.uber.org/fx"

	"fuku/internal/adapters/cli"
	"fuku/internal/bootstrap/lifecycle"
	"fuku/internal/model"
)

// Help composes the usage text
func Help(cmd *cli.Options, telemetry model.Telemetry) fx.Option {
	return fx.Options(
		base,
		standalone(cmd, telemetry),
		fx.Provide(func(h *cli.Help) lifecycle.Command { return h }),
	)
}
