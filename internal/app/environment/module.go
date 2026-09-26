package environment

import "go.uber.org/fx"

// Module provides the environment store for dependency injection
var Module = fx.Options(
	fx.Provide(NewStore),
)
