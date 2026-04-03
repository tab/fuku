package api

import (
	"go.uber.org/fx"
)

// Module provides API package dependencies for FX injection
var Module = fx.Options(
	fx.Provide(NewServer),
)
