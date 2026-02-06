package services

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/rs/zerolog"
)

// Runner defines the interface for test services
type Runner interface {
	Run()
}

// Service contains common service functionality
type Service struct {
	Name string
	Log  zerolog.Logger
}

// newService creates a service with logging
func newService(name string) Service {
	return Service{
		Name: name,
		Log: zerolog.New(zerolog.ConsoleWriter{Out: os.Stdout}).
			With().
			Timestamp().
			Str("service", name).
			Logger(),
	}
}

// WaitForShutdown blocks until SIGTERM or SIGINT is received
func (s *Service) WaitForShutdown(ctx context.Context) {
	sigCh := make(chan os.Signal, 1)

	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	defer signal.Stop(sigCh)

	select {
	case <-sigCh:
	case <-ctx.Done():
	}
}
