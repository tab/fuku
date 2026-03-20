package config

import (
	"strings"
	"time"
)

// Config represents the application configuration
type Config struct {
	AppEnv    string
	SentryDSN string
	Telemetry bool
	Services  map[string]*Service `yaml:"services"`
	Defaults  *ServiceDefaults    `yaml:"defaults"`
	Profiles  map[string]any      `yaml:"profiles"`
	Logging   struct {
		Level  string `yaml:"level"`
		Format string `yaml:"format"`
	}
	Concurrency struct {
		Workers int `yaml:"workers"`
	}
	Retry struct {
		Attempts int           `yaml:"attempts"`
		Backoff  time.Duration `yaml:"backoff"`
	}
	Logs struct {
		Buffer  int `yaml:"buffer"`
		History int `yaml:"history"`
	}
	Version int
}

// Service represents a service configuration
type Service struct {
	Dir       string     `yaml:"dir"`
	Command   string     `yaml:"command"`
	Profiles  []string   `yaml:"profiles"`
	Tier      string     `yaml:"tier"`
	Readiness *Readiness `yaml:"readiness"`
	Logs      *Logs      `yaml:"logs"`
	Watch     *Watch     `yaml:"watch"`
}

// Readiness represents readiness check configuration for a service
type Readiness struct {
	Type     string        `yaml:"type"`
	Address  string        `yaml:"address"`
	URL      string        `yaml:"url"`
	Pattern  string        `yaml:"pattern"`
	Timeout  time.Duration `yaml:"timeout"`
	Interval time.Duration `yaml:"interval"`
}

// Watch represents file watch configuration for hot-reload
type Watch struct {
	Include  []string      `yaml:"include"`
	Ignore   []string      `yaml:"ignore"`
	Shared   []string      `yaml:"shared"`
	Debounce time.Duration `yaml:"debounce"`
}

// Logs represents per-service console logging configuration
type Logs struct {
	Output []string `yaml:"output"`
}

// ServiceDefaults represents default configuration for services
type ServiceDefaults struct {
	Profiles []string `yaml:"profiles"`
	Tier     string   `yaml:"tier"`
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	cfg := &Config{
		Services: make(map[string]*Service),
		Profiles: make(map[string]any),
		Version:  1,
	}

	cfg.Logging.Level = LogLevel
	cfg.Logging.Format = LogFormat

	cfg.Concurrency.Workers = MaxWorkers

	cfg.Retry.Attempts = RetryAttempts
	cfg.Retry.Backoff = RetryBackoff

	cfg.Logs.Buffer = SocketLogsBufferSize
	cfg.Logs.History = SocketLogsHistorySize

	cfg.Profiles[Default] = "*"

	return cfg
}

// ApplyDefaults applies default configuration to services
func (c *Config) ApplyDefaults() {
	for name, service := range c.Services {
		if service.Dir == "" {
			service.Dir = name
		}

		if c.Defaults == nil {
			continue
		}

		if len(service.Profiles) == 0 && len(c.Defaults.Profiles) > 0 {
			service.Profiles = make([]string, len(c.Defaults.Profiles))
			copy(service.Profiles, c.Defaults.Profiles)
		}

		if service.Tier == "" && c.Defaults.Tier != "" {
			service.Tier = c.Defaults.Tier
		}
	}
}

// TelemetryEnabled reports whether telemetry is active (opted in and DSN configured)
func (c *Config) TelemetryEnabled() bool {
	return c.Telemetry && c.SentryDSN != ""
}

// TelemetryDisabled reports whether telemetry is inactive (opted out or DSN missing)
func (c *Config) TelemetryDisabled() bool {
	return !c.TelemetryEnabled()
}

// normalizeTiers normalizes tier names in services to match parsed values
func (c *Config) normalizeTiers() {
	for _, service := range c.Services {
		service.Tier = normalizeTier(service.Tier)
	}

	if c.Defaults != nil {
		c.Defaults.Tier = normalizeTier(c.Defaults.Tier)
	}
}

// normalizeTier trims whitespace and lowercases a tier name
func normalizeTier(tier string) string {
	return strings.ToLower(strings.TrimSpace(tier))
}
