package telemetry

import (
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/getsentry/sentry-go"

	"fuku/internal/platform/buildinfo"
)

// Flush deadlines for the regular exit and for a panic that is about to unwind the process
const (
	flushTimeout = 2 * time.Second
	panicTimeout = 5 * time.Second
)

// Client owns the Sentry SDK for the lifetime of the process
type Client struct{}

// NewClient initializes the Sentry SDK, or leaves it uninitialized when telemetry is off
func NewClient(options Options) *Client {
	if !options.Enabled {
		return &Client{}
	}

	err := sentry.Init(sentry.ClientOptions{
		Dsn:                   options.DSN,
		Environment:           options.Environment,
		Release:               fmt.Sprintf("%s@%s", buildinfo.AppName, buildinfo.Version),
		AttachStacktrace:      true,
		SampleRate:            1.0,
		EnableTracing:         true,
		TracesSampleRate:      0.1,
		BeforeSend:            stripPII,
		BeforeSendTransaction: stripPII,
		Transport:             options.transport,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize Sentry: %v\n", err)

		return &Client{}
	}

	sentry.ConfigureScope(func(scope *sentry.Scope) {
		scope.SetTag(TagEnv, options.Environment)
		scope.SetTag(TagOS, runtime.GOOS)
		scope.SetTag(TagArch, runtime.GOARCH)
		scope.SetTag(TagGoVersion, runtime.Version())

		if id := loadTelemetryID(); id != "" {
			scope.SetUser(sentry.User{ID: id})
		}
	})

	return &Client{}
}

// Flush waits for pending Sentry events to be sent
func (c *Client) Flush() {
	sentry.Flush(flushTimeout)
}

// Recover reports a recovered panic value and flushes it before the caller re-panics
func (c *Client) Recover(r any) {
	sentry.CurrentHub().Recover(r)
	sentry.Flush(panicTimeout)
}

// stripPII removes or obfuscates sensitive information from a Sentry event before it's sent
func stripPII(event *sentry.Event, _ *sentry.EventHint) *sentry.Event {
	event.ServerName = ""
	event.User = sentry.User{ID: event.User.ID}

	return event
}
