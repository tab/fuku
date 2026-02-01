package lifecycle

import "go.uber.org/fx"

// Module provides the lifecycle and its dependencies
var Module = fx.Options(
	fx.Provide(
		NewLifecycle,
	),
)
