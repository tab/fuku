package modules

import (
	"go.uber.org/fx"

	"fuku/internal/adapters/cli"
	"fuku/internal/app/services"
	"fuku/internal/bootstrap/lifecycle"
	"fuku/internal/model"
)

// Stop composes the stop of a profile: the services cleaner kills what the profile left behind
func Stop(cmd *cli.Options, project model.Project) fx.Option {
	return fx.Options(
		base,
		configured(cmd, project),
		processes,
		fx.Provide(
			func(c *services.Cleaner) cli.Cleaner { return c },
			func(c *cli.Stop) lifecycle.Command { return c },
			observerParams.participants,
		),
	)
}
