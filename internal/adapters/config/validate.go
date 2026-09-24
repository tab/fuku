package config

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"fuku/internal/model"
)

// validate validates the configuration
func (c *Config) validate() error {
	if err := c.validateConcurrency(); err != nil {
		return err
	}

	if err := c.validateRetry(); err != nil {
		return err
	}

	if err := c.validateLogs(); err != nil {
		return err
	}

	if err := c.validateServer(); err != nil {
		return err
	}

	if err := c.validateProfiles(); err != nil {
		return err
	}

	for name, service := range c.Services {
		if service == nil {
			return fmt.Errorf("service %s: %w", name, ErrEmptyService)
		}

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

// validateProfiles ensures every service referenced by a profile exists in the service catalog
func (c *Config) validateProfiles() error {
	for profile, value := range c.Profiles {
		if err := c.validateProfileEntry(profile, value); err != nil {
			return err
		}
	}

	return nil
}

// validateProfileEntry validates a single profile value against the service catalog
func (c *Config) validateProfileEntry(profile string, value any) error {
	switch v := value.(type) {
	case string:
		if v == "*" {
			return nil
		}

		return c.checkProfileService(profile, v)
	case []any:
		for _, item := range v {
			name, ok := item.(string)
			if !ok {
				return fmt.Errorf("%w: profile '%s' contains non-string entry", ErrUnsupportedProfileFormat, profile)
			}

			if err := c.checkProfileService(profile, name); err != nil {
				return err
			}
		}

		return nil
	default:
		return fmt.Errorf("%w: %s", ErrUnsupportedProfileFormat, profile)
	}
}

// checkProfileService verifies a single referenced service exists in the catalog
func (c *Config) checkProfileService(profile, name string) error {
	if _, exists := c.Services[name]; exists {
		return nil
	}

	return fmt.Errorf("%w: profile '%s' references undefined service '%s'", ErrProfileReferenceUndefined, profile, name)
}

// validateConcurrency validates concurrency settings
func (c *Config) validateConcurrency() error {
	if c.Concurrency.Workers <= 0 {
		return ErrInvalidConcurrencyWorkers
	}

	return nil
}

// validateRetry validates retry settings
func (c *Config) validateRetry() error {
	if c.Retry.Attempts <= 0 {
		return ErrInvalidRetryAttempts
	}

	if c.Retry.Backoff < 0 {
		return ErrInvalidRetryBackoff
	}

	return nil
}

// validateLogs validates logs settings
func (c *Config) validateLogs() error {
	if c.Logs.Buffer <= 0 {
		return ErrInvalidLogsBuffer
	}

	if c.Logs.History <= 0 {
		return ErrInvalidLogsHistory
	}

	return nil
}

// validateServer validates the built-in HTTP API server configuration
func (c *Config) validateServer() error {
	if c.Server.Listen == "" {
		return nil
	}

	if c.Server.Auth.Token == "" {
		return ErrAPITokenRequired
	}

	host, portStr, err := net.SplitHostPort(c.Server.Listen)
	if err != nil || host == "" {
		return ErrAPIInvalidListen
	}

	port, err := strconv.Atoi(portStr)
	if err != nil || port < 1 || port > 65535 {
		return ErrAPIInvalidListen
	}

	if !isLoopback(host) {
		return ErrAPINotLoopback
	}

	return nil
}

// isLoopback checks whether a host is a loopback IP literal or known loopback hostname
func isLoopback(host string) bool {
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback()
	}

	return host == LoopbackHostname || host == LoopbackIPv6Hostname
}

// validateCommand validates the command configuration
func (s *Service) validateCommand() error {
	if s.Command != "" && strings.TrimSpace(s.Command) == "" {
		return ErrInvalidCommand
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
	case model.ReadinessHTTP:
		if r.URL == "" {
			return ErrReadinessURLRequired
		}
	case model.ReadinessTCP:
		if r.Address == "" {
			return ErrReadinessAddressRequired
		}
	case model.ReadinessLog:
		if r.Pattern == "" {
			return ErrReadinessPatternRequired
		}
	case "":
		return ErrReadinessTypeRequired
	default:
		return fmt.Errorf("%w: '%s' (must be 'http', 'tcp', or 'log')", ErrInvalidReadinessType, r.Type)
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
			return fmt.Errorf("%w: '%s'", ErrInvalidLogsOutput, output)
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
		return ErrWatchIncludeRequired
	}

	return nil
}
