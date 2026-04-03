package registry

import (
	"context"

	"go.uber.org/fx"
)

// Module provides the registry and its dependencies
var Module = fx.Options(
	fx.Provide(
		NewRegistry,
		NewStore,
	),
	fx.Invoke(startStore),
)

// startStore starts the runtime store as part of the FX lifecycle
func startStore(lc fx.Lifecycle, store Store) {
	ctx, cancel := context.WithCancel(context.Background())

	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			go store.Run(ctx)

			store.WaitReady()

			return nil
		},
		OnStop: func(_ context.Context) error {
			cancel()

			return nil
		},
	})
}
