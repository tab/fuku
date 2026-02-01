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
	discovery.Module,
	lifecycle.Module,
	readiness.Module,
	registry.Module,
	fx.Provide(
		NewGuard,
		NewWorkerPool,
		NewService,
		NewRunner,
	),
)
