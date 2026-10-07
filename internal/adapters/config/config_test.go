package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"fuku/internal/model"
)

func Test_defaultConfig(t *testing.T) {
	cfg := defaultConfig()

	assert.NotNil(t, cfg.Services)
	assert.NotNil(t, cfg.Profiles)
	assert.Equal(t, DefaultLogLevel, cfg.Logging.Level)
	assert.Equal(t, DefaultLogFormat, cfg.Logging.Format)
	assert.Equal(t, MaxWorkers, cfg.Concurrency.Workers)
	assert.Equal(t, RetryAttempts, cfg.Retry.Attempts)
	assert.Equal(t, RetryBackoff, cfg.Retry.Backoff)
	assert.Equal(t, SocketLogsBufferSize, cfg.Logs.Buffer)
	assert.Equal(t, SocketLogsHistorySize, cfg.Logs.History)
	assert.Equal(t, DefaultAPIListen, cfg.Server.Listen)
}

func Test_applyDefaults(t *testing.T) {
	tests := []struct {
		name     string
		config   *Config
		expected *Config
	}{
		{
			name: "no defaults",
			config: &Config{
				Services: map[string]*Service{
					"test": {Dir: "test"},
				},
			},
			expected: &Config{
				Services: map[string]*Service{
					"test": {Dir: "test"},
				},
			},
		},
		{
			name: "apply defaults to service",
			config: &Config{
				Services: map[string]*Service{
					"api":  {Dir: "api"},
					"test": {},
				},
				Defaults: &ServiceDefaults{
					Tier: " Platform ",
				},
			},
			expected: &Config{
				Services: map[string]*Service{
					"api":  {Dir: "api", Tier: "platform"},
					"test": {Dir: "test", Tier: "platform"},
				},
				Defaults: &ServiceDefaults{
					Tier: " Platform ",
				},
			},
		},
		{
			name: "service tier is normalized and wins over the defaults tier",
			config: &Config{
				Services: map[string]*Service{
					"api": {Dir: "api", Tier: " Foundation "},
				},
				Defaults: &ServiceDefaults{
					Tier: "platform",
				},
			},
			expected: &Config{
				Services: map[string]*Service{
					"api": {Dir: "api", Tier: "foundation"},
				},
				Defaults: &ServiceDefaults{
					Tier: "platform",
				},
			},
		},
		{
			name: "whitespace-only service tier takes the defaults tier",
			config: &Config{
				Services: map[string]*Service{
					"api": {Dir: "api", Tier: "   "},
				},
				Defaults: &ServiceDefaults{
					Tier: "platform",
				},
			},
			expected: &Config{
				Services: map[string]*Service{
					"api": {Dir: "api", Tier: "platform"},
				},
				Defaults: &ServiceDefaults{
					Tier: "platform",
				},
			},
		},
		{
			name: "service tier is normalized without defaults",
			config: &Config{
				Services: map[string]*Service{
					"api": {Dir: "api", Tier: "PLATFORM"},
				},
			},
			expected: &Config{
				Services: map[string]*Service{
					"api": {Dir: "api", Tier: "platform"},
				},
			},
		},
		{
			name: "readiness without timeout and interval gets the defaults",
			config: &Config{
				Services: map[string]*Service{
					"api": {Dir: "api", Readiness: &Readiness{Type: model.ReadinessHTTP}},
				},
			},
			expected: &Config{
				Services: map[string]*Service{
					"api": {Dir: "api", Readiness: &Readiness{Type: model.ReadinessHTTP, Timeout: DefaultTimeout, Interval: DefaultInterval}},
				},
			},
		},
		{
			name: "readiness keeps an explicit timeout and interval",
			config: &Config{
				Services: map[string]*Service{
					"api": {Dir: "api", Readiness: &Readiness{Type: model.ReadinessHTTP, Timeout: time.Minute, Interval: time.Second}},
				},
			},
			expected: &Config{
				Services: map[string]*Service{
					"api": {Dir: "api", Readiness: &Readiness{Type: model.ReadinessHTTP, Timeout: time.Minute, Interval: time.Second}},
				},
			},
		},
		{
			name: "service without watch config not affected",
			config: &Config{
				Services: map[string]*Service{
					"api": {Dir: "api"},
				},
			},
			expected: &Config{
				Services: map[string]*Service{
					"api": {Dir: "api"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.config.applyDefaults()
			assert.Equal(t, tt.expected, tt.config)
		})
	}
}

func Test_NormalizeTier(t *testing.T) {
	tests := []struct {
		name     string
		tier     string
		expected string
	}{
		{
			name:     "lowercase tier unchanged",
			tier:     "foundation",
			expected: "foundation",
		},
		{
			name:     "uppercase tier lowercased",
			tier:     "FOUNDATION",
			expected: "foundation",
		},
		{
			name:     "mixed case tier lowercased",
			tier:     "Foundation",
			expected: "foundation",
		},
		{
			name:     "whitespace trimmed",
			tier:     "  foundation  ",
			expected: "foundation",
		},
		{
			name:     "mixed case with whitespace",
			tier:     " Platform ",
			expected: "platform",
		},
		{
			name:     "empty tier",
			tier:     "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeTier(tt.tier)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_NormalizeExclude(t *testing.T) {
	cfg := &Config{}

	tests := []struct {
		name     string
		exclude  []string
		expected []string
	}{
		{
			name:     "nil exclude unchanged",
			exclude:  nil,
			expected: nil,
		},
		{
			name:     "empty exclude unchanged",
			exclude:  []string{},
			expected: []string{},
		},
		{
			name:     "single entry preserved",
			exclude:  []string{"api"},
			expected: []string{"api"},
		},
		{
			name:     "whitespace trimmed",
			exclude:  []string{"  api  ", "\tworker\t"},
			expected: []string{"api", "worker"},
		},
		{
			name:     "empty strings dropped",
			exclude:  []string{"api", "", "worker"},
			expected: []string{"api", "worker"},
		},
		{
			name:     "whitespace-only entries dropped",
			exclude:  []string{"api", "   ", "worker"},
			expected: []string{"api", "worker"},
		},
		{
			name:     "duplicates deduplicated preserving order",
			exclude:  []string{"api", "worker", "api", "db", "worker"},
			expected: []string{"api", "worker", "db"},
		},
		{
			name:     "duplicates after trim deduplicated",
			exclude:  []string{"api", " api ", "worker"},
			expected: []string{"api", "worker"},
		},
		{
			name:     "all empty entries result in empty slice",
			exclude:  []string{"", "   ", "\t"},
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg.Exclude = tt.exclude

			cfg.normalizeExclude()
			assert.Equal(t, tt.expected, cfg.Exclude)
		})
	}
}
