package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_DefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	assert.Equal(t, DefaultLogLevel, cfg.Logging.Level)
	assert.Equal(t, DefaultLogFormat, cfg.Logging.Format)
}

func Test_Load(t *testing.T) {
	tests := []struct {
		name          string
		configFile    string
		configContent string
		error         bool
		expected      *Config
	}{
		{
			name:     "Default config",
			error:    false,
			expected: DefaultConfig(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := Load()

			if tt.error {
				assert.Error(t, err)
				assert.Nil(t, cfg)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, cfg)
				assert.Equal(t, tt.expected.Logging.Level, cfg.Logging.Level)
			}
		})
	}
}
