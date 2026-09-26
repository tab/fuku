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

// Client owns the Sentry SDK from Start to Stop
type Client struct {
	options Options
	sentry  *sentry.Client
}

// NewClient creates the Sentry client, which initializes nothing until Start
func NewClient(options Options) *Client {
	return &Client{options: options}
}

// Start initializes the Sentry SDK, or leaves it uninitialized when telemetry is off
func (c *Client) Start() {
	if !c.options.Enabled {
		return
	}

	client, err := sentry.NewClient(sentry.ClientOptions{
		Dsn:                   c.options.DSN,
		Environment:           c.options.Environment,
		Release:               fmt.Sprintf("%s@%s", buildinfo.AppName, buildinfo.Version),
		AttachStacktrace:      true,
		SampleRate:            1.0,
		EnableTracing:         true,
		TracesSampleRate:      0.1,
		BeforeSend:            stripPII,
		BeforeSendTransaction: stripPII,
		Transport:             c.options.transport,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize Sentry: %v\n", err)

		return
	}

	sentry.CurrentHub().BindClient(client)
	sentry.ConfigureScope(func(scope *sentry.Scope) {
		scope.SetTag(TagOS, runtime.GOOS)
		scope.SetTag(TagArch, runtime.GOARCH)
		scope.SetTag(TagGoVersion, runtime.Version())

		if id := loadTelemetryID(); id != "" {
			scope.SetUser(sentry.User{ID: id})
		}
	})

	c.sentry = client
}

// Stop unbinds the SDK so a later Recover sends nothing, flushes pending events and closes the transport
func (c *Client) Stop() {
	if c.sentry == nil {
		return
	}

	sentry.CurrentHub().BindClient(nil)

	// ponytail: a timed-out flush leaves the transport to the process exit; close it too once sentry-go bounds Close
	if c.sentry.Flush(flushTimeout) {
		c.sentry.Close()
	}
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
