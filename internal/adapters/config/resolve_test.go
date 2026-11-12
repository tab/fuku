package config

import (
	"os"
	"path/filepath"
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
		expectedToken    string
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
			name: "explicit path with an override that cannot be checked carries the error",
			before: func(t *testing.T) {
				t.Chdir(t.TempDir())
				require.NoError(t, os.WriteFile("custom.yaml", []byte("version: 1"), 0600))
				require.NoError(t, os.Symlink("fuku.override.yaml", "fuku.override.yaml"))
			},
			path:         "custom.yaml",
			expectErr:    contracts.ErrFailedToReadConfig,
			expectedPath: "custom.yaml",
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
		{
			name: "invalid config keeps the API token",
			before: func(t *testing.T) {
				t.Chdir(t.TempDir())
				require.NoError(t, os.WriteFile("fuku.yaml", []byte("version: 1\nconcurrency:\n  workers: 0\nserver:\n  auth:\n    token: secret\n"), 0600))
			},
			expectErr:     contracts.ErrInvalidConfig,
			expectedPath:  "fuku.yaml",
			expectedToken: "secret",
		},
		{
			name: "undecodable workers keep the override API token",
			before: func(t *testing.T) {
				t.Chdir(t.TempDir())
				require.NoError(t, os.WriteFile("fuku.yaml", []byte("version: 1\nconcurrency:\n  workers: abc\nserver:\n  auth:\n    token: base\n"), 0600))
				require.NoError(t, os.WriteFile("fuku.override.yaml", []byte("server:\n  auth:\n    token: secret\n"), 0600))
			},
			expectErr:        ErrFailedToParseConfig,
			expectedPath:     "fuku.yaml",
			expectedOverride: "fuku.override.yaml",
			expectedToken:    "secret",
		},
		{
			name: "undecodable workers keep a mixed-case API token",
			before: func(t *testing.T) {
				t.Chdir(t.TempDir())
				require.NoError(t, os.WriteFile("fuku.yaml", []byte("version: 1\nconcurrency:\n  workers: abc\nServer:\n  Auth:\n    Token: secret\n"), 0600))
			},
			expectErr:     ErrFailedToParseConfig,
			expectedPath:  "fuku.yaml",
			expectedToken: "secret",
		},
		{
			name: "undecodable duration keeps the API token",
			before: func(t *testing.T) {
				t.Chdir(t.TempDir())
				require.NoError(t, os.WriteFile("fuku.yaml", []byte("version: 1\nservices:\n  api:\n    dir: api\n    readiness:\n      type: http\n      timeout: soon\nserver:\n  auth:\n    token: secret\n"), 0600))
			},
			expectErr:     ErrFailedToParseConfig,
			expectedPath:  "fuku.yaml",
			expectedToken: "secret",
		},
		{
			name: "yaml syntax error keeps no API token",
			before: func(t *testing.T) {
				t.Chdir(t.TempDir())
				require.NoError(t, os.WriteFile("fuku.yaml", []byte("services: [\nserver:\n  auth:\n    token: secret\n"), 0600))
			},
			expectErr:    ErrFailedToParseConfig,
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
			assert.Equal(t, tt.expectedToken, result.Project.Server.Token)
			assert.Equal(t, tt.expectedTopology, result.Topology)
		})
	}
}

func Test_LoadDir(t *testing.T) {
	dir := t.TempDir()
	other := t.TempDir()
	wd := t.TempDir()

	base := "version: 1\nserver:\n  auth:\n    token: base\n"
	override := "server:\n  auth:\n    token: override\n"
	alt := "version: 1\nserver:\n  auth:\n    token: alt\n"
	custom := "version: 1\nserver:\n  auth:\n    token: custom\n"

	require.NoError(t, os.WriteFile(filepath.Join(wd, ConfigFile), []byte("version: 1\nserver:\n  auth:\n    token: wd\n"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(wd, OverrideConfigFile), []byte("server:\n  auth:\n    token: wd-override\n"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(other, ConfigFile), []byte("version: 1\nserver:\n  auth:\n    token: other\n"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(other, OverrideConfigFile), []byte(override), 0600))
	t.Chdir(wd)

	tests := []struct {
		name             string
		files            map[string]string
		path             string
		expectErr        error
		expectedPath     string
		expectedOverride string
		expectedToken    string
	}{
		{
			name:          "the default lookup reads the directory, not the working directory",
			files:         map[string]string{ConfigFile: base},
			expectedPath:  filepath.Join(dir, ConfigFile),
			expectedToken: "base",
		},
		{
			name:             "the default lookup merges the override",
			files:            map[string]string{ConfigFile: base, OverrideConfigFile: override},
			expectedPath:     filepath.Join(dir, ConfigFile),
			expectedOverride: filepath.Join(dir, OverrideConfigFile),
			expectedToken:    "override",
		},
		{
			name:          "the default lookup falls back to fuku.yml",
			files:         map[string]string{ConfigFileAlt: alt},
			expectedPath:  filepath.Join(dir, ConfigFileAlt),
			expectedToken: "alt",
		},
		{
			name:  "no default file in the directory loads the defaults",
			files: map[string]string{},
		},
		{
			name:             "a path naming the default file merges the override",
			files:            map[string]string{ConfigFile: base, OverrideConfigFile: override},
			path:             "./" + ConfigFile,
			expectedPath:     filepath.Join(dir, ConfigFile),
			expectedOverride: filepath.Join(dir, OverrideConfigFile),
			expectedToken:    "override",
		},
		{
			name:             "an absolute path naming the default file merges the override",
			files:            map[string]string{ConfigFile: base, OverrideConfigFile: override},
			path:             filepath.Join(dir, ConfigFile),
			expectedPath:     filepath.Join(dir, ConfigFile),
			expectedOverride: filepath.Join(dir, OverrideConfigFile),
			expectedToken:    "override",
		},
		{
			name:             "an unclean absolute path naming the default file merges the override",
			files:            map[string]string{ConfigFile: base, OverrideConfigFile: override},
			path:             dir + "/./" + ConfigFile,
			expectedPath:     filepath.Join(dir, ConfigFile),
			expectedOverride: filepath.Join(dir, OverrideConfigFile),
			expectedToken:    "override",
		},
		{
			name:             "a path naming fuku.yml beside a fuku.yaml skips the override",
			files:            map[string]string{ConfigFile: base, ConfigFileAlt: alt, OverrideConfigFile: override},
			path:             ConfigFileAlt,
			expectedPath:     filepath.Join(dir, ConfigFileAlt),
			expectedOverride: filepath.Join(dir, OverrideConfigFile),
			expectedToken:    "alt",
		},
		{
			name:             "a relative explicit path skips the override",
			files:            map[string]string{"custom.yaml": custom, OverrideConfigFile: override},
			path:             "custom.yaml",
			expectedPath:     filepath.Join(dir, "custom.yaml"),
			expectedOverride: filepath.Join(dir, OverrideConfigFile),
			expectedToken:    "custom",
		},
		{
			name:             "an absolute explicit path skips the override",
			files:            map[string]string{ConfigFile: base},
			path:             filepath.Join(other, ConfigFile),
			expectedPath:     filepath.Join(other, ConfigFile),
			expectedOverride: filepath.Join(other, OverrideConfigFile),
			expectedToken:    "other",
		},
		{
			name:         "a path naming a missing default file fails",
			files:        map[string]string{ConfigFileAlt: alt},
			path:         ConfigFile,
			expectErr:    contracts.ErrFailedToReadConfig,
			expectedPath: filepath.Join(dir, ConfigFile),
		},
		{
			name:             "an invalid config keeps the override API token",
			files:            map[string]string{ConfigFile: "version: 1\nconcurrency:\n  workers: 0\nserver:\n  auth:\n    token: base\n", OverrideConfigFile: override},
			expectErr:        contracts.ErrInvalidConfig,
			expectedPath:     filepath.Join(dir, ConfigFile),
			expectedOverride: filepath.Join(dir, OverrideConfigFile),
			expectedToken:    "override",
		},
		{
			name:          "an undecodable config keeps the API token",
			files:         map[string]string{ConfigFile: "version: 1\nconcurrency:\n  workers: abc\nserver:\n  auth:\n    token: base\n"},
			expectErr:     ErrFailedToParseConfig,
			expectedPath:  filepath.Join(dir, ConfigFile),
			expectedToken: "base",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for name, content := range tt.files {
				path := filepath.Join(dir, name)
				require.NoError(t, os.WriteFile(path, []byte(content), 0600))
				t.Cleanup(func() { require.NoError(t, os.Remove(path)) })
			}

			result := LoadDir(dir, tt.path)

			require.ErrorIs(t, result.Error, tt.expectErr)
			assert.Equal(t, tt.expectedPath, result.Path)
			assert.Equal(t, tt.expectedOverride, result.OverridePath)
			assert.Equal(t, tt.expectedToken, result.Project.Server.Token)
		})
	}
}
