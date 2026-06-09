package doctor

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_timed(t *testing.T) {
	r := timed(func() Result {
		return Result{ID: "x", Status: StatusOK}
	})

	assert.Equal(t, "x", r.ID)
	assert.GreaterOrEqual(t, r.DurationMs, int64(0))
}

func Test_fileExists(t *testing.T) {
	dir := t.TempDir()

	filePath := filepath.Join(dir, "real.txt")
	require.NoError(t, os.WriteFile(filePath, []byte(""), 0600))

	subDir := filepath.Join(dir, "subdir")
	require.NoError(t, os.MkdirAll(subDir, 0755))

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
			assert.Equal(t, tt.expected, fileExists(tt.path))
		})
	}
}
