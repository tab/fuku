package runner

import (
	"go.uber.org/fx"
)

// Module provides the runner and its dependencies
var Module = fx.Options(
	fx.Provide(
		NewDiscovery,
		NewLifecycle,
		NewReadiness,
		NewService,
		NewRunner,
		NewRegistry,
		NewWorkerPool,
	),
)
