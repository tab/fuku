package wire

import (
	"go.uber.org/fx"

	"fuku/internal/app/ui/services"
)

// Module aggregates all UI modules and provides the UI factory
var Module = fx.Options(
	services.Module,
	fx.Provide(NewUI),
)
