package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"

	"fuku/internal/app/errors"
)

// Load loads configuration from the default config file with optional override merging
func Load() (*Config, *Topology, error) {
	cfg := initConfig()

	filePath, err := resolveDefaultConfig()
	if err != nil {
		return nil, nil, err
	}

	if filePath == "" {
		return cfg, DefaultTopology(), nil
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: %w", errors.ErrFailedToReadConfig, err)
	}

	data, err = applyOverride(filePath, data)
	if err != nil {
		return nil, nil, err
	}

	return parseConfig(cfg, data)
}

// LoadFromFile loads configuration from an explicit file path without override merging
func LoadFromFile(path string) (*Config, *Topology, error) {
	cfg := initConfig()

	filePath, err := resolveExplicitConfig(path)
	if err != nil {
		return nil, nil, err
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: %w", errors.ErrFailedToReadConfig, err)
	}

	return parseConfig(cfg, data)
}

// LoadEnv loads environment variables from .env files in priority order.
// Files loaded first take precedence (godotenv does not override existing vars):
//
//	.env.<GO_ENV>.local  (highest priority)
//	.env.<GO_ENV>        (environment-specific)
//	.env                 (lowest priority)
func LoadEnv() {
	goEnv := os.Getenv("GO_ENV")
	if goEnv != "" {
		_ = godotenv.Load(".env." + goEnv + ".local")
		_ = godotenv.Load(".env." + goEnv)
	}

	_ = godotenv.Load()
}

// ResolveEnv returns the current environment name from GO_ENV, defaulting to EnvProduction
func ResolveEnv() string {
	env := os.Getenv("GO_ENV")
	if env == "" {
		return EnvProduction
	}

	return env
}

// initConfig creates a default config populated with environment values
func initConfig() *Config {
	LoadEnv()

	cfg := DefaultConfig()
	cfg.AppEnv = ResolveEnv()
	cfg.SentryDSN = os.Getenv("SENTRY_DSN")
	cfg.Telemetry = os.Getenv("FUKU_TELEMETRY_DISABLED") != "1"

	return cfg
}

// resolveDefaultConfig tries fuku.yaml then fuku.yml in the current directory
func resolveDefaultConfig() (string, error) {
	for _, candidate := range []string{ConfigFile, ConfigFileAlt} {
		exists, err := fileExists(candidate)
		if err != nil {
			return "", fmt.Errorf("%w: %w", errors.ErrFailedToReadConfig, err)
		}

		if exists {
			return candidate, nil
		}
	}

	return "", nil
}

// applyOverride resolves an override file next to basePath and merges it into data
func applyOverride(basePath string, data []byte) ([]byte, error) {
	overridePath, err := resolveOverrideFile(basePath)
	if err != nil {
		return nil, err
	}

	if overridePath == "" {
		return data, nil
	}

	overrideData, err := os.ReadFile(overridePath)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errors.ErrFailedToReadConfig, err)
	}

	merged, err := mergeYAML(data, overrideData)
	if err != nil {
		return nil, err
	}

	return merged, nil
}

// resolveOverrideFile finds an override config file in the same directory as the base config
func resolveOverrideFile(basePath string) (string, error) {
	dir := filepath.Dir(basePath)

	for _, candidate := range []string{OverrideConfigFile, OverrideConfigFileAlt} {
		path := filepath.Join(dir, candidate)

		exists, err := fileExists(path)
		if err != nil {
			return "", fmt.Errorf("%w: %w", errors.ErrFailedToReadConfig, err)
		}

		if exists {
			return path, nil
		}
	}

	return "", nil
}

// resolveExplicitConfig validates that an explicit config path exists
func resolveExplicitConfig(path string) (string, error) {
	exists, err := fileExists(path)
	if err != nil {
		return "", fmt.Errorf("%w: %w", errors.ErrFailedToReadConfig, err)
	}

	if !exists {
		return "", fmt.Errorf("%w: %s", errors.ErrFailedToReadConfig, path)
	}

	return path, nil
}

// fileExists checks whether a file exists, returning an error for non-ENOENT failures
func fileExists(path string) (bool, error) {
	_, err := os.Stat(path)

	switch {
	case err == nil:
		return true, nil
	case os.IsNotExist(err):
		return false, nil
	default:
		return false, err
	}
}

// parseConfig runs the config pipeline on raw YAML bytes
func parseConfig(cfg *Config, data []byte) (*Config, *Topology, error) {
	topology, err := parseTierOrder(data)
	if err != nil {
		return nil, nil, errors.ErrFailedToParseConfig
	}

	v := viper.New()
	v.SetConfigType("yaml")

	if err := v.ReadConfig(bytes.NewReader(data)); err != nil {
		return nil, nil, fmt.Errorf("%w: %w", errors.ErrFailedToParseConfig, err)
	}

	if err := v.Unmarshal(cfg); err != nil {
		return nil, nil, errors.ErrFailedToParseConfig
	}

	cfg.ApplyDefaults()
	cfg.normalizeTiers()

	if err := cfg.Validate(); err != nil {
		return nil, nil, fmt.Errorf("%w: %w", errors.ErrInvalidConfig, err)
	}

	return cfg, topology, nil
}
