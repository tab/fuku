package worker

import "go.uber.org/fx"

// Module provides the worker pool for dependency injection
var Module = fx.Options(
	fx.Provide(NewPool),
)
