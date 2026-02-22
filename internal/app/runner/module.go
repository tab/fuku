package runner

import (
	"go.uber.org/fx"

	"fuku/internal/app/discovery"
	"fuku/internal/app/lifecycle"
	"fuku/internal/app/preflight"
	"fuku/internal/app/readiness"
	"fuku/internal/app/registry"
	"fuku/internal/app/worker"
)

// Module provides the runner and its dependencies
var Module = fx.Options(
	discovery.Module,
	lifecycle.Module,
	preflight.Module,
	readiness.Module,
	registry.Module,
	worker.Module,
	fx.Provide(
		NewGuard,
		NewService,
		NewRunner,
	),
)
