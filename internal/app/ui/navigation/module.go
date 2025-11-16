package navigation

import "go.uber.org/fx"

// Module provides the navigation dependencies
var Module = fx.Options(
	fx.Provide(NewNavigator),
)
