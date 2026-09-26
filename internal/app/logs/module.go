package logs

import "go.uber.org/fx"

// Module provides the log hub and the log session for dependency injection
var Module = fx.Options(
	fx.Provide(
		NewHub,
		NewSession,
	),
)
