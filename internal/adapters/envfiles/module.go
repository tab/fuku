package envfiles

import "go.uber.org/fx"

// Module provides the .env file reader for dependency injection
var Module = fx.Options(
	fx.Provide(NewReader),
)
