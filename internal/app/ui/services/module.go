package services

import (
	"go.uber.org/fx"
)

// Module provides the services UI and its dependencies
var Module = fx.Options(
	fx.Provide(
		NewController,
	),
)
