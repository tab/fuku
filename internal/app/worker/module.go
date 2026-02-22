package worker

import "go.uber.org/fx"

// Module provides the worker pool and its dependencies
var Module = fx.Options(
	fx.Provide(NewWorkerPool),
)
