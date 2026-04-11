package tracer

import (
	"context"

	"go.uber.org/fx"

	"fuku/internal/app/bus"
	"fuku/internal/config"
)

// Module provides the tracer for dependency injection
var Module = fx.Options(
	fx.Provide(NewTracer),
	fx.Invoke(startTracer),
)

// startTracer starts the tracer as part of the FX lifecycle
func startTracer(lc fx.Lifecycle, ctx context.Context, cfg *config.Config, t Tracer, b bus.Bus) {
	if cfg.TelemetryDisabled() {
		return
	}

	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			ch := b.Subscribe(ctx)
			go t.Run(ctx, ch)

			return nil
		},
	})
}
