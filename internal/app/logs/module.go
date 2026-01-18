package logs

import (
	"go.uber.org/fx"
)

// Module provides the logs package dependencies
var Module = fx.Options(
	fx.Provide(NewClient),
	fx.Provide(NewRunner),
)
