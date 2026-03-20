package bus

import (
	"go.uber.org/fx"

	"fuku/internal/config"
	"fuku/internal/config/logger"
)

// Module provides bus for dependency injection
var Module = fx.Module("bus",
	fx.Provide(NewFormatter),
	fx.Provide(func(cfg *config.Config, formatter *Formatter, log logger.Logger) Bus {
		return NewBus(cfg, formatter, log.WithComponent("BUS"))
	}),
)
