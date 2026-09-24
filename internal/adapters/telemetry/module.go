package telemetry

import "go.uber.org/fx"

// Module provides the Sentry client, the metrics collector and the tracer for dependency injection
var Module = fx.Options(
	fx.Provide(
		NewClient,
		NewCollector,
		NewTracer,
	),
)
