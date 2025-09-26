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
    depends_on: []
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
					os.Chmod("fuku.yaml", 0644)
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
					"test": {Dir: "test", DependsOn: []string{}},
				},
			},
			expected: &Config{
				Services: map[string]*Service{
					"test": {Dir: "test", DependsOn: []string{}},
				},
			},
		},
		{
			name: "apply defaults to service",
			config: &Config{
				Services: map[string]*Service{
					"api":  {Dir: "api", DependsOn: []string{}},
					"test": {DependsOn: []string{}},
				},
				Defaults: &ServiceDefaults{
					DependsOn: []string{"api"},
					Profiles:  []string{"default"},
					Exclude:   []string{"api"},
				},
			},
			expected: &Config{
				Services: map[string]*Service{
					"api":  {Dir: "api", DependsOn: []string{}},
					"test": {Dir: "test", DependsOn: []string{"api"}, Profiles: []string{"default"}},
				},
				Defaults: &ServiceDefaults{
					DependsOn: []string{"api"},
					Profiles:  []string{"default"},
					Exclude:   []string{"api"},
				},
			},
		},
		{
			name: "skip excluded services",
			config: &Config{
				Services: map[string]*Service{
					"api":  {Dir: "api", DependsOn: []string{}},
					"test": {DependsOn: []string{}},
				},
				Defaults: &ServiceDefaults{
					DependsOn: []string{"api"},
					Exclude:   []string{"api", "test"},
				},
			},
			expected: &Config{
				Services: map[string]*Service{
					"api":  {Dir: "api", DependsOn: []string{}},
					"test": {Dir: "", DependsOn: []string{}},
				},
				Defaults: &ServiceDefaults{
					DependsOn: []string{"api"},
					Exclude:   []string{"api", "test"},
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

func Test_GetServicesForProfile(t *testing.T) {
	type result struct {
		services []string
		error    bool
	}

	tests := []struct {
		name     string
		config   *Config
		profile  string
		expected result
	}{
		{
			name: "profile not found",
			config: &Config{
				Services: map[string]*Service{
					"api": {Dir: "api"},
				},
				Profiles: map[string]interface{}{},
			},
			profile: "nonexistent",
			expected: result{
				services: nil,
				error:    true,
			},
		},
		{
			name: "profile with wildcard returns all services",
			config: &Config{
				Services: map[string]*Service{
					"api": {Dir: "api"},
					"web": {Dir: "web"},
					"db":  {Dir: "db"},
				},
				Profiles: map[string]interface{}{
					"all": "*",
				},
			},
			profile: "all",
			expected: result{
				services: []string{"api", "web", "db"},
				error:    false,
			},
		},
		{
			name: "profile with single service as string",
			config: &Config{
				Services: map[string]*Service{
					"api": {Dir: "api"},
					"web": {Dir: "web"},
				},
				Profiles: map[string]interface{}{
					"api-only": "api",
				},
			},
			profile: "api-only",
			expected: result{
				services: []string{"api"},
				error:    false,
			},
		},
		{
			name: "profile with multiple services as array",
			config: &Config{
				Services: map[string]*Service{
					"api": {Dir: "api"},
					"web": {Dir: "web"},
					"db":  {Dir: "db"},
				},
				Profiles: map[string]interface{}{
					"backend": []interface{}{"api", "db"},
				},
			},
			profile: "backend",
			expected: result{
				services: []string{"api", "db"},
				error:    false,
			},
		},
		{
			name: "profile with empty array",
			config: &Config{
				Services: map[string]*Service{
					"api": {Dir: "api"},
				},
				Profiles: map[string]interface{}{
					"empty": []interface{}{},
				},
			},
			profile: "empty",
			expected: result{
				services: nil,
				error:    false,
			},
		},
		{
			name: "profile with mixed array types filters non-strings",
			config: &Config{
				Services: map[string]*Service{
					"api": {Dir: "api"},
					"web": {Dir: "web"},
				},
				Profiles: map[string]interface{}{
					"mixed": []interface{}{"api", 123, "web", nil},
				},
			},
			profile: "mixed",
			expected: result{
				services: []string{"api", "web"},
				error:    false,
			},
		},
		{
			name: "profile with unsupported type",
			config: &Config{
				Services: map[string]*Service{
					"api": {Dir: "api"},
				},
				Profiles: map[string]interface{}{
					"invalid": 123,
				},
			},
			profile: "invalid",
			expected: result{
				services: nil,
				error:    true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			services, err := tt.config.GetServicesForProfile(tt.profile)

			if tt.expected.error {
				assert.Error(t, err)
				assert.Nil(t, services)
				return
			}

			assert.NoError(t, err)
			if tt.profile == "all" {
				assert.ElementsMatch(t, tt.expected.services, services)
				return
			}

			if tt.expected.services == nil {
				assert.Nil(t, services)
				return
			}

			assert.Equal(t, tt.expected.services, services)
		})
	}
}
