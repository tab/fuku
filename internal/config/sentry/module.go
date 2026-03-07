package sentry

import "go.uber.org/fx"

// Module provides the fx dependency injection options for the sentry package
var Module = fx.Options(
	fx.Provide(NewSentry),
)
