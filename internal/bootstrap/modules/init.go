package modules

import (
	"go.uber.org/fx"

	"fuku/internal/adapters/cli"
	"fuku/internal/adapters/config"
	"fuku/internal/bootstrap/lifecycle"
	"fuku/internal/model"
)

// Init composes the creation of the fuku.yaml template
func Init(cmd *cli.Options, telemetry model.Telemetry) fx.Option {
	return fx.Options(
		base,
		standalone(cmd, telemetry),
		fx.Supply(config.Create),
		fx.Provide(func(i *cli.Init) lifecycle.Command { return i }),
	)
}
