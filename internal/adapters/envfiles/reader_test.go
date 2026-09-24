package envfiles

import (
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fuku/internal/model"
)

func Test_NewReader(t *testing.T) {
	r := NewReader()

	assert.NotNil(t, r)
}

func Test_Reader_Read(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".env"), []byte("FOO=bar\n# comment\nBAZ=qux\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "empty.env"), []byte(""), 0o600))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".env.dir"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "config"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "config", ".env"), []byte("NESTED=1\n"), 0o600))

	subject := NewReader()

	tests := []struct {
		name        string
		file        string
		expected    []model.Env
		expectedErr error
	}{
		{
			name: "entries in declaration order",
			file: ".env",
			expected: []model.Env{
				{Key: "FOO", Value: "bar"},
				{Key: "BAZ", Value: "qux"},
			},
		},
		{
			name:     "empty file has no entries",
			file:     "empty.env",
			expected: nil,
		},
		{
			name: "nested relative path is read",
			file: "config/.env",
			expected: []model.Env{
				{Key: "NESTED", Value: "1"},
			},
		},
		{
			name:        "missing file",
			file:        ".env.local",
			expectedErr: fs.ErrNotExist,
		},
		{
			name:        "directory",
			file:        ".env.dir",
			expectedErr: syscall.EISDIR,
		},
		{
			name:        "parent traversal entry is rejected",
			file:        "../escape",
			expectedErr: ErrUnsafePath,
		},
		{
			name:        "absolute path entry is rejected",
			file:        "/etc/passwd",
			expectedErr: ErrUnsafePath,
		},
		{
			name:        "empty name is rejected",
			file:        "",
			expectedErr: ErrUnsafePath,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := subject.Read(dir, tt.file)

			require.ErrorIs(t, err, tt.expectedErr)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func Test_isSafeRelativePath(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{
			name:     "plain .env name",
			path:     ".env",
			expected: true,
		},
		{
			name:     "nested relative path",
			path:     "config/.env",
			expected: true,
		},
		{
			name:     "empty path",
			path:     "",
			expected: false,
		},
		{
			name:     "absolute path",
			path:     "/etc/passwd",
			expected: false,
		},
		{
			name:     "parent traversal at start",
			path:     "../escape",
			expected: false,
		},
		{
			name:     "parent traversal segment",
			path:     "config/../../escape",
			expected: false,
		},
		{
			name:     "literal parent",
			path:     "..",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, isSafeRelativePath(tt.path))
		})
	}
}
