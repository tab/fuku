package bus

import (
	"go.uber.org/fx"

	"fuku/internal/app/logs"
	"fuku/internal/config"
	"fuku/internal/config/logger"
)

// Module provides bus for dependency injection
var Module = fx.Module("bus",
	fx.Provide(func(cfg *config.Config, server logs.Server, event logger.EventLogger, log logger.Logger) Bus {
		return NewBus(cfg, server, event, log.WithComponent("BUS"))
	}),
)
