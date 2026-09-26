package services

import "go.uber.org/fx"

// Module provides the services runtime, its guard, the shared control and the cleaner
var Module = fx.Options(
	fx.Provide(
		NewGuard,
		NewRuntime,
		NewControl,
		NewCleaner,
	),
)
