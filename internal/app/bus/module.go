package bus

import (
	"go.uber.org/fx"

	"fuku/internal/config"
	"fuku/internal/config/logger"
)

// Module provides bus for dependency injection
var Module = fx.Module("bus",
	fx.Provide(func(cfg *config.Config, log logger.Logger) Bus {
		return New(cfg, log.WithComponent("BUS"))
	}),
)
