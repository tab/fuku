package sentry

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_LoadTelemetryID_CreatesNewID(t *testing.T) {
	path := filepath.Join(t.TempDir(), telemetryIDFile)

	id := loadTelemetryIDFromPath(path)

	assert.NotEmpty(t, id)
	_, err := uuid.Parse(id)
	require.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(data), id)
}

func Test_LoadTelemetryID_ReadsExistingID(t *testing.T) {
	path := filepath.Join(t.TempDir(), telemetryIDFile)

	require.NoError(t, os.WriteFile(path, []byte("existing-id\n"), 0o600))

	id := loadTelemetryIDFromPath(path)

	assert.Equal(t, "existing-id", id)
}

func Test_LoadTelemetryID_IgnoresEmptyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), telemetryIDFile)

	require.NoError(t, os.WriteFile(path, []byte("  \n"), 0o600))

	id := loadTelemetryIDFromPath(path)

	assert.NotEmpty(t, id)
	_, err := uuid.Parse(id)
	require.NoError(t, err)
}

func Test_LoadTelemetryID_CreatesParentDir(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "dir", telemetryIDFile)

	id := loadTelemetryIDFromPath(path)

	assert.NotEmpty(t, id)
	assert.FileExists(t, path)
}

func Test_LoadTelemetryID_StableAcrossReads(t *testing.T) {
	path := filepath.Join(t.TempDir(), telemetryIDFile)

	first := loadTelemetryIDFromPath(path)
	second := loadTelemetryIDFromPath(path)

	assert.Equal(t, first, second)
}

func Test_LoadTelemetryID_RecreatesAfterDeletion(t *testing.T) {
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

func Test_LoadTelemetryID(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	id := loadTelemetryID()

	assert.NotEmpty(t, id)
}
