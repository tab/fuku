package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"

	"fuku/internal/contracts"
	"fuku/internal/model"
)

// files names the config file a load read and the override file beside it
type files struct {
	path     string
	override string
}

// loadDefault loads configuration from the default config file with optional override merging
func loadDefault() (*Config, *model.Topology, files, error) {
	cfg := initConfig()

	path, err := resolveDefaultConfig()
	if err != nil {
		return nil, nil, files{}, err
	}

	if path == "" {
		return cfg, defaultTopology(), files{}, nil
	}

	override, err := resolveOverrideFile(path)
	if err != nil {
		return nil, nil, files{path: path}, err
	}

	source := files{path: path, override: override}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, source, fmt.Errorf("%w: %w", contracts.ErrFailedToReadConfig, err)
	}

	data, err = applyOverride(override, data)
	if err != nil {
		return nil, nil, source, err
	}

	cfg, topology, err := parseConfig(cfg, data)

	return cfg, topology, source, err
}

// loadFromFile loads an explicit file without override merging and reports the override beside it as skipped
func loadFromFile(path string) (*Config, *model.Topology, files, error) {
	cfg := initConfig()

	override, err := resolveOverrideFile(path)
	if err != nil {
		return nil, nil, files{path: path}, err
	}

	source := files{path: path, override: override}

	filePath, err := resolveExplicitConfig(path)
	if err != nil {
		return nil, nil, source, err
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, nil, source, fmt.Errorf("%w: %w", contracts.ErrFailedToReadConfig, err)
	}

	cfg, topology, err := parseConfig(cfg, data)

	return cfg, topology, source, err
}

// loadEnv reads .env.<GO_ENV>.local, .env.<GO_ENV> and .env in that order, and the first file to set a variable wins
func loadEnv() {
	goEnv := os.Getenv("GO_ENV")
	if goEnv != "" {
		_ = godotenv.Load(".env." + goEnv + ".local")
		_ = godotenv.Load(".env." + goEnv)
	}

	_ = godotenv.Load()
}

// resolveEnv returns the current environment name from GO_ENV, defaulting to EnvProduction
func resolveEnv() string {
	env := os.Getenv("GO_ENV")
	if env == "" {
		return EnvProduction
	}

	return env
}

// Telemetry loads the env files and returns the telemetry settings the environment sets
func Telemetry() model.Telemetry {
	loadEnv()

	return model.Telemetry{
		Enabled:     os.Getenv("FUKU_TELEMETRY_DISABLED") != "1",
		DSN:         os.Getenv("SENTRY_DSN"),
		Environment: resolveEnv(),
	}
}

// initConfig creates a default config populated with environment values
func initConfig() *Config {
	cfg := defaultConfig()
	telemetry := Telemetry()
	cfg.AppEnv = telemetry.Environment
	cfg.SentryDSN = telemetry.DSN
	cfg.Telemetry = telemetry.Enabled
	cfg.API = os.Getenv("FUKU_API_DISABLED") != "1"
	cfg.Updater = os.Getenv("FUKU_UPDATER_DISABLED") != "1"

	return cfg
}

// resolveDefaultConfig tries fuku.yaml then fuku.yml in the current directory
func resolveDefaultConfig() (string, error) {
	for _, candidate := range []string{ConfigFile, ConfigFileAlt} {
		exists, err := fileExists(candidate)
		if err != nil {
			return "", fmt.Errorf("%w: %w", contracts.ErrFailedToReadConfig, err)
		}

		if exists {
			return candidate, nil
		}
	}

	return "", nil
}

// applyOverride merges the override file into data (data as is when there is no override)
func applyOverride(overridePath string, data []byte) ([]byte, error) {
	if overridePath == "" {
		return data, nil
	}

	overrideData, err := os.ReadFile(overridePath)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", contracts.ErrFailedToReadConfig, err)
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
			return "", fmt.Errorf("%w: %w", contracts.ErrFailedToReadConfig, err)
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
		return "", fmt.Errorf("%w: %w", contracts.ErrFailedToReadConfig, err)
	}

	if !exists {
		return "", fmt.Errorf("%w: %s", contracts.ErrFailedToReadConfig, path)
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
func parseConfig(cfg *Config, data []byte) (*Config, *model.Topology, error) {
	topology, err := parseTierOrder(data)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: %w", ErrFailedToParseConfig, err)
	}

	v := viper.New()
	v.SetConfigType("yaml")

	if err := v.ReadConfig(bytes.NewReader(data)); err != nil {
		return nil, nil, fmt.Errorf("%w: %w", ErrFailedToParseConfig, err)
	}

	if err := v.Unmarshal(cfg); err != nil {
		return nil, nil, fmt.Errorf("%w: %w", ErrFailedToParseConfig, err)
	}

	cfg.keepEmptyServices(v.GetStringMap(keyServices))

	if err := cfg.validate(); err != nil {
		return nil, nil, fmt.Errorf("%w: %w", contracts.ErrInvalidConfig, err)
	}

	cfg.applyDefaults()
	cfg.normalizeExclude()

	return cfg, topology, nil
}

// keepEmptyServices adds back the services viper drops for a null body, so validation can reject them
func (c *Config) keepEmptyServices(declared map[string]any) {
	for name := range declared {
		if _, exists := c.Services[name]; !exists {
			c.Services[name] = nil
		}
	}
}
