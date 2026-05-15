package updater

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_readCache(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		write    func(t *testing.T, path string)
		expect   cache
		expectOk bool
	}{
		{
			name: "fresh entry",
			write: func(t *testing.T, path string) {
				entry := cache{Tag: "v0.20.0", FetchedAt: now.Add(-time.Hour)}
				raw, err := json.Marshal(entry)
				require.NoError(t, err)
				require.NoError(t, os.WriteFile(path, raw, 0o600))
			},
			expect:   cache{Tag: "v0.20.0"},
			expectOk: true,
		},
		{
			name:     "missing file",
			write:    func(t *testing.T, path string) {},
			expect:   cache{},
			expectOk: false,
		},
		{
			name: "corrupt json",
			write: func(t *testing.T, path string) {
				require.NoError(t, os.WriteFile(path, []byte("{not json"), 0o600))
			},
			expect:   cache{},
			expectOk: false,
		},
		{
			name: "expired entry",
			write: func(t *testing.T, path string) {
				entry := cache{Tag: "v0.20.0", FetchedAt: now.Add(-25 * time.Hour)}
				raw, err := json.Marshal(entry)
				require.NoError(t, err)
				require.NoError(t, os.WriteFile(path, raw, 0o600))
			},
			expect:   cache{},
			expectOk: false,
		},
		{
			name: "empty tag",
			write: func(t *testing.T, path string) {
				entry := cache{Tag: "", FetchedAt: now}
				raw, err := json.Marshal(entry)
				require.NoError(t, err)
				require.NoError(t, os.WriteFile(path, raw, 0o600))
			},
			expect:   cache{},
			expectOk: false,
		},
		{
			name: "zero fetched_at",
			write: func(t *testing.T, path string) {
				entry := cache{Tag: "v0.20.0"}
				raw, err := json.Marshal(entry)
				require.NoError(t, err)
				require.NoError(t, os.WriteFile(path, raw, 0o600))
			},
			expect:   cache{},
			expectOk: false,
		},
		{
			name: "invalid semver tag",
			write: func(t *testing.T, path string) {
				entry := cache{Tag: "not-a-version", FetchedAt: now.Add(-time.Hour)}
				raw, err := json.Marshal(entry)
				require.NoError(t, err)
				require.NoError(t, os.WriteFile(path, raw, 0o600))
			},
			expect:   cache{},
			expectOk: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "version.json")

			tt.write(t, path)

			entry, ok := readCache(path)
			assert.Equal(t, tt.expectOk, ok)
			assert.Equal(t, tt.expect.Tag, entry.Tag)
		})
	}
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
			dir := t.TempDir()
			path := filepath.Join(dir, "nested", "subdir", "version.json")

			err := writeCache(path, tt.entry)
			require.NoError(t, err)

			info, err := os.Stat(path)
			require.NoError(t, err)
			assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())

			raw, err := os.ReadFile(path)
			require.NoError(t, err)

			var got cache
			require.NoError(t, json.Unmarshal(raw, &got))
			assert.Equal(t, tt.entry.Tag, got.Tag)
		})
	}
}

func Test_writeCache_Roundtrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "fuku", "version.json")

	entry := cache{Tag: "v0.20.0", FetchedAt: time.Now().Add(-time.Hour).Truncate(time.Second)}

	require.NoError(t, writeCache(path, entry))

	got, ok := readCache(path)
	require.True(t, ok)
	assert.Equal(t, entry.Tag, got.Tag)
	assert.WithinDuration(t, entry.FetchedAt, got.FetchedAt, time.Second)
}

func Test_cachePath(t *testing.T) {
	path, err := cachePath()
	require.NoError(t, err)
	assert.Contains(t, path, filepath.Join("fuku", "version.json"))
}
