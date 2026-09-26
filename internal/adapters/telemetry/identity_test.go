package telemetry

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_loadTelemetryID_NoConfigDir(t *testing.T) {
	t.Setenv("HOME", "")
	t.Setenv("XDG_CONFIG_HOME", "")

	id := loadTelemetryID()

	assert.Empty(t, id)
}

func Test_loadTelemetryIDFromPath_Generates(t *testing.T) {
	tests := []struct {
		name   string
		before func(t *testing.T) string
	}{
		{
			name: "creates a new ID",
			before: func(t *testing.T) string {
				return filepath.Join(t.TempDir(), telemetryIDFile)
			},
		},
		{
			name: "ignores an empty file",
			before: func(t *testing.T) string {
				path := filepath.Join(t.TempDir(), telemetryIDFile)
				require.NoError(t, os.WriteFile(path, []byte("  \n"), 0o600))

				return path
			},
		},
		{
			name: "creates the parent directories",
			before: func(t *testing.T) string {
				return filepath.Join(t.TempDir(), "nested", "dir", telemetryIDFile)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := tt.before(t)

			id := loadTelemetryIDFromPath(path)

			_, err := uuid.Parse(id)
			require.NoError(t, err)

			data, err := os.ReadFile(path)
			require.NoError(t, err)
			assert.Equal(t, id+"\n", string(data))
		})
	}
}

func Test_loadTelemetryIDFromPath(t *testing.T) {
	tests := []struct {
		name     string
		before   func(t *testing.T) string
		expected string
	}{
		{
			name: "reads an existing ID",
			before: func(t *testing.T) string {
				path := filepath.Join(t.TempDir(), telemetryIDFile)
				require.NoError(t, os.WriteFile(path, []byte("existing-id\n"), 0o600))

				return path
			},
			expected: "existing-id",
		},
		{
			name: "returns empty when the directory cannot be created",
			before: func(t *testing.T) string {
				blockingFile := filepath.Join(t.TempDir(), "blocker")
				require.NoError(t, os.WriteFile(blockingFile, []byte("x"), 0o600))

				return filepath.Join(blockingFile, "subdir", telemetryIDFile)
			},
			expected: "",
		},
		{
			name: "returns empty when the file cannot be written",
			before: func(t *testing.T) string {
				dir := filepath.Join(t.TempDir(), "readonly")
				require.NoError(t, os.MkdirAll(dir, 0o500))

				return filepath.Join(dir, telemetryIDFile)
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := tt.before(t)

			id := loadTelemetryIDFromPath(path)

			assert.Equal(t, tt.expected, id)
		})
	}
}

func Test_loadTelemetryIDFromPath_StableAcrossReads(t *testing.T) {
	path := filepath.Join(t.TempDir(), telemetryIDFile)

	first := loadTelemetryIDFromPath(path)
	second := loadTelemetryIDFromPath(path)

	assert.Equal(t, first, second)
}

func Test_loadTelemetryIDFromPath_RecreatesAfterDeletion(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "fuku", telemetryIDFile)

	first := loadTelemetryIDFromPath(path)
	require.NotEmpty(t, first)

	require.NoError(t, os.RemoveAll(filepath.Join(dir, "fuku")))

	second := loadTelemetryIDFromPath(path)

	assert.NotEmpty(t, second)
	assert.NotEqual(t, first, second)
}
