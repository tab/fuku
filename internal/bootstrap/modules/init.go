package modules

import (
	"go.uber.org/fx"

	"fuku/internal/adapters/cli"
	"fuku/internal/adapters/config"
	"fuku/internal/bootstrap/lifecycle"
)

// Init composes the creation of the fuku.yaml template
func Init(cmd *cli.Options) fx.Option {
	return fx.Options(
		base,
		standalone(cmd),
		fx.Supply(config.Create),
		fx.Provide(func(i *cli.Init) lifecycle.Command { return i }),
	)
}
