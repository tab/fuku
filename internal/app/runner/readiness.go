package runner

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"time"

	"fuku/internal/app/errors"
	"fuku/internal/config"
	"fuku/internal/config/logger"
)

// Readiness handles service readiness checking
type Readiness interface {
	CheckHTTP(ctx context.Context, url string, timeout, interval time.Duration) error
	CheckLog(ctx context.Context, pattern string, stdout, stderr *io.PipeReader, timeout time.Duration) error
	Check(ctx context.Context, name string, service *config.Service, process Process)
}

type readiness struct {
	log logger.Logger
}

// NewReadiness creates a new readiness checker instance
func NewReadiness(log logger.Logger) Readiness {
	return &readiness{
		log: log,
	}
}

// CheckHTTP checks if an HTTP endpoint is ready
func (r *readiness) CheckHTTP(ctx context.Context, url string, timeout, interval time.Duration) error {
	client := &http.Client{Timeout: interval}
	deadline := time.Now().Add(timeout)

	for {
		if time.Now().After(deadline) {
			return fmt.Errorf("%w: HTTP check after %v", errors.ErrReadinessTimeout, timeout)
		}

		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
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
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(interval):
		}
	}
}

// CheckLog checks if a log pattern appears in stdout/stderr
func (r *readiness) CheckLog(ctx context.Context, pattern string, stdout, stderr *io.PipeReader, timeout time.Duration) error {
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

	duration := time.Until(deadline)
	if duration < 0 {
		duration = 0
	}

	select {
	case <-matched:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(duration):
		return fmt.Errorf("%w: log pattern check after %v", errors.ErrReadinessTimeout, timeout)
	}
}

// Check performs the appropriate readiness check for a service
func (r *readiness) Check(ctx context.Context, name string, service *config.Service, process Process) {
	options := service.Readiness
	r.log.Info().Msgf("Starting %s readiness check for service '%s'", options.Type, name)

	var err error
	switch options.Type {
	case config.TypeHTTP:
		err = r.CheckHTTP(ctx, options.URL, options.Timeout, options.Interval)
	case config.TypeLog:
		err = r.CheckLog(ctx, options.Pattern, process.StdoutReader(), process.StderrReader(), options.Timeout)
	default:
		err = fmt.Errorf("%w: %s", errors.ErrInvalidReadinessType, options.Type)
	}

	if err != nil {
		r.log.Error().Err(err).Msgf("Readiness check failed for service '%s'", name)
	} else {
		r.log.Info().Msgf("Service '%s' is ready", name)
	}

	process.SignalReady(err)
}
