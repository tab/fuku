package registry

import "go.uber.org/fx"

// Module provides the runtime store and its dependencies
var Module = fx.Options(
	fx.Provide(NewStore),
)
