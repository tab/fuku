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

			cfg, err := Load()

			if tt.error != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.error, err)
				assert.Nil(t, cfg)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, cfg)
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
			name: "valid configuration",
			config: &Config{
				Services: map[string]*Service{
					"api": {Dir: "api", Tier: Foundation},
					"web": {Dir: "web", Tier: Platform},
				},
			},
			expectError: false,
		},
		{
			name: "service with invalid tier",
			config: &Config{
				Services: map[string]*Service{
					"api": {Dir: "api", Tier: "invalid"},
				},
			},
			expectError: true,
			errorMsg:    "service api",
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
			name: "multiple services with one invalid",
			config: &Config{
				Services: map[string]*Service{
					"api":     {Dir: "api", Tier: Foundation},
					"invalid": {Dir: "invalid", Tier: "bad-tier"},
				},
			},
			expectError: true,
			errorMsg:    "service invalid",
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

func Test_ValidateTier(t *testing.T) {
	tests := []struct {
		name        string
		tier        string
		expectError bool
		expectedErr error
	}{
		{name: "empty tier is valid", tier: "", expectError: false},
		{name: "foundation tier is valid", tier: Foundation, expectError: false},
		{name: "platform tier is valid", tier: Platform, expectError: false},
		{name: "edge tier is valid", tier: Edge, expectError: false},
		{name: "invalid tier", tier: "invalid", expectError: true, expectedErr: errors.ErrInvalidTier},
		{name: "uppercase tier is invalid", tier: "FOUNDATION", expectError: true, expectedErr: errors.ErrInvalidTier},
		{name: "mixed case tier is invalid", tier: "Foundation", expectError: true, expectedErr: errors.ErrInvalidTier},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := &Service{Tier: tt.tier}
			err := service.validateTier()

			if tt.expectError {
				assert.Error(t, err)
				assert.ErrorIs(t, err, tt.expectedErr)
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
		{name: "nil readiness is valid", readiness: nil, expectError: false},
		{name: "http type with url is valid", readiness: &Readiness{Type: TypeHTTP, URL: "http://localhost:8080"}, expectError: false},
		{name: "log type with pattern is valid", readiness: &Readiness{Type: TypeLog, Pattern: "Server started"}, expectError: false},
		{name: "http type without url", readiness: &Readiness{Type: TypeHTTP}, expectError: true, expectedErr: errors.ErrReadinessURLRequired},
		{name: "log type without pattern", readiness: &Readiness{Type: TypeLog}, expectError: true, expectedErr: errors.ErrReadinessPatternRequired},
		{name: "empty type", readiness: &Readiness{Type: ""}, expectError: true, expectedErr: errors.ErrReadinessTypeRequired},
		{name: "invalid type", readiness: &Readiness{Type: "invalid"}, expectError: true, expectedErr: errors.ErrInvalidReadinessType},
		{name: "uppercase type is invalid", readiness: &Readiness{Type: "HTTP", URL: "http://localhost:8080"}, expectError: true, expectedErr: errors.ErrInvalidReadinessType},
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
