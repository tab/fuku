package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_LoadPath(t *testing.T) {
	t.Run("default config when path empty", func(t *testing.T) {
		t.Chdir(t.TempDir())

		cfg, topo, err := LoadPath("")

		require.NoError(t, err)
		assert.NotNil(t, cfg)
		assert.NotNil(t, topo)
	})

	t.Run("explicit path is loaded", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "custom.yaml")
		require.NoError(t, os.WriteFile(path, []byte("version: 1"), 0600))

		cfg, topo, err := LoadPath(path)

		require.NoError(t, err)
		assert.NotNil(t, cfg)
		assert.NotNil(t, topo)
	})

	t.Run("explicit path not found returns error", func(t *testing.T) {
		t.Chdir(t.TempDir())

		_, _, err := LoadPath("nonexistent.yaml")

		require.Error(t, err)
	})
}

func Test_ResolveConfigPath(t *testing.T) {
	tests := []struct {
		name     string
		explicit string
		setup    func(t *testing.T) string
		expected string
	}{
		{
			name:     "explicit path returned as-is",
			explicit: "custom.yaml",
			setup:    func(t *testing.T) string { return t.TempDir() },
			expected: "custom.yaml",
		},
		{
			name: "default fuku.yaml found",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				require.NoError(t, os.WriteFile(filepath.Join(dir, "fuku.yaml"), []byte("version: 1"), 0600))

				return dir
			},
			expected: "fuku.yaml",
		},
		{
			name: "alt fuku.yml found when fuku.yaml absent",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				require.NoError(t, os.WriteFile(filepath.Join(dir, "fuku.yml"), []byte("version: 1"), 0600))

				return dir
			},
			expected: "fuku.yml",
		},
		{
			name:     "no config returns empty",
			setup:    func(t *testing.T) string { return t.TempDir() },
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Chdir(tt.setup(t))

			path, err := ResolveConfigPath(tt.explicit)

			require.NoError(t, err)
			assert.Equal(t, tt.expected, path)
		})
	}
}

func Test_ResolveOverridePath(t *testing.T) {
	tests := []struct {
		name     string
		basePath string
		setup    func(t *testing.T) string
		expected string
	}{
		{
			name:     "empty base returns empty",
			basePath: "",
			setup:    func(t *testing.T) string { return t.TempDir() },
			expected: "",
		},
		{
			name:     "override yaml found",
			basePath: "fuku.yaml",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				require.NoError(t, os.WriteFile(filepath.Join(dir, "fuku.override.yaml"), []byte(""), 0600))

				return dir
			},
			expected: "fuku.override.yaml",
		},
		{
			name:     "override yml found when yaml absent",
			basePath: "fuku.yaml",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				require.NoError(t, os.WriteFile(filepath.Join(dir, "fuku.override.yml"), []byte(""), 0600))

				return dir
			},
			expected: "fuku.override.yml",
		},
		{
			name:     "no override returns empty",
			basePath: "fuku.yaml",
			setup:    func(t *testing.T) string { return t.TempDir() },
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Chdir(tt.setup(t))

			path, err := ResolveOverridePath(tt.basePath)

			require.NoError(t, err)
			assert.Equal(t, tt.expected, path)
		})
	}
}
