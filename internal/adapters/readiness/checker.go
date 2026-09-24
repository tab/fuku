package readiness

import (
	"context"
	"fmt"
	"time"

	"fuku/internal/contracts"
	"fuku/internal/model"
)

// Logger is the logging surface the readiness checker writes through
type Logger interface {
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
}

// Checker probes a service for readiness the way its config describes
type Checker struct {
	publisher contracts.Publisher
	log       Logger
}

// NewChecker creates a new readiness checker
func NewChecker(publisher contracts.Publisher, log Logger) *Checker {
	return &Checker{
		publisher: publisher,
		log:       log,
	}
}

// Check runs the readiness probe against the service's process and returns why it failed
func (c *Checker) Check(ctx context.Context, readiness model.Readiness, proc contracts.Process) error {
	startTime := time.Now()
	svc := proc.Service()

	c.log.Info(fmt.Sprintf("Starting %s readiness check for service '%s'", readiness.Type, svc.Name))

	var err error

	done := proc.Done()

	switch readiness.Type {
	case model.ReadinessHTTP:
		err = c.checkHTTP(ctx, readiness.URL, readiness.Timeout, readiness.Interval, done)
	case model.ReadinessTCP:
		err = c.checkTCP(ctx, readiness.Address, readiness.Timeout, readiness.Interval, done)
	case model.ReadinessLog:
		err = c.checkLog(ctx, readiness.Pattern, proc.Stdout(), proc.Stderr(), readiness.Timeout, done)
	}

	if err != nil {
		c.log.Error(fmt.Sprintf("Readiness check failed for service '%s'", svc.Name), "error", err)

		return err
	}

	c.log.Info(fmt.Sprintf("Service '%s' is ready", svc.Name))
	c.publishComplete(svc, readiness.Type, time.Since(startTime))

	return nil
}

// contextWithDone creates a context that cancels when either ctx is cancelled or done is closed
func (c *Checker) contextWithDone(ctx context.Context, done <-chan struct{}) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(ctx)
	stopped := make(chan struct{})

	go func() {
		select {
		case <-done:
			cancel()
		case <-ctx.Done():
		}

		close(stopped)
	}()

	return ctx, func() {
		cancel()
		<-stopped
	}
}

// isDone checks if the done channel is closed
func (c *Checker) isDone(done <-chan struct{}) bool {
	select {
	case <-done:
		return true
	default:
		return false
	}
}
