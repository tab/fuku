package sentry

import (
	"fmt"
	"os"
	"runtime"
	"time"

	gosentry "github.com/getsentry/sentry-go"

	"fuku/internal/config"
)

// Re-exported functions from sentry-go
var (
	ConfigureScope = gosentry.ConfigureScope
	CurrentHub     = gosentry.CurrentHub
	FlushSDK       = gosentry.Flush
)

// Re-exported types from sentry-go
type (
	Hub   = gosentry.Hub
	Scope = gosentry.Scope
)

// Sentry defines the interface for Sentry operations
type Sentry interface {
	Flush()
}

type sentryClient struct{}

// NewSentry creates a new Sentry client using application configuration
func NewSentry(cfg *config.Config) Sentry {
	if cfg.TelemetryDisabled() {
		return &sentryClient{}
	}

	err := gosentry.Init(gosentry.ClientOptions{
		Dsn:                   cfg.SentryDSN,
		Environment:           cfg.AppEnv,
		Release:               fmt.Sprintf("%s@%s", config.AppName, config.Version),
		AttachStacktrace:      true,
		SampleRate:            1.0,
		EnableTracing:         true,
		TracesSampleRate:      0.1,
		SendDefaultPII:        false,
		BeforeSend:            stripPII,
		BeforeSendTransaction: stripPII,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize Sentry: %v\n", err)

		return &sentryClient{}
	}

	gosentry.ConfigureScope(func(scope *gosentry.Scope) {
		scope.SetTag(TagEnv, cfg.AppEnv)
		scope.SetTag(TagOS, runtime.GOOS)
		scope.SetTag(TagArch, runtime.GOARCH)
		scope.SetTag(TagGoVersion, runtime.Version())

		if id := loadTelemetryID(); id != "" {
			scope.SetUser(gosentry.User{ID: id})
		}
	})

	return &sentryClient{}
}

// Flush waits for pending Sentry events to be sent
func (s *sentryClient) Flush() {
	gosentry.Flush(2 * time.Second)
}

// stripPII removes or obfuscates sensitive information from a Sentry event before it's sent
func stripPII(event *gosentry.Event, _ *gosentry.EventHint) *gosentry.Event {
	event.ServerName = ""
	event.User = gosentry.User{ID: event.User.ID}

	return event
}
