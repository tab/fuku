package config

import (
	"fmt"

	"github.com/spf13/viper"

	"fuku/internal/app/errors"
)

const (
	DefaultProfile = "default"

	DefaultLogLevel  = "info"
	DefaultLogFormat = "console"

	AppName        = "Fuku"
	AppDescription = "Lightweight CLI orchestrator for managing local services"

	Version = "0.2.0"
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
	Dir       string          `yaml:"dir"`
	DependsOn []string        `yaml:"depends_on"`
	Profiles  []string        `yaml:"profiles"`
	Readiness *ReadinessCheck `yaml:"readiness,omitempty"`
}

// ReadinessCheck defines how to check if a service is ready
type ReadinessCheck struct {
	Type     string `yaml:"type"`               // http, log, port, tcp
	URL      string `yaml:"url,omitempty"`      // for http type
	Pattern  string `yaml:"pattern,omitempty"`  // for log type
	Port     int    `yaml:"port,omitempty"`     // for port/tcp type
	Timeout  string `yaml:"timeout,omitempty"`  // e.g., "30s"
	Interval string `yaml:"interval,omitempty"` // e.g., "500ms"
}

// ServiceDefaults represents default configuration for services
type ServiceDefaults struct {
	DependsOn []string `yaml:"depends_on"`
	Profiles  []string `yaml:"profiles"`
	Exclude   []string `yaml:"exclude"`
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

	return cfg, nil
}

// ApplyDefaults applies default configuration to services
func (c *Config) ApplyDefaults() {
	if c.Defaults == nil {
		return
	}

	excludeMap := make(map[string]bool)
	for _, svc := range c.Defaults.Exclude {
		excludeMap[svc] = true
	}

	for name, service := range c.Services {
		if excludeMap[name] {
			continue
		}

		if len(service.DependsOn) == 0 && len(c.Defaults.DependsOn) > 0 {
			service.DependsOn = make([]string, len(c.Defaults.DependsOn))
			copy(service.DependsOn, c.Defaults.DependsOn)
		}

		if len(service.Profiles) == 0 && len(c.Defaults.Profiles) > 0 {
			service.Profiles = make([]string, len(c.Defaults.Profiles))
			copy(service.Profiles, c.Defaults.Profiles)
		}

		if service.Dir == "" {
			service.Dir = name
		}
	}
}

// GetServicesForProfile returns the list of services for a given profile
func (c *Config) GetServicesForProfile(profile string) ([]string, error) {
	profileConfig, exists := c.Profiles[profile]
	if !exists {
		return nil, fmt.Errorf("profile %s not found", profile)
	}

	switch v := profileConfig.(type) {
	case string:
		if v == "*" {
			var allServices []string
			for name := range c.Services {
				allServices = append(allServices, name)
			}
			return allServices, nil
		}
		return []string{v}, nil
	case []interface{}:
		var services []string
		for _, item := range v {
			if str, ok := item.(string); ok {
				services = append(services, str)
			}
		}
		return services, nil
	default:
		return nil, fmt.Errorf("unsupported profile format for %s", profile)
	}
}
