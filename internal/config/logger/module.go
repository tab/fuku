package logger

import (
	"go.uber.org/fx"
)

// Module provides the fx dependency injection options for the logger package
var Module = fx.Options(
	fx.Provide(NewLogger),
)
