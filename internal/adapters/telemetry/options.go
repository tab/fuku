package telemetry

import "github.com/getsentry/sentry-go"

// Options enables telemetry with the DSN and environment it reports under
type Options struct {
	Enabled     bool
	DSN         string
	Environment string
	transport   sentry.Transport
}
