package relay

import (
	"context"

	"go.uber.org/fx"
)

// Module provides relay package dependencies for FX injection
var Module = fx.Options(
	fx.Provide(NewServer),
	fx.Provide(NewClient),
	fx.Provide(func(server *Server) Broadcaster { return server }),
	fx.Provide(NewBridge),
	fx.Invoke(startBridge),
	fx.Invoke(startServer),
)

// startBridge starts the bus-to-relay bridge as part of the FX lifecycle
func startBridge(lc fx.Lifecycle, ctx context.Context, bridge *Bridge) {
	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			bridge.Start(ctx)

			return nil
		},
	})
}

// startServer starts the relay server as part of the FX lifecycle
func startServer(lc fx.Lifecycle, ctx context.Context, server *Server) {
	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			go server.Run(ctx)

			return nil
		},
		OnStop: func(_ context.Context) error {
			server.Stop()

			return nil
		},
	})
}
