package dotenv

import (
	"context"

	"go.uber.org/fx"
)

// Module provides the dotenv loader for dependency injection
var Module = fx.Options(
	fx.Provide(NewLoader),
	fx.Invoke(startLoader),
)

// startLoader starts the loader subscriber as part of the FX lifecycle
func startLoader(lc fx.Lifecycle, ctx context.Context, l Loader) {
	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			go l.Run(ctx)

			return nil
		},
	})
}
