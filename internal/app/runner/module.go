package runner

import (
	"go.uber.org/fx"

	"fuku/internal/app/discovery"
	"fuku/internal/app/lifecycle"
	"fuku/internal/app/readiness"
	"fuku/internal/app/registry"
)

// Module provides the runner and its dependencies
var Module = fx.Options(
	fx.Provide(
		discovery.New,
		NewGuard,
		lifecycle.New,
		readiness.New,
		NewService,
		NewRunner,
		registry.New,
		NewWorkerPool,
	),
)
