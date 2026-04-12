package sampler

import (
	"context"

	"go.uber.org/fx"

	"fuku/internal/config"
)

// Module provides the resource sampler for dependency injection
var Module = fx.Options(
	fx.Provide(
		fx.Annotate(
			NewSampler,
			fx.ParamTags(``, `name:"sampler"`),
		),
	),
	fx.Invoke(startSampler),
)

// startSampler starts the resource sampler as part of the FX lifecycle
func startSampler(lc fx.Lifecycle, ctx context.Context, cfg *config.Config, s Sampler) {
	if cfg.TelemetryDisabled() {
		return
	}

	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			go s.Run(ctx)

			return nil
		},
	})
}
