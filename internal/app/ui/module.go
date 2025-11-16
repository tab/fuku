package ui

import (
	"go.uber.org/fx"
)

// Module provides the fx dependency injection options for the ui package
var Module = fx.Options(
	fx.Provide(NewLogFilter),
)
