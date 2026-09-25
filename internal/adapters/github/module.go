package github

import "go.uber.org/fx"

// Module provides the release client and the HTTP client it sends through for dependency injection
var Module = fx.Options(
	fx.Provide(
		NewClient,
		newHTTPClient,
	),
)
