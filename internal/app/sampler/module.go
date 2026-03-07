package sampler

import (
	"context"

	"go.uber.org/fx"

	"fuku/internal/config"
)

// Module provides the resource sampler for dependency injection
var Module = fx.Module("sampler",
	fx.Provide(
		fx.Annotate(
			NewSampler,
			fx.ParamTags(``, `name:"sampler"`),
		),
	),
	fx.Invoke(func(lc fx.Lifecycle, cfg *config.Config, s Sampler) {
		if cfg.TelemetryDisabled() {
			return
		}

		ctx, cancel := context.WithCancel(context.Background())

		lc.Append(fx.Hook{
			OnStart: func(_ context.Context) error {
				go s.Run(ctx)

				return nil
			},
			OnStop: func(_ context.Context) error {
				cancel()

				return nil
			},
		})
	}),
)
