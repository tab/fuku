package config

import (
	"fmt"
	"time"

	"github.com/spf13/viper"

	"fuku/internal/app/errors"
)

// Config represents the application configuration
type Config struct {
	Services map[string]*Service    `yaml:"services"`
	Defaults *ServiceDefaults       `yaml:"defaults"`
	Profiles map[string]interface{} `yaml:"profiles"`
	Logging  struct {
		Level  string `yaml:"level"`
		Format string `yaml:"format"`
	}
	Version int
}

// Service represents a service configuration
type Service struct {
	Dir       string     `yaml:"dir"`
	Profiles  []string   `yaml:"profiles"`
	Tier      string     `yaml:"tier"`
	Readiness *Readiness `yaml:"readiness"`
}

// Readiness represents readiness check configuration for a service
type Readiness struct {
	Type     string        `yaml:"type"`
	URL      string        `yaml:"url"`
	Pattern  string        `yaml:"pattern"`
	Timeout  time.Duration `yaml:"timeout"`
	Interval time.Duration `yaml:"interval"`
}

// ServiceDefaults represents default configuration for services
type ServiceDefaults struct {
	Profiles []string `yaml:"profiles"`
	Tier     string   `yaml:"tier"`
}

// Option allows for functional options pattern
type Option func(*Config)

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	cfg := &Config{
		Version:  1,
		Services: make(map[string]*Service),
		Profiles: make(map[string]interface{}),
	}

	cfg.Logging.Level = DefaultLogLevel
	cfg.Logging.Format = DefaultLogFormat

	return cfg
}

// Load loads the configuration from file
func Load() (*Config, error) {
	cfg := DefaultConfig()

	v := viper.New()
	v.SetConfigName("fuku")
	v.SetConfigType("yaml")
	v.AddConfigPath(".")

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, errors.ErrFailedToReadConfig
		}
	} else {
		if err = v.Unmarshal(cfg); err != nil {
			return nil, errors.ErrFailedToParseConfig
		}
	}

	cfg.ApplyDefaults()

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %w", errors.ErrInvalidConfig, err)
	}

	return cfg, nil
}

// ApplyDefaults applies default configuration to services
func (c *Config) ApplyDefaults() {
	for name, service := range c.Services {
		if service.Dir == "" {
			service.Dir = name
		}

		if c.Defaults != nil {
			if len(service.Profiles) == 0 && len(c.Defaults.Profiles) > 0 {
				service.Profiles = make([]string, len(c.Defaults.Profiles))
				copy(service.Profiles, c.Defaults.Profiles)
			}

			if service.Tier == "" && c.Defaults.Tier != "" {
				service.Tier = c.Defaults.Tier
			}
		}
	}
}

// Validate validates the configuration
func (c *Config) Validate() error {
	for name, service := range c.Services {
		if err := service.validateTier(); err != nil {
			return fmt.Errorf("service %s: %w", name, err)
		}

		if err := service.validateReadiness(); err != nil {
			return fmt.Errorf("service %s: %w", name, err)
		}
	}

	return nil
}

// validateTier validates the tier value
func (s *Service) validateTier() error {
	if s.Tier == "" {
		return nil
	}

	switch s.Tier {
	case Foundation, Platform, Edge:
		return nil
	default:
		return fmt.Errorf("%w: '%s' (must be one of foundation, platform, or edge)", errors.ErrInvalidTier, s.Tier)
	}
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
	case TypeLog:
		if r.Pattern == "" {
			return errors.ErrReadinessPatternRequired
		}
	case "":
		return errors.ErrReadinessTypeRequired
	default:
		return fmt.Errorf("%w: '%s' (must be 'http' or 'log')", errors.ErrInvalidReadinessType, r.Type)
	}

	if r.Timeout == 0 {
		r.Timeout = DefaultTimeout
	}

	if r.Interval == 0 {
		r.Interval = DefaultInterval
	}

	return nil
}
