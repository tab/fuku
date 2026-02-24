package sentry

import (
	"fmt"
	"os"
	"runtime"
	"time"

	gosentry "github.com/getsentry/sentry-go"
	"go.uber.org/fx"

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
	if cfg.SentryDSN == "" {
		return &sentryClient{}
	}

	err := gosentry.Init(gosentry.ClientOptions{
		Dsn:              cfg.SentryDSN,
		Environment:      cfg.AppEnv,
		Release:          config.Version,
		AttachStacktrace: true,
		SampleRate:       1.0,
		SendDefaultPII:   false,
		BeforeSend: func(event *gosentry.Event, hint *gosentry.EventHint) *gosentry.Event {
			event.ServerName = ""
			event.User = gosentry.User{}

			return event
		},
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
	})

	return &sentryClient{}
}

// Flush waits for pending Sentry events to be sent
func (s *sentryClient) Flush() {
	gosentry.Flush(2 * time.Second)
}

// Module provides the fx dependency injection options for the sentry package
var Module = fx.Options(
	fx.Provide(NewSentry),
)
