package instance

import "go.uber.org/fx"

// Module provides the instance identity for dependency injection
var Module = fx.Options(
	fx.Provide(NewInstance),
)
