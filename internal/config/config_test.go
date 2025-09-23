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
	assert.NotNil(t, cfg.Scopes)
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
scopes:
  test-scope:
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
