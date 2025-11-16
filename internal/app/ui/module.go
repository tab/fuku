package ui

import (
	"go.uber.org/fx"

	"fuku/internal/app/ui/services"
)

// Module provides the fx dependency injection options for the ui package
var Module = fx.Options(
	services.Module,
)
