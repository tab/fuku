package modules

import (
	"go.uber.org/fx"

	"fuku/internal/adapters/cli"
	"fuku/internal/bootstrap/lifecycle"
	"fuku/internal/model"
)

// Version composes the version line
func Version(cmd *cli.Options, telemetry model.Telemetry) fx.Option {
	return fx.Options(
		base,
		standalone(cmd, telemetry),
		fx.Provide(func(v *cli.Version) lifecycle.Command { return v }),
	)
}
