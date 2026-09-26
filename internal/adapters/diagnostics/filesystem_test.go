package diagnostics

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_Filesystem_Getwd(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	subject := NewFilesystem()

	got, err := subject.Getwd()

	require.NoError(t, err)

	expected, err := filepath.EvalSymlinks(dir)
	require.NoError(t, err)

	cwd, err := filepath.EvalSymlinks(got)
	require.NoError(t, err)
	assert.Equal(t, expected, cwd)
}

func Test_Filesystem_FileExists(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "real.txt")
	require.NoError(t, os.WriteFile(filePath, []byte(""), 0o600))

	subDir := filepath.Join(dir, "subdir")
	require.NoError(t, os.MkdirAll(subDir, 0o755))

	subject := NewFilesystem()

	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{
			name:     "existing file",
			path:     filePath,
			expected: true,
		},
		{
			name:     "directory returns false",
			path:     subDir,
			expected: false,
		},
		{
			name:     "missing path",
			path:     filepath.Join(dir, "missing.txt"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, subject.FileExists(tt.path))
		})
	}
}

func Test_Filesystem_DirExists(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "real.txt")
	require.NoError(t, os.WriteFile(filePath, []byte(""), 0o600))

	subDir := filepath.Join(dir, "subdir")
	require.NoError(t, os.MkdirAll(subDir, 0o755))

	subject := NewFilesystem()

	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{
			name:     "existing directory",
			path:     subDir,
			expected: true,
		},
		{
			name:     "file returns false",
			path:     filePath,
			expected: false,
		},
		{
			name:     "missing path",
			path:     filepath.Join(dir, "missing"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, subject.DirExists(tt.path))
		})
	}
}
