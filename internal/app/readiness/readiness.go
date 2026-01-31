package readiness

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"time"

	"fuku/internal/app/errors"
	"fuku/internal/app/process"
	"fuku/internal/config"
	"fuku/internal/config/logger"
)

// Readiness handles service readiness checking
type Readiness interface {
	CheckHTTP(ctx context.Context, url string, timeout, interval time.Duration, done <-chan struct{}) error
	CheckLog(ctx context.Context, pattern string, stdout, stderr *io.PipeReader, timeout time.Duration, done <-chan struct{}) error
	Check(ctx context.Context, name string, service *config.Service, proc process.Process)
}

// readiness implements the Readiness interface
type readiness struct {
	log logger.Logger
}

// New creates a new readiness checker instance
func New(log logger.Logger) Readiness {
	return &readiness{
		log: log.WithComponent("READINESS"),
	}
}

// CheckHTTP checks if an HTTP endpoint is ready
func (r *readiness) CheckHTTP(ctx context.Context, url string, timeout, interval time.Duration, done <-chan struct{}) error {
	client := &http.Client{Timeout: interval}
	deadline := time.Now().Add(timeout)

	reqCtx, cancel := r.contextWithDone(ctx, done)
	defer cancel()

	for {
		if time.Now().After(deadline) {
			return fmt.Errorf("%w: HTTP check after %v", errors.ErrReadinessTimeout, timeout)
		}

		req, err := http.NewRequestWithContext(reqCtx, "GET", url, nil)
		if err != nil {
			return fmt.Errorf("%w: %w", errors.ErrFailedToCreateRequest, err)
		}

		resp, err := client.Do(req)
		if err == nil && resp.StatusCode >= 200 && resp.StatusCode < 300 {
			resp.Body.Close()
			return nil
		}

		if resp != nil {
			resp.Body.Close()
		}

		select {
		case <-reqCtx.Done():
			if r.isDone(done) {
				return errors.ErrProcessExited
			}

			return reqCtx.Err()
		case <-time.After(interval):
		}
	}
}

// CheckLog checks if a log pattern appears in stdout/stderr
func (r *readiness) CheckLog(ctx context.Context, pattern string, stdout, stderr *io.PipeReader, timeout time.Duration, done <-chan struct{}) error {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return fmt.Errorf("%w: %w", errors.ErrInvalidRegexPattern, err)
	}

	matched := make(chan struct{}, 1)
	deadline := time.Now().Add(timeout)

	scanStream := func(reader *io.PipeReader) {
		scanner := bufio.NewScanner(reader)
		for scanner.Scan() {
			if re.MatchString(scanner.Text()) {
				select {
				case matched <- struct{}{}:
				default:
				}

				return
			}
		}
	}

	go scanStream(stdout)
	go scanStream(stderr)

	reqCtx, cancel := r.contextWithDone(ctx, done)
	defer cancel()

	duration := time.Until(deadline)
	if duration < 0 {
		duration = 0
	}

	select {
	case <-matched:
		return nil
	case <-reqCtx.Done():
		if r.isDone(done) {
			return errors.ErrProcessExited
		}

		return reqCtx.Err()
	case <-time.After(duration):
		return fmt.Errorf("%w: log pattern check after %v", errors.ErrReadinessTimeout, timeout)
	}
}

// Check performs the appropriate readiness check for a service
func (r *readiness) Check(ctx context.Context, name string, service *config.Service, proc process.Process) {
	options := service.Readiness
	r.log.Info().Msgf("Starting %s readiness check for service '%s'", options.Type, name)

	var err error

	done := proc.Done()

	switch options.Type {
	case config.TypeHTTP:
		err = r.CheckHTTP(ctx, options.URL, options.Timeout, options.Interval, done)
	case config.TypeLog:
		err = r.CheckLog(ctx, options.Pattern, proc.StdoutReader(), proc.StderrReader(), options.Timeout, done)
	default:
		err = fmt.Errorf("%w: %s", errors.ErrInvalidReadinessType, options.Type)
	}

	if err != nil {
		r.log.Error().Err(err).Msgf("Readiness check failed for service '%s'", name)
	} else {
		r.log.Info().Msgf("Service '%s' is ready", name)
	}

	proc.SignalReady(err)
}

// contextWithDone creates a context that cancels when either ctx is cancelled or done is closed
func (r *readiness) contextWithDone(ctx context.Context, done <-chan struct{}) (context.Context, context.CancelFunc) {
	newCtx, cancel := context.WithCancel(ctx)
	stopped := make(chan struct{})

	go func() {
		select {
		case <-done:
			cancel()
		case <-newCtx.Done():
		}

		close(stopped)
	}()

	return newCtx, func() {
		cancel()
		<-stopped
	}
}

// isDone checks if the done channel is closed
func (r *readiness) isDone(done <-chan struct{}) bool {
	select {
	case <-done:
		return true
	default:
		return false
	}
}
