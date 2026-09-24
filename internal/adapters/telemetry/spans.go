package telemetry

import (
	"time"

	"github.com/getsentry/sentry-go"
)

// Span operation names for trace instrumentation
const (
	OpDiscovery      = "discovery"
	OpPreflight      = "preflight"
	OpTierStartup    = "tier_startup"
	OpShutdown       = "shutdown"
	OpWatchRestart   = "watch_restart"
	OpServiceStop    = "service_stop"
	OpServiceRestart = "service_restart"
)

// withStartTime backdates a span to the moment its work began
func withStartTime(ts time.Time) sentry.SpanOption {
	return func(s *sentry.Span) {
		s.StartTime = ts
	}
}
