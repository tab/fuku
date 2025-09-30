package runner

import (
	"go.uber.org/fx"

	"fuku/internal/app/readiness"
	"fuku/internal/config"
	"fuku/internal/config/logger"
)

// newRunnerForFX creates a runner with a no-op callback for FX dependency injection
func newRunnerForFX(cfg *config.Config, readinessFactory readiness.Factory, log logger.Logger) Runner {
	return NewRunner(cfg, readinessFactory, log, func(Event) {
		// Discard all events for non-TUI usage
	})
}

// Module provides the fx dependency injection options for the runner package
var Module = fx.Options(
	fx.Provide(newRunnerForFX),
)
