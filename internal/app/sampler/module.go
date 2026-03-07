package sampler

import (
	"context"

	"go.uber.org/fx"
)

// Module provides the resource sampler for dependency injection
var Module = fx.Module("sampler",
	fx.Provide(NewSampler),
	fx.Invoke(func(lc fx.Lifecycle, s Sampler) {
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
