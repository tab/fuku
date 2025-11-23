package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"fuku/internal/app/errors"
)

func Test_DefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	assert.NotNil(t, cfg.Services)
	assert.NotNil(t, cfg.Profiles)
	assert.Equal(t, DefaultLogLevel, cfg.Logging.Level)
	assert.Equal(t, DefaultLogFormat, cfg.Logging.Format)
	assert.Equal(t, 1, cfg.Version)
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
				assert.Equal(t, tt.error, err)
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
			name: "valid configuration with standard tiers",
			config: &Config{
				Services: map[string]*Service{
					"api": {Dir: "api", Tier: Foundation},
					"web": {Dir: "web", Tier: Platform},
				},
			},
			expectError: false,
		},
		{
			name: "valid configuration with custom tier",
			config: &Config{
				Services: map[string]*Service{
					"api": {Dir: "api", Tier: "custom-tier"},
				},
			},
			expectError: false,
		},
		{
			name: "valid configuration with mixed tiers",
			config: &Config{
				Services: map[string]*Service{
					"api":     {Dir: "api", Tier: Foundation},
					"custom":  {Dir: "custom", Tier: "middleware"},
					"another": {Dir: "another", Tier: "services"},
				},
			},
			expectError: false,
		},
		{
			name: "service with invalid readiness type",
			config: &Config{
				Services: map[string]*Service{
					"api": {Dir: "api", Readiness: &Readiness{Type: "invalid"}},
				},
			},
			expectError: true,
			errorMsg:    "service api",
		},
		{
			name: "service with http readiness missing url",
			config: &Config{
				Services: map[string]*Service{
					"api": {Dir: "api", Readiness: &Readiness{Type: TypeHTTP}},
				},
			},
			expectError: true,
			errorMsg:    "service api",
		},
		{
			name: "service with log readiness missing pattern",
			config: &Config{
				Services: map[string]*Service{
					"api": {Dir: "api", Readiness: &Readiness{Type: TypeLog}},
				},
			},
			expectError: true,
			errorMsg:    "service api",
		},
		{
			name: "empty services map",
			config: &Config{
				Services: map[string]*Service{},
			},
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
			name:      "nil readiness is valid",
			readiness: nil, expectError: false},
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
			name: "empty tier",
			tier: "", expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeTier(tt.tier)
			assert.Equal(t, tt.expected, result)
		})
	}
}
