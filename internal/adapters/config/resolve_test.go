package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fuku/internal/contracts"
	"fuku/internal/model"
)

func Test_LoadPath(t *testing.T) {
	tests := []struct {
		name             string
		before           func(t *testing.T)
		path             string
		expectErr        error
		expectedPath     string
		expectedOverride string
		expectedLevel    string
		expectedTopology model.Topology
	}{
		{
			name: "no config file loads the defaults",
			before: func(t *testing.T) {
				t.Chdir(t.TempDir())
			},
			expectedLevel: DefaultLogLevel,
			expectedTopology: model.Topology{
				Order:        []string{},
				TierServices: map[string][]string{},
			},
		},
		{
			name: "default file with override reports both paths",
			before: func(t *testing.T) {
				t.Chdir(t.TempDir())
				require.NoError(t, os.WriteFile("fuku.yaml", []byte("version: 1\nservices:\n  api:\n    dir: api\n"), 0600))
				require.NoError(t, os.WriteFile("fuku.override.yaml", []byte("logging:\n  level: debug\n"), 0600))
			},
			expectedPath:     "fuku.yaml",
			expectedOverride: "fuku.override.yaml",
			expectedLevel:    "debug",
			expectedTopology: model.Topology{
				Order:        []string{model.TierDefault},
				TierServices: map[string][]string{model.TierDefault: {"api"}},
			},
		},
		{
			name: "explicit path skips the override beside it",
			before: func(t *testing.T) {
				t.Chdir(t.TempDir())
				require.NoError(t, os.WriteFile("custom.yaml", []byte("version: 1"), 0600))
				require.NoError(t, os.WriteFile("fuku.override.yaml", []byte("logging:\n  level: debug\n"), 0600))
			},
			path:             "custom.yaml",
			expectedPath:     "custom.yaml",
			expectedOverride: "fuku.override.yaml",
			expectedLevel:    DefaultLogLevel,
			expectedTopology: model.Topology{
				Order:        []string{},
				TierServices: map[string][]string{},
			},
		},
		{
			name: "explicit path not found carries the error",
			before: func(t *testing.T) {
				t.Chdir(t.TempDir())
			},
			path:         "nonexistent.yaml",
			expectErr:    contracts.ErrFailedToReadConfig,
			expectedPath: "nonexistent.yaml",
		},
		{
			name: "invalid config carries the validation error",
			before: func(t *testing.T) {
				t.Chdir(t.TempDir())
				require.NoError(t, os.WriteFile("fuku.yaml", []byte("version: 1\nconcurrency:\n  workers: 0\n"), 0600))
			},
			expectErr:    contracts.ErrInvalidConfig,
			expectedPath: "fuku.yaml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.before(t)

			result := LoadPath(tt.path)

			require.ErrorIs(t, result.Error, tt.expectErr)
			assert.Equal(t, tt.expectedPath, result.Path)
			assert.Equal(t, tt.expectedOverride, result.OverridePath)
			assert.Equal(t, tt.expectedLevel, result.Project.Logging.Level)
			assert.Equal(t, tt.expectedTopology, result.Topology)
		})
	}
}
