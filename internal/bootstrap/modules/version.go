package modules

import (
	"go.uber.org/fx"

	"fuku/internal/adapters/cli"
	"fuku/internal/bootstrap/lifecycle"
)

// Version composes the version line
func Version(cmd *cli.Options) fx.Option {
	return fx.Options(
		base,
		standalone(cmd),
		fx.Provide(func(v *cli.Version) lifecycle.Command { return v }),
	)
}
