package generator

import "go.uber.org/fx"

// Module provides the generator dependencies
var Module = fx.Options(
	fx.Provide(NewGenerator),
)
