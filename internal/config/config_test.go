package config

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"fuku/internal/app/errors"
)

func Test_DefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	assert.NotNil(t, cfg.Services)
	assert.NotNil(t, cfg.Profiles)
	assert.Equal(t, LogLevel, cfg.Logging.Level)
	assert.Equal(t, LogFormat, cfg.Logging.Format)
	assert.Equal(t, MaxWorkers, cfg.Concurrency.Workers)
	assert.Equal(t, RetryAttempts, cfg.Retry.Attempts)
	assert.Equal(t, RetryBackoff, cfg.Retry.Backoff)
	assert.Equal(t, SocketLogsBufferSize, cfg.Logs.Buffer)
	assert.Equal(t, 1, cfg.Version)
}

func Test_DefaultTopology(t *testing.T) {
	topology := DefaultTopology()

	assert.NotNil(t, topology.TierServices)
	assert.Empty(t, topology.Order)
	assert.True(t, topology.HasDefaultOnly)
}

func Test_Load(t *testing.T) {
	tests := []struct {
		name        string
		setupFunc   func() func()
		expectError bool
		error       error
	}{
		{
			name: "no config file found - uses default",
			setupFunc: func() func() {
				return func() {}
			},
			error: nil,
		},
		{
			name: "valid config file",
			setupFunc: func() func() {
				content := `version: 1
services:
  test-service:
    dir: ./test
    profiles: [test]
profiles:
  test:
    include:
      - test-service
logging:
  level: debug
  format: json
`

				err := os.WriteFile("fuku.yaml", []byte(content), 0644)
				if err != nil {
					t.Fatal(err)
				}

				return func() { os.Remove("fuku.yaml") }
			},
			error: nil,
		},
		{
			name: "valid config file with concurrency",
			setupFunc: func() func() {
				content := `version: 1
services:
  test-service:
    dir: ./test
concurrency:
  workers: 10
`

				err := os.WriteFile("fuku.yaml", []byte(content), 0644)
				if err != nil {
					t.Fatal(err)
				}

				return func() { os.Remove("fuku.yaml") }
			},
			error: nil,
		},
		{
			name: "invalid concurrency workers zero",
			setupFunc: func() func() {
				content := `version: 1
services:
  test-service:
    dir: ./test
concurrency:
  workers: 0
`

				err := os.WriteFile("fuku.yaml", []byte(content), 0644)
				if err != nil {
					t.Fatal(err)
				}

				return func() { os.Remove("fuku.yaml") }
			},
			error: errors.ErrInvalidConfig,
		},
		{
			name: "invalid yaml structure for unmarshal",
			setupFunc: func() func() {
				content := `version: "invalid_version_type"
services: "this should be a map not a string"
`

				err := os.WriteFile("fuku.yaml", []byte(content), 0644)
				if err != nil {
					t.Fatal(err)
				}

				return func() { os.Remove("fuku.yaml") }
			},
			error: errors.ErrFailedToParseConfig,
		},
		{
			name: "permission denied error",
			setupFunc: func() func() {
				err := os.WriteFile("fuku.yaml", []byte("test"), 0644)
				if err != nil {
					t.Fatal(err)
				}

				err = os.Chmod("fuku.yaml", 0000)
				if err != nil {
					t.Fatal(err)
				}

				return func() {
					_ = os.Chmod("fuku.yaml", 0644)
					os.Remove("fuku.yaml")
				}
			},
			error: errors.ErrFailedToReadConfig,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanup := tt.setupFunc()
			defer cleanup()

			cfg, topology, err := Load()

			if tt.error != nil {
				assert.Error(t, err)
				assert.True(t, errors.Is(err, tt.error), "expected error %v, got %v", tt.error, err)
				assert.Nil(t, cfg)
				assert.Nil(t, topology)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, cfg)
				assert.NotNil(t, topology)
			}
		})
	}
}

func Test_LoadConcurrencyConfig(t *testing.T) {
	tests := []struct {
		name            string
		yaml            string
		expectedWorkers int
	}{
		{
			name:            "default workers when not specified",
			yaml:            `version: 1`,
			expectedWorkers: MaxWorkers,
		},
		{
			name: "custom workers value",
			yaml: `version: 1
concurrency:
  workers: 10`,
			expectedWorkers: 10,
		},
		{
			name: "workers value of 1",
			yaml: `version: 1
concurrency:
  workers: 1`,
			expectedWorkers: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := os.WriteFile("fuku.yaml", []byte(tt.yaml), 0644)
			if err != nil {
				t.Fatal(err)
			}
			defer os.Remove("fuku.yaml")

			cfg, _, err := Load()
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedWorkers, cfg.Concurrency.Workers)
		})
	}
}

func Test_LoadRetryConfig(t *testing.T) {
	tests := []struct {
		name             string
		yaml             string
		expectedAttempts int
		expectedBackoff  time.Duration
	}{
		{
			name:             "default retry when not specified",
			yaml:             `version: 1`,
			expectedAttempts: RetryAttempts,
			expectedBackoff:  RetryBackoff,
		},
		{
			name: "custom retry values",
			yaml: `version: 1
retry:
  attempts: 5
  backoff: 1s`,
			expectedAttempts: 5,
			expectedBackoff:  time.Second,
		},
		{
			name: "retry with zero backoff",
			yaml: `version: 1
retry:
  attempts: 1
  backoff: 0s`,
			expectedAttempts: 1,
			expectedBackoff:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := os.WriteFile("fuku.yaml", []byte(tt.yaml), 0644)
			if err != nil {
				t.Fatal(err)
			}
			defer os.Remove("fuku.yaml")

			cfg, _, err := Load()
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedAttempts, cfg.Retry.Attempts)
			assert.Equal(t, tt.expectedBackoff, cfg.Retry.Backoff)
		})
	}
}

func Test_LoadLogsConfig(t *testing.T) {
	tests := []struct {
		name           string
		yaml           string
		expectedBuffer int
	}{
		{
			name:           "default buffer when not specified",
			yaml:           `version: 1`,
			expectedBuffer: SocketLogsBufferSize,
		},
		{
			name: "custom buffer value",
			yaml: `version: 1
logs:
  buffer: 500`,
			expectedBuffer: 500,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := os.WriteFile("fuku.yaml", []byte(tt.yaml), 0644)
			if err != nil {
				t.Fatal(err)
			}
			defer os.Remove("fuku.yaml")

			cfg, _, err := Load()
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedBuffer, cfg.Logs.Buffer)
		})
	}
}

func Test_ApplyDefaults(t *testing.T) {
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
					Profiles: []string{"default"},
				},
			},
			expected: &Config{
				Services: map[string]*Service{
					"api":  {Dir: "api", Profiles: []string{"default"}},
					"test": {Dir: "test", Profiles: []string{"default"}},
				},
				Defaults: &ServiceDefaults{
					Profiles: []string{"default"},
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
			tt.config.ApplyDefaults()
			assert.Equal(t, tt.expected, tt.config)
		})
	}
}

func Test_Validate(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid configuration with default workers",
			config:      DefaultConfig(),
			expectError: false,
		},
		{
			name: "valid configuration with custom workers",
			config: func() *Config {
				cfg := DefaultConfig()
				cfg.Concurrency.Workers = 10

				return cfg
			}(),
			expectError: false,
		},
		{
			name: "invalid workers zero",
			config: func() *Config {
				cfg := DefaultConfig()
				cfg.Concurrency.Workers = 0

				return cfg
			}(),
			expectError: true,
			errorMsg:    "concurrency workers must be greater than 0",
		},
		{
			name: "invalid workers negative",
			config: func() *Config {
				cfg := DefaultConfig()
				cfg.Concurrency.Workers = -1

				return cfg
			}(),
			expectError: true,
			errorMsg:    "concurrency workers must be greater than 0",
		},
		{
			name: "invalid retry attempts zero",
			config: func() *Config {
				cfg := DefaultConfig()
				cfg.Retry.Attempts = 0

				return cfg
			}(),
			expectError: true,
			errorMsg:    "retry attempts must be greater than 0",
		},
		{
			name: "invalid retry attempts negative",
			config: func() *Config {
				cfg := DefaultConfig()
				cfg.Retry.Attempts = -1

				return cfg
			}(),
			expectError: true,
			errorMsg:    "retry attempts must be greater than 0",
		},
		{
			name: "invalid retry backoff negative",
			config: func() *Config {
				cfg := DefaultConfig()
				cfg.Retry.Backoff = -1

				return cfg
			}(),
			expectError: true,
			errorMsg:    "retry backoff must not be negative",
		},
		{
			name: "invalid logs buffer zero",
			config: func() *Config {
				cfg := DefaultConfig()
				cfg.Logs.Buffer = 0

				return cfg
			}(),
			expectError: true,
			errorMsg:    "logs buffer must be greater than 0",
		},
		{
			name: "invalid logs buffer negative",
			config: func() *Config {
				cfg := DefaultConfig()
				cfg.Logs.Buffer = -1

				return cfg
			}(),
			expectError: true,
			errorMsg:    "logs buffer must be greater than 0",
		},
		{
			name: "valid configuration with standard tiers",
			config: func() *Config {
				cfg := DefaultConfig()
				cfg.Services = map[string]*Service{
					"api": {Dir: "api", Tier: "foundation"},
					"web": {Dir: "web", Tier: "platform"},
				}

				return cfg
			}(),
			expectError: false,
		},
		{
			name: "valid configuration with custom tier",
			config: func() *Config {
				cfg := DefaultConfig()
				cfg.Services = map[string]*Service{
					"api": {Dir: "api", Tier: "custom-tier"},
				}

				return cfg
			}(),
			expectError: false,
		},
		{
			name: "valid configuration with mixed tiers",
			config: func() *Config {
				cfg := DefaultConfig()
				cfg.Services = map[string]*Service{
					"api":     {Dir: "api", Tier: "foundation"},
					"custom":  {Dir: "custom", Tier: "middleware"},
					"another": {Dir: "another", Tier: "services"},
				}

				return cfg
			}(),
			expectError: false,
		},
		{
			name: "service with invalid readiness type",
			config: func() *Config {
				cfg := DefaultConfig()
				cfg.Services = map[string]*Service{
					"api": {Dir: "api", Readiness: &Readiness{Type: "invalid"}},
				}

				return cfg
			}(),
			expectError: true,
			errorMsg:    "service api",
		},
		{
			name: "service with http readiness missing url",
			config: func() *Config {
				cfg := DefaultConfig()
				cfg.Services = map[string]*Service{
					"api": {Dir: "api", Readiness: &Readiness{Type: TypeHTTP}},
				}

				return cfg
			}(),
			expectError: true,
			errorMsg:    "service api",
		},
		{
			name: "service with log readiness missing pattern",
			config: func() *Config {
				cfg := DefaultConfig()
				cfg.Services = map[string]*Service{
					"api": {Dir: "api", Readiness: &Readiness{Type: TypeLog}},
				}

				return cfg
			}(),
			expectError: true,
			errorMsg:    "service api",
		},
		{
			name:        "empty services map",
			config:      DefaultConfig(),
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func Test_ValidateReadiness(t *testing.T) {
	tests := []struct {
		name        string
		readiness   *Readiness
		expectError bool
		expectedErr error
	}{
		{
			name:        "nil readiness is valid",
			readiness:   nil,
			expectError: false,
		},
		{
			name: "http type with url is valid",
			readiness: &Readiness{
				Type: TypeHTTP,
				URL:  "http://localhost:8080",
			},
			expectError: false,
		},
		{
			name: "log type with pattern is valid",
			readiness: &Readiness{
				Type:    TypeLog,
				Pattern: "Server started",
			},
			expectError: false,
		},
		{
			name: "http type without url",
			readiness: &Readiness{
				Type: TypeHTTP,
			},
			expectError: true,
			expectedErr: errors.ErrReadinessURLRequired,
		},
		{
			name: "log type without pattern",
			readiness: &Readiness{
				Type: TypeLog,
			},
			expectError: true,
			expectedErr: errors.ErrReadinessPatternRequired,
		},
		{
			name: "empty type",
			readiness: &Readiness{
				Type: "",
			},
			expectError: true,
			expectedErr: errors.ErrReadinessTypeRequired,
		},
		{
			name: "invalid type",
			readiness: &Readiness{
				Type: "invalid",
			},
			expectError: true,
			expectedErr: errors.ErrInvalidReadinessType,
		},
		{
			name: "uppercase type is invalid",
			readiness: &Readiness{
				Type: "HTTP",
				URL:  "http://localhost:8080",
			},
			expectError: true,
			expectedErr: errors.ErrInvalidReadinessType,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := &Service{Readiness: tt.readiness}
			err := service.validateReadiness()

			if tt.expectError {
				assert.Error(t, err)
				assert.ErrorIs(t, err, tt.expectedErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func Test_ParseTierOrder(t *testing.T) {
	tests := []struct {
		name              string
		yaml              string
		expectedTierOrder []string
		expectedServices  map[string][]string
	}{
		{
			name: "standard tiers in order",
			yaml: `services:
  storage:
    tier: foundation
  api:
    tier: platform
  web:
    tier: edge`,
			expectedTierOrder: []string{"foundation", "platform", "edge"},
			expectedServices:  map[string][]string{"foundation": {"storage"}, "platform": {"api"}, "edge": {"web"}},
		},
		{
			name: "custom tier names",
			yaml: `services:
  db:
    tier: infrastructure
  svc:
    tier: middleware
  ui:
    tier: frontend`,
			expectedTierOrder: []string{"infrastructure", "middleware", "frontend"},
			expectedServices:  map[string][]string{"infrastructure": {"db"}, "middleware": {"svc"}, "frontend": {"ui"}},
		},
		{
			name: "mixed tiers with duplicates",
			yaml: `services:
  db1:
    tier: foundation
  api1:
    tier: platform
  db2:
    tier: foundation
  api2:
    tier: platform`,
			expectedTierOrder: []string{"foundation", "platform"},
			expectedServices:  map[string][]string{"foundation": {"db1", "db2"}, "platform": {"api1", "api2"}},
		},
		{
			name: "services without tiers",
			yaml: `services:
  svc1:
    dir: ./svc1
  svc2:
    dir: ./svc2`,
			expectedTierOrder: []string{"default"},
			expectedServices:  map[string][]string{"default": {"svc1", "svc2"}},
		},
		{
			name: "mixed with and without tiers",
			yaml: `services:
  db:
    tier: foundation
  svc1:
    dir: ./svc1
  api:
    tier: platform
  svc2:
    dir: ./svc2`,
			expectedTierOrder: []string{"foundation", "platform", "default"},
			expectedServices:  map[string][]string{"foundation": {"db"}, "default": {"svc1", "svc2"}, "platform": {"api"}},
		},
		{
			name: "case insensitive and whitespace trimming",
			yaml: `services:
  svc1:
    tier: " Foundation "
  svc2:
    tier: PLATFORM
  svc3:
    tier: foundation`,
			expectedTierOrder: []string{"foundation", "platform"},
			expectedServices:  map[string][]string{"foundation": {"svc1", "svc3"}, "platform": {"svc2"}},
		},
		{
			name:              "empty services",
			yaml:              `services: {}`,
			expectedTierOrder: []string{},
			expectedServices:  map[string][]string{},
		},
		{
			name: "services inherit defaults.tier",
			yaml: `services:
  api:
    dir: ./api
  web:
    dir: ./web
defaults:
  tier: platform`,
			expectedTierOrder: []string{"platform"},
			expectedServices:  map[string][]string{"platform": {"api", "web"}},
		},
		{
			name: "mixed explicit and inherited tiers",
			yaml: `services:
  db:
    tier: foundation
  api:
    dir: ./api
  cache:
    dir: ./cache
defaults:
  tier: platform`,
			expectedTierOrder: []string{"foundation", "platform"},
			expectedServices:  map[string][]string{"foundation": {"db"}, "platform": {"api", "cache"}},
		},
		{
			name: "defaults.tier with whitespace and case",
			yaml: `services:
  api:
    dir: ./api
defaults:
  tier: " PLATFORM "`,
			expectedTierOrder: []string{"platform"},
			expectedServices:  map[string][]string{"platform": {"api"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			topology, err := parseTierOrder([]byte(tt.yaml))
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedTierOrder, topology.Order)
			assert.Equal(t, tt.expectedServices, topology.TierServices)
		})
	}
}

func Test_ParseTierOrder_HasDefaultOnly(t *testing.T) {
	tests := []struct {
		name                   string
		yaml                   string
		expectedHasDefaultOnly bool
	}{
		{
			name:                   "empty services has default only",
			yaml:                   `services: {}`,
			expectedHasDefaultOnly: true,
		},
		{
			name: "services without tiers has default only",
			yaml: `services:
  api:
    dir: ./api`,
			expectedHasDefaultOnly: true,
		},
		{
			name: "services with only default tier has default only",
			yaml: `services:
  api:
    tier: default`,
			expectedHasDefaultOnly: true,
		},
		{
			name: "services with multiple tiers not default only",
			yaml: `services:
  db:
    tier: foundation
  api:
    tier: platform`,
			expectedHasDefaultOnly: false,
		},
		{
			name: "services with foundation tier not default only",
			yaml: `services:
  db:
    tier: foundation`,
			expectedHasDefaultOnly: false,
		},
		{
			name: "mixed explicit and default not default only",
			yaml: `services:
  db:
    tier: foundation
  api:
    dir: ./api`,
			expectedHasDefaultOnly: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			topology, err := parseTierOrder([]byte(tt.yaml))
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedHasDefaultOnly, topology.HasDefaultOnly)
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

func Test_ValidateWatch(t *testing.T) {
	tests := []struct {
		name        string
		watch       *Watch
		expectError bool
		expectedErr error
	}{
		{
			name:        "nil watch is valid",
			watch:       nil,
			expectError: false,
		},
		{
			name: "watch with include is valid",
			watch: &Watch{
				Include: []string{"**/*.go"},
			},
			expectError: false,
		},
		{
			name: "watch with include and ignore is valid",
			watch: &Watch{
				Include: []string{"**/*.go", "**/*.yaml"},
				Ignore:  []string{"*_test.go", "vendor/**"},
			},
			expectError: false,
		},
		{
			name: "watch with include ignore and shared is valid",
			watch: &Watch{
				Include: []string{"**/*.go"},
				Ignore:  []string{"*_test.go"},
				Shared:  []string{"pkg/common", "pkg/models"},
			},
			expectError: false,
		},
		{
			name: "watch with empty include",
			watch: &Watch{
				Include: []string{},
			},
			expectError: true,
			expectedErr: errors.ErrWatchIncludeRequired,
		},
		{
			name: "watch without include field",
			watch: &Watch{
				Ignore: []string{"*_test.go"},
			},
			expectError: true,
			expectedErr: errors.ErrWatchIncludeRequired,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := &Service{Watch: tt.watch}
			err := service.validateWatch()

			if tt.expectError {
				assert.Error(t, err)
				assert.ErrorIs(t, err, tt.expectedErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func Test_LoadWatchConfig(t *testing.T) {
	tests := []struct {
		name          string
		yaml          string
		expectedWatch map[string]*Watch
		expectError   bool
	}{
		{
			name: "service with watch config",
			yaml: `version: 1
services:
  api:
    dir: ./api
    watch:
      include: ["**/*.go"]
      ignore: ["*_test.go"]`,
			expectedWatch: map[string]*Watch{
				"api": {
					Include: []string{"**/*.go"},
					Ignore:  []string{"*_test.go"},
				},
			},
			expectError: false,
		},
		{
			name: "service with watch config and shared dirs",
			yaml: `version: 1
services:
  api:
    dir: ./api
    watch:
      include: ["**/*.go"]
      ignore: ["*_test.go"]
      shared: ["pkg/common", "pkg/models"]`,
			expectedWatch: map[string]*Watch{
				"api": {
					Include: []string{"**/*.go"},
					Ignore:  []string{"*_test.go"},
					Shared:  []string{"pkg/common", "pkg/models"},
				},
			},
			expectError: false,
		},
		{
			name: "service without watch config",
			yaml: `version: 1
services:
  api:
    dir: ./api`,
			expectedWatch: map[string]*Watch{
				"api": nil,
			},
			expectError: false,
		},
		{
			name: "service with watch but empty include",
			yaml: `version: 1
services:
  api:
    dir: ./api
    watch:
      include: []`,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := os.WriteFile("fuku.yaml", []byte(tt.yaml), 0644)
			if err != nil {
				t.Fatal(err)
			}
			defer os.Remove("fuku.yaml")

			cfg, _, err := Load()

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)

				for name, expectedWatch := range tt.expectedWatch {
					service, ok := cfg.Services[name]
					assert.True(t, ok)
					assert.Equal(t, expectedWatch, service.Watch)
				}
			}
		})
	}
}
