package metrics

import (
	"context"

	"go.uber.org/fx"
)

// Module provides the metrics collector for dependency injection
var Module = fx.Module("metrics",
	fx.Provide(NewCollector),
	fx.Invoke(func(lc fx.Lifecycle, c Collector) {
		ctx, cancel := context.WithCancel(context.Background())

		lc.Append(fx.Hook{
			OnStart: func(_ context.Context) error {
				go c.Run(ctx)

				return nil
			},
			OnStop: func(_ context.Context) error {
				cancel()

				return nil
			},
		})
	}),
)
