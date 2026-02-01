package discovery

import "go.uber.org/fx"

// Module provides the discovery and its dependencies
var Module = fx.Options(
	fx.Provide(
		NewDiscovery,
	),
)
