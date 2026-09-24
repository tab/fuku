package doctor

import "go.uber.org/fx"

// Module provides the doctor runner for dependency injection
var Module = fx.Options(
	fx.Provide(NewRunner),
)
