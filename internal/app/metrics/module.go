package metrics

import (
	"context"

	"go.uber.org/fx"
)

// Module provides the metrics collector for dependency injection
var Module = fx.Options(
	fx.Provide(NewCollector),
	fx.Invoke(startCollector),
)

// startCollector starts the metrics collector as part of the FX lifecycle
func startCollector(lc fx.Lifecycle, ctx context.Context, c Collector) {
	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			go c.Run(ctx)

			return nil
		},
	})
}
