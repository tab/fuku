package config

import (
	"github.com/spf13/viper"

	"fuku/internal/app/errors"
)

const (
	DefaultLogLevel  = "info"
	DefaultLogFormat = "console"

	Version = "0.1.0"
)

// Config represents the application configuration
type Config struct {
	Services map[string]*Service `yaml:"services"`
	Scopes   map[string]*Scope   `yaml:"scopes"`
	Logging  struct {
		Level  string `yaml:"level"`
		Format string `yaml:"format"`
	}
	Version int
}

// Service represents a service configuration
type Service struct {
	Dir       string   `yaml:"dir"`
	DependsOn []string `yaml:"depends_on"`
}

// Scope represents a scope to run services in
type Scope struct {
	Include []string `yaml:"include"`
}

// Option allows for functional options pattern
type Option func(*Config)

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	cfg := &Config{
		Version:  1,
		Services: make(map[string]*Service),
		Scopes:   make(map[string]*Scope),
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

	return cfg, nil
}
