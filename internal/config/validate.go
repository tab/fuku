package config

import (
	"fmt"
	"strings"

	"fuku/internal/app/errors"
)

// Validate validates the configuration
func (c *Config) Validate() error {
	if err := c.validateConcurrency(); err != nil {
		return err
	}

	if err := c.validateRetry(); err != nil {
		return err
	}

	if err := c.validateLogs(); err != nil {
		return err
	}

	for name, service := range c.Services {
		if err := service.validateCommand(); err != nil {
			return fmt.Errorf("service %s: %w", name, err)
		}

		if err := service.validateReadiness(); err != nil {
			return fmt.Errorf("service %s: %w", name, err)
		}

		if err := service.validateLogs(); err != nil {
			return fmt.Errorf("service %s: %w", name, err)
		}

		if err := service.validateWatch(); err != nil {
			return fmt.Errorf("service %s: %w", name, err)
		}
	}

	return nil
}

// validateConcurrency validates concurrency settings
func (c *Config) validateConcurrency() error {
	if c.Concurrency.Workers <= 0 {
		return errors.ErrInvalidConcurrencyWorkers
	}

	return nil
}

// validateRetry validates retry settings
func (c *Config) validateRetry() error {
	if c.Retry.Attempts <= 0 {
		return errors.ErrInvalidRetryAttempts
	}

	if c.Retry.Backoff < 0 {
		return errors.ErrInvalidRetryBackoff
	}

	return nil
}

// validateLogs validates logs settings
func (c *Config) validateLogs() error {
	if c.Logs.Buffer <= 0 {
		return errors.ErrInvalidLogsBuffer
	}

	return nil
}

// validateCommand validates the command configuration
func (s *Service) validateCommand() error {
	if s.Command != "" && strings.TrimSpace(s.Command) == "" {
		return errors.ErrInvalidCommand
	}

	return nil
}

// validateReadiness validates the readiness configuration
func (s *Service) validateReadiness() error {
	if s.Readiness == nil {
		return nil
	}

	r := s.Readiness

	switch r.Type {
	case TypeHTTP:
		if r.URL == "" {
			return errors.ErrReadinessURLRequired
		}
	case TypeTCP:
		if r.Address == "" {
			return errors.ErrReadinessAddressRequired
		}
	case TypeLog:
		if r.Pattern == "" {
			return errors.ErrReadinessPatternRequired
		}
	case "":
		return errors.ErrReadinessTypeRequired
	default:
		return fmt.Errorf("%w: '%s' (must be 'http', 'tcp', or 'log')", errors.ErrInvalidReadinessType, r.Type)
	}

	if r.Timeout == 0 {
		r.Timeout = DefaultTimeout
	}

	if r.Interval == 0 {
		r.Interval = DefaultInterval
	}

	return nil
}

// validateLogs validates the service logs configuration
func (s *Service) validateLogs() error {
	if s.Logs == nil {
		return nil
	}

	for _, output := range s.Logs.Output {
		switch strings.ToLower(output) {
		case "stdout", "stderr":
		default:
			return fmt.Errorf("%w: '%s'", errors.ErrInvalidLogsOutput, output)
		}
	}

	return nil
}

// validateWatch validates the watch configuration
func (s *Service) validateWatch() error {
	if s.Watch == nil {
		return nil
	}

	if len(s.Watch.Include) == 0 {
		return errors.ErrWatchIncludeRequired
	}

	return nil
}
