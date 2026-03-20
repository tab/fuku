package relay

import (
	"context"

	"go.uber.org/fx"
)

// Module provides relay package dependencies for FX injection
var Module = fx.Options(
	fx.Provide(NewServer),
	fx.Provide(NewClient),
	fx.Provide(func(server Server) Broadcaster { return server }),
	fx.Provide(NewBridge),
	fx.Invoke(startBridge),
)

// startBridge starts the bus-to-relay bridge as part of the FX lifecycle
func startBridge(lc fx.Lifecycle, bridge *Bridge) {
	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			bridge.Start()

			return nil
		},
		OnStop: func(_ context.Context) error {
			bridge.Stop()

			return nil
		},
	})
}
