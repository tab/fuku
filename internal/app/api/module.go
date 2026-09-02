package api

import (
	"go.uber.org/fx"

	"fuku/internal/app/relay"
)

// Module provides API package dependencies for FX injection
var Module = fx.Options(
	fx.Provide(NewServer),
	fx.Provide(func(s *Server) Listener { return s }),
	fx.Provide(func(s *relay.Server) Journal { return s }),
)
