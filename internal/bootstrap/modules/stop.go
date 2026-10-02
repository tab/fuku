package modules

import (
	"go.uber.org/fx"

	"fuku/internal/adapters/cli"
	"fuku/internal/adapters/detach"
	"fuku/internal/adapters/instance"
	"fuku/internal/adapters/logsocket"
	"fuku/internal/app/services"
	"fuku/internal/bootstrap/lifecycle"
	"fuku/internal/model"
)

// Stop composes the stop of a profile: the running instance is signalled, then the cleaner kills what it left behind
func Stop(cmd *cli.Options, project model.Project) fx.Option {
	return fx.Options(
		base,
		configured(cmd, project),
		processes,
		fx.Provide(
			func(p model.Project) detach.StopOptions { return detach.StopOptions{Timeout: stopTimeout(p)} },
			func(c *logsocket.Client) detach.StatusSource { return c },
			func(s *detach.Stopper) cli.Instance { return s },
			func(c *services.Cleaner) cli.Cleaner { return c },
			func(c *cli.Stop) lifecycle.Command { return c },
			observerParams.participants,
		),
		detach.Module,
		instance.Module,
		logsocket.Module,
	)
}
