package github

import (
	"encoding/json"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_DefaultCachePath(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/home/dev/.config")

	path := DefaultCachePath()

	assert.True(t, filepath.IsAbs(path))
	assert.Equal(t, filepath.Join("fuku", "version.json"), filepath.Join(filepath.Base(filepath.Dir(path)), filepath.Base(path)))
}

func Test_DefaultCachePath_NoConfigDir(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", "")

	path := DefaultCachePath()

	assert.Empty(t, path)
}

func Test_readCache(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name        string
		before      func(path string)
		expectedTag string
		expectedErr error
	}{
		{
			name: "fresh entry",
			before: func(path string) {
				raw, err := json.Marshal(cache{Tag: "v0.20.0", FetchedAt: now.Add(-time.Hour)})
				require.NoError(t, err)
				require.NoError(t, os.WriteFile(path, raw, 0o600))
			},
			expectedTag: "v0.20.0",
		},
		{
			name: "compatible JSON keys",
			before: func(path string) {
				require.NoError(t, os.WriteFile(path, []byte(`{"tag":"v0.21.0","fetched_at":"`+now.Add(-time.Hour).Format(time.RFC3339)+`"}`), 0o600))
			},
			expectedTag: "v0.21.0",
		},
		{
			name:   "missing file is legitimate miss",
			before: func(string) {},
		},
		{
			name: "a directory at the path fails the read",
			before: func(path string) {
				require.NoError(t, os.Mkdir(path, 0o700))
			},
			expectedErr: syscall.EISDIR,
		},
		{
			name: "expired entry is silent miss",
			before: func(path string) {
				raw, err := json.Marshal(cache{Tag: "v0.20.0", FetchedAt: now.Add(-25 * time.Hour)})
				require.NoError(t, err)
				require.NoError(t, os.WriteFile(path, raw, 0o600))
			},
		},
		{
			name: "empty tag is silent miss",
			before: func(path string) {
				raw, err := json.Marshal(cache{Tag: "", FetchedAt: now})
				require.NoError(t, err)
				require.NoError(t, os.WriteFile(path, raw, 0o600))
			},
		},
		{
			name: "zero fetched_at is silent miss",
			before: func(path string) {
				raw, err := json.Marshal(cache{Tag: "v0.20.0"})
				require.NoError(t, err)
				require.NoError(t, os.WriteFile(path, raw, 0o600))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "version.json")
			tt.before(path)

			entry, err := readCache(path)

			require.ErrorIs(t, err, tt.expectedErr)
			assert.Equal(t, tt.expectedTag, entry.Tag)
		})
	}
}

func Test_readCache_CorruptJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "version.json")
	require.NoError(t, os.WriteFile(path, []byte("{not json"), 0o600))

	entry, err := readCache(path)

	require.Error(t, err)
	assert.Equal(t, cache{}, entry)
}

func Test_writeCache(t *testing.T) {
	tests := []struct {
		name  string
		entry cache
	}{
		{
			name:  "writes valid entry",
			entry: cache{Tag: "v0.20.0", FetchedAt: time.Now()},
		},
		{
			name:  "writes empty entry",
			entry: cache{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "nested", "subdir", "version.json")

			err := writeCache(path, tt.entry)

			require.NoError(t, err)

			info, err := os.Stat(path)
			require.NoError(t, err)
			assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())

			dirInfo, err := os.Stat(filepath.Dir(path))
			require.NoError(t, err)
			assert.Equal(t, os.FileMode(0o700), dirInfo.Mode().Perm())

			raw, err := os.ReadFile(path)
			require.NoError(t, err)

			var got map[string]any
			require.NoError(t, json.Unmarshal(raw, &got))
			assert.Equal(t, tt.entry.Tag, got["tag"])
			assert.Contains(t, got, "fetched_at")
		})
	}
}

func Test_writeCache_UnwritableDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "readonly")
	require.NoError(t, os.Mkdir(dir, 0o500))

	entry := cache{Tag: "v0.20.0", FetchedAt: time.Now()}

	tests := []struct {
		name          string
		path          string
		expectedError string
	}{
		{
			name:          "nested cache dir cannot be created",
			path:          filepath.Join(dir, "fuku", "version.json"),
			expectedError: "create updater cache dir",
		},
		{
			name:          "cache file cannot be written",
			path:          filepath.Join(dir, "version.json"),
			expectedError: "write updater cache",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := writeCache(tt.path, entry)

			require.ErrorIs(t, err, os.ErrPermission)
			require.ErrorContains(t, err, tt.expectedError)
			assert.NoFileExists(t, tt.path)
		})
	}
}

func Test_writeCache_Roundtrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fuku", "version.json")

	entry := cache{Tag: "v0.20.0", FetchedAt: time.Now().Add(-time.Hour).Truncate(time.Second)}

	require.NoError(t, writeCache(path, entry))

	got, err := readCache(path)

	require.NoError(t, err)
	assert.Equal(t, entry.Tag, got.Tag)
	assert.WithinDuration(t, entry.FetchedAt, got.FetchedAt, time.Second)
}
