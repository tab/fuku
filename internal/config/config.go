package config

import (
	"bytes"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/spf13/viper"
	"go.yaml.in/yaml/v3"

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
	Concurrency struct {
		Workers int `yaml:"workers"`
	}
	Retry struct {
		Attempts int           `yaml:"attempts"`
		Backoff  time.Duration `yaml:"backoff"`
	}
	Logs struct {
		Buffer int `yaml:"buffer"`
	}
	Version int
}

// Topology represents the derived tier ordering and grouping metadata
type Topology struct {
	Order          []string
	TierServices   map[string][]string
	HasDefaultOnly bool
}

// Service represents a service configuration
type Service struct {
	Dir       string     `yaml:"dir"`
	Profiles  []string   `yaml:"profiles"`
	Tier      string     `yaml:"tier"`
	Readiness *Readiness `yaml:"readiness"`
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

// ServiceDefaults represents default configuration for services
type ServiceDefaults struct {
	Profiles []string `yaml:"profiles"`
	Tier     string   `yaml:"tier"`
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	cfg := &Config{
		Services: make(map[string]*Service),
		Profiles: make(map[string]interface{}),
		Version:  1,
	}

	cfg.Logging.Level = LogLevel
	cfg.Logging.Format = LogFormat

	cfg.Concurrency.Workers = MaxWorkers

	cfg.Retry.Attempts = RetryAttempts
	cfg.Retry.Backoff = RetryBackoff

	cfg.Logs.Buffer = SocketLogsBufferSize

	cfg.Profiles[Default] = "*"

	return cfg
}

// DefaultTopology returns the default topology
func DefaultTopology() *Topology {
	return &Topology{
		Order:          []string{},
		TierServices:   make(map[string][]string),
		HasDefaultOnly: true,
	}
}

// Load loads the configuration from file and returns read-only config with derived topology
func Load() (*Config, *Topology, error) {
	cfg := DefaultConfig()
	defaultTopology := DefaultTopology()

	data, err := os.ReadFile("fuku.yaml")
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, defaultTopology, nil
		}

		return nil, nil, errors.ErrFailedToReadConfig
	}

	topology, err := parseTierOrder(data)
	if err != nil {
		return nil, nil, errors.ErrFailedToParseConfig
	}

	v := viper.New()
	v.SetConfigType("yaml")

	if err := v.ReadConfig(bytes.NewReader(data)); err != nil {
		return nil, nil, errors.ErrFailedToReadConfig
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

// parseTierOrder reads fuku.yaml and extracts tier ordering
func parseTierOrder(data []byte) (*Topology, error) {
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, err
	}

	topology := &Topology{
		Order:        []string{},
		TierServices: make(map[string][]string),
	}

	tierSeen := make(map[string]bool)
	hasDefaultServices := false

	if root.Kind != yaml.DocumentNode || len(root.Content) == 0 {
		return topology, nil
	}

	doc := root.Content[0]
	if doc.Kind != yaml.MappingNode {
		return topology, nil
	}

	defaultTier := ""

	for i := 0; i < len(doc.Content); i += 2 {
		key := doc.Content[i]
		value := doc.Content[i+1]

		if key.Value == "defaults" && value.Kind == yaml.MappingNode {
			for j := 0; j < len(value.Content); j += 2 {
				fieldKey := value.Content[j]
				fieldValue := value.Content[j+1]

				if fieldKey.Value == "tier" {
					defaultTier = normalizeTier(fieldValue.Value)
					break
				}
			}
		}
	}

	for i := 0; i < len(doc.Content); i += 2 {
		key := doc.Content[i]
		value := doc.Content[i+1]

		if key.Value != "services" || value.Kind != yaml.MappingNode {
			continue
		}

		for j := 0; j < len(value.Content); j += 2 {
			serviceName := value.Content[j].Value
			serviceNode := value.Content[j+1]

			if serviceNode.Kind != yaml.MappingNode {
				continue
			}

			tier := ""

			for k := 0; k < len(serviceNode.Content); k += 2 {
				fieldKey := serviceNode.Content[k]
				fieldValue := serviceNode.Content[k+1]

				if fieldKey.Value == "tier" {
					tier = normalizeTier(fieldValue.Value)
					break
				}
			}

			if tier == "" {
				if defaultTier != "" {
					tier = defaultTier
				} else {
					tier = Default
					hasDefaultServices = true
				}
			}

			if tier != Default && !tierSeen[tier] {
				tierSeen[tier] = true
				topology.Order = append(topology.Order, tier)
			}

			topology.TierServices[tier] = append(topology.TierServices[tier], serviceName)
		}
	}

	if hasDefaultServices {
		topology.Order = append(topology.Order, Default)
	}

	for tier := range topology.TierServices {
		sort.Strings(topology.TierServices[tier])
	}

	topology.HasDefaultOnly = len(topology.Order) == 0 || (len(topology.Order) == 1 && topology.Order[0] == Default)

	return topology, nil
}

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
		if err := service.validateReadiness(); err != nil {
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
