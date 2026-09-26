package process

import "go.uber.org/fx"

// Module provides the process tracker, factory and preflight cleaner for dependency injection
var Module = fx.Options(
	fx.Provide(
		NewTracker,
		NewFactory,
		NewPreflight,
	),
)
