package registry

import "go.uber.org/fx"

// Module provides the registry and its dependencies
var Module = fx.Options(
	fx.Provide(
		NewRegistry,
	),
)
