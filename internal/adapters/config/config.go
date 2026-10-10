package config

import (
	"strings"
	"time"

	"fuku/internal/model"
)

// Config represents the application configuration
type Config struct {
	AppEnv      string              `mapstructure:"-"`
	SentryDSN   string              `mapstructure:"-"`
	Telemetry   bool                `mapstructure:"-"`
	Services    map[string]*Service `yaml:"services"`
	Defaults    *ServiceDefaults    `yaml:"defaults"`
	Profiles    map[string]any      `yaml:"profiles"`
	Exclude     []string            `yaml:"exclude"`
	Logging     Logging             `yaml:"logging"`
	Concurrency Concurrency         `yaml:"concurrency"`
	Retry       Retry               `yaml:"retry"`
	Logs        LogStream           `yaml:"logs"`
	Server      Server              `yaml:"server"`
	API         bool                `mapstructure:"-"`
	Updater     bool                `mapstructure:"-"`
}

// defaultConfig returns the default configuration
func defaultConfig() *Config {
	cfg := &Config{
		Services: make(map[string]*Service),
		Profiles: make(map[string]any),
	}

	cfg.Logging.Level = DefaultLogLevel
	cfg.Logging.Format = DefaultLogFormat

	cfg.Concurrency.Workers = MaxWorkers

	cfg.Retry.Attempts = RetryAttempts
	cfg.Retry.Backoff = RetryBackoff

	cfg.Logs.Buffer = SocketLogsBufferSize
	cfg.Logs.History = SocketLogsHistorySize

	cfg.Server.Listen = DefaultAPIListen

	cfg.Profiles[model.ProfileDefault] = "*"

	return cfg
}

// applyDefaults applies default configuration to services
func (c *Config) applyDefaults() {
	defaultTier := ""
	if c.Defaults != nil {
		defaultTier = c.Defaults.Tier
	}

	for name, service := range c.Services {
		if service.Dir == "" {
			service.Dir = name
		}

		service.applyReadinessDefaults()

		service.Tier = serviceTier(service.Tier, defaultTier)
	}
}

// applyReadinessDefaults fills the readiness timeout and interval a service leaves unset
func (s *Service) applyReadinessDefaults() {
	if s.Readiness == nil {
		return
	}

	if s.Readiness.Timeout == 0 {
		s.Readiness.Timeout = DefaultTimeout
	}

	if s.Readiness.Interval == 0 {
		s.Readiness.Interval = DefaultInterval
	}
}

// serviceTier returns the normalized tier of a service, else the normalized defaults tier (empty when both are unset)
func serviceTier(tier, defaultTier string) string {
	if normalized := normalizeTier(tier); normalized != "" {
		return normalized
	}

	return normalizeTier(defaultTier)
}

// normalizeTier trims whitespace and lowercases a tier name
func normalizeTier(tier string) string {
	return strings.ToLower(strings.TrimSpace(tier))
}

// normalizeExclude trims whitespace, drops empty entries, and deduplicates the exclude list preserving order
func (c *Config) normalizeExclude() {
	if len(c.Exclude) == 0 {
		return
	}

	seen := make(map[string]bool, len(c.Exclude))
	result := make([]string, 0, len(c.Exclude))

	for _, name := range c.Exclude {
		trimmed := strings.TrimSpace(name)

		if trimmed == "" {
			continue
		}

		if seen[trimmed] {
			continue
		}

		seen[trimmed] = true

		result = append(result, trimmed)
	}

	c.Exclude = result
}

// Service represents a service configuration
type Service struct {
	Dir       string     `yaml:"dir"`
	Command   string     `yaml:"command"`
	Tier      string     `yaml:"tier"`
	Readiness *Readiness `yaml:"readiness"`
	Logs      *Logs      `yaml:"logs"`
	Watch     *Watch     `yaml:"watch"`
	Env       *Env       `yaml:"env"`
}

// Env declares the .env files shown in the UI's env tab (the values are never exported to the service process)
type Env struct {
	Files []string `yaml:"files"`
}

// Readiness represents readiness check configuration for a service
type Readiness struct {
	Type     model.ReadinessType `yaml:"type"`
	Address  string              `yaml:"address"`
	URL      string              `yaml:"url"`
	Pattern  string              `yaml:"pattern"`
	Timeout  time.Duration       `yaml:"timeout"`
	Interval time.Duration       `yaml:"interval"`
}

// Logs represents per-service console logging configuration
type Logs struct {
	Output []string `yaml:"output"`
}

// Watch represents file watch configuration for hot-reload
type Watch struct {
	Include  []string      `yaml:"include"`
	Ignore   []string      `yaml:"ignore"`
	Shared   []string      `yaml:"shared"`
	Debounce time.Duration `yaml:"debounce"`
}

// ServiceDefaults represents default configuration for services
type ServiceDefaults struct {
	Tier string `yaml:"tier"`
}

// Logging represents logging configuration
type Logging struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}

// Concurrency represents concurrency settings
type Concurrency struct {
	Workers int `yaml:"workers"`
}

// Retry represents retry settings
type Retry struct {
	Attempts int           `yaml:"attempts"`
	Backoff  time.Duration `yaml:"backoff"`
}

// LogStream represents log streaming configuration
type LogStream struct {
	Buffer  int `yaml:"buffer"`
	History int `yaml:"history"`
}

// Server represents the built-in API server configuration
type Server struct {
	Listen string `yaml:"listen"`
	Auth   struct {
		Token string `yaml:"token"`
	} `yaml:"auth"`
}
