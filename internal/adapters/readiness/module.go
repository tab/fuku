package readiness

import "go.uber.org/fx"

// Module provides the readiness checker for dependency injection
var Module = fx.Options(
	fx.Provide(NewChecker),
)
