package tracer

import (
	"context"

	"go.uber.org/fx"

	"fuku/internal/app/bus"
	"fuku/internal/config"
)

// Module provides the tracer for dependency injection
var Module = fx.Module("tracer",
	fx.Provide(NewTracer),
	fx.Invoke(func(lc fx.Lifecycle, cfg *config.Config, t Tracer, b bus.Bus) {
		if cfg.TelemetryDisabled() {
			return
		}

		ctx, cancel := context.WithCancel(context.Background())

		lc.Append(fx.Hook{
			OnStart: func(_ context.Context) error {
				ch := b.Subscribe(ctx)
				go t.Run(ctx, ch)

				return nil
			},
			OnStop: func(_ context.Context) error {
				cancel()

				return nil
			},
		})
	}),
)
