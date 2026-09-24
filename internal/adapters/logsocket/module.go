package logsocket

import "go.uber.org/fx"

// Module provides the log socket server and client for dependency injection
var Module = fx.Options(
	fx.Provide(
		NewServer,
		NewClient,
	),
)
