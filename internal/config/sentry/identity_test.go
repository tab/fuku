package sentry

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_LoadTelemetryID(t *testing.T) {
	tests := []struct {
		name   string
		before func(t *testing.T)
		expect bool
	}{
		{
			name: "Returns ID from config directory",
			before: func(t *testing.T) {
				t.Setenv("XDG_CONFIG_HOME", t.TempDir())
			},
			expect: true,
		},
		{
			name: "Returns empty when config directory unavailable",
			before: func(t *testing.T) {
				t.Setenv("HOME", "")
				t.Setenv("XDG_CONFIG_HOME", "")
			},
			expect: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.before(t)

			id := loadTelemetryID()

			if tt.expect {
				assert.NotEmpty(t, id)
			} else {
				assert.Empty(t, id)
			}
		})
	}
}

func Test_LoadTelemetryIDFromPath(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T) string
		check func(t *testing.T, id string, path string)
	}{
		{
			name: "Creates new ID",
			setup: func(t *testing.T) string {
				return filepath.Join(t.TempDir(), telemetryIDFile)
			},
			check: func(t *testing.T, id string, path string) {
				assert.NotEmpty(t, id)
				_, err := uuid.Parse(id)
				require.NoError(t, err)

				data, err := os.ReadFile(path)
				require.NoError(t, err)
				assert.Contains(t, string(data), id)
			},
		},
		{
			name: "Reads existing ID",
			setup: func(t *testing.T) string {
				path := filepath.Join(t.TempDir(), telemetryIDFile)
				require.NoError(t, os.WriteFile(path, []byte("existing-id\n"), 0o600))

				return path
			},
			check: func(t *testing.T, id string, _ string) {
				assert.Equal(t, "existing-id", id)
			},
		},
		{
			name: "Ignores empty file",
			setup: func(t *testing.T) string {
				path := filepath.Join(t.TempDir(), telemetryIDFile)
				require.NoError(t, os.WriteFile(path, []byte("  \n"), 0o600))

				return path
			},
			check: func(t *testing.T, id string, _ string) {
				assert.NotEmpty(t, id)
				_, err := uuid.Parse(id)
				require.NoError(t, err)
			},
		},
		{
			name: "Creates parent directories",
			setup: func(t *testing.T) string {
				return filepath.Join(t.TempDir(), "nested", "dir", telemetryIDFile)
			},
			check: func(t *testing.T, id string, path string) {
				assert.NotEmpty(t, id)
				assert.FileExists(t, path)
			},
		},
		{
			name: "Returns empty when mkdir fails",
			setup: func(t *testing.T) string {
				blockingFile := filepath.Join(t.TempDir(), "blocker")
				require.NoError(t, os.WriteFile(blockingFile, []byte("x"), 0o600))

				return filepath.Join(blockingFile, "subdir", telemetryIDFile)
			},
			check: func(t *testing.T, id string, _ string) {
				assert.Empty(t, id)
			},
		},
		{
			name: "Returns empty when write fails",
			setup: func(t *testing.T) string {
				dir := filepath.Join(t.TempDir(), "readonly")
				require.NoError(t, os.MkdirAll(dir, 0o500))

				return filepath.Join(dir, telemetryIDFile)
			},
			check: func(t *testing.T, id string, _ string) {
				assert.Empty(t, id)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := tt.setup(t)

			id := loadTelemetryIDFromPath(path)

			tt.check(t, id, path)
		})
	}
}

func Test_LoadTelemetryIDFromPath_StableAcrossReads(t *testing.T) {
	path := filepath.Join(t.TempDir(), telemetryIDFile)

	first := loadTelemetryIDFromPath(path)
	second := loadTelemetryIDFromPath(path)

	assert.Equal(t, first, second)
}

func Test_LoadTelemetryIDFromPath_RecreatesAfterDeletion(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, configDir, telemetryIDFile)

	first := loadTelemetryIDFromPath(path)
	require.NotEmpty(t, first)

	require.NoError(t, os.RemoveAll(filepath.Join(dir, configDir)))

	second := loadTelemetryIDFromPath(path)

	assert.NotEmpty(t, second)
	assert.NotEqual(t, first, second)
}

func Test_TelemetryIDPath(t *testing.T) {
	path, err := telemetryIDPath()

	require.NoError(t, err)
	assert.Contains(t, path, configDir)
	assert.Contains(t, path, telemetryIDFile)
}
