package profiles

import "go.uber.org/fx"

// Module provides the profile resolver
var Module = fx.Options(
	fx.Provide(NewResolver),
)
