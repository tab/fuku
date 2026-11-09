package mcp

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fuku/internal/adapters/instance"
)

func Test_configFile(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)

	sub := filepath.Join(root, "sub")

	t.Setenv(envProjectDir, root)

	tests := []struct {
		name     string
		files    []string
		links    []string
		config   string
		dir      string
		expected string
	}{
		{
			name: "the default lookup",
			dir:  root,
		},
		{
			name:   "the root fuku.yaml",
			files:  []string{"fuku.yaml"},
			config: "fuku.yaml",
			dir:    root,
		},
		{
			name:   "the root fuku.yaml from the current directory",
			files:  []string{"fuku.yaml"},
			config: "./fuku.yaml",
			dir:    root,
		},
		{
			name:   "an absolute path to the root fuku.yaml",
			files:  []string{"fuku.yaml"},
			config: filepath.Join(root, "fuku.yaml"),
			dir:    root,
		},
		{
			name:   "the root fuku.yaml beside a fuku.yml",
			files:  []string{"fuku.yaml", "fuku.yml"},
			config: "fuku.yaml",
			dir:    root,
		},
		{
			name:   "the root fuku.yml without a fuku.yaml",
			files:  []string{"fuku.yml"},
			config: "fuku.yml",
			dir:    root,
		},
		{
			name:     "the root fuku.yaml without one beside a fuku.yml",
			files:    []string{"fuku.yml"},
			config:   "fuku.yaml",
			dir:      root,
			expected: filepath.Join(root, "fuku.yaml"),
		},
		{
			name:     "the root fuku.yml beside an unreadable fuku.yaml",
			files:    []string{"fuku.yml"},
			links:    []string{"fuku.yaml"},
			config:   "fuku.yml",
			dir:      root,
			expected: filepath.Join(root, "fuku.yml"),
		},
		{
			name:     "an unreadable root fuku.yaml",
			links:    []string{"fuku.yaml"},
			config:   "fuku.yaml",
			dir:      root,
			expected: filepath.Join(root, "fuku.yaml"),
		},
		{
			name:     "the root fuku.yml beside a fuku.yaml",
			files:    []string{"fuku.yaml", "fuku.yml"},
			config:   "fuku.yml",
			dir:      root,
			expected: filepath.Join(root, "fuku.yml"),
		},
		{
			name:     "another file in the root",
			files:    []string{"fuku.yaml"},
			config:   "custom.yaml",
			dir:      root,
			expected: filepath.Join(root, "custom.yaml"),
		},
		{
			name:     "a fuku.yaml below the root",
			files:    []string{"fuku.yaml"},
			config:   "sub/fuku.yaml",
			dir:      sub,
			expected: filepath.Join(sub, "fuku.yaml"),
		},
		{
			name:     "a relative path",
			config:   "sub/custom.yaml",
			dir:      sub,
			expected: filepath.Join(sub, "custom.yaml"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, file := range tt.files {
				path := filepath.Join(root, file)
				require.NoError(t, os.WriteFile(path, nil, 0o600))
				t.Cleanup(func() { require.NoError(t, os.Remove(path)) })
			}

			for _, link := range tt.links {
				path := filepath.Join(root, link)
				require.NoError(t, os.Symlink(link, path))
				t.Cleanup(func() { require.NoError(t, os.Remove(path)) })
			}

			result := configFile(tt.config, tt.dir)

			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_projectDir(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)

	wd, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)

	other, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)

	link := filepath.Join(t.TempDir(), "link")
	require.NoError(t, os.Symlink(root, link))
	require.NoError(t, os.Mkdir(filepath.Join(root, "sub"), 0o755))
	require.NoError(t, os.Mkdir(filepath.Join(wd, "sub"), 0o755))
	t.Chdir(wd)

	tests := []struct {
		name          string
		env           string
		config        string
		expected      string
		expectedError error
	}{
		{
			name:     "the working directory without the variable",
			expected: wd,
		},
		{
			name:     "the variable over the working directory",
			env:      root,
			expected: root,
		},
		{
			name:     "the variable through a symlink",
			env:      link,
			expected: root,
		},
		{
			name:     "a relative config under the variable",
			env:      root,
			config:   "sub/fuku.yaml",
			expected: filepath.Join(root, "sub"),
		},
		{
			name:     "a relative config in the working directory",
			config:   "sub/fuku.yaml",
			expected: filepath.Join(wd, "sub"),
		},
		{
			name:     "a bare config file name under the variable",
			env:      root,
			config:   "custom.yaml",
			expected: root,
		},
		{
			name:     "an absolute config over the variable",
			env:      root,
			config:   filepath.Join(other, "fuku.yaml"),
			expected: other,
		},
		{
			name:          "a missing config directory",
			env:           root,
			config:        "missing/fuku.yaml",
			expectedError: instance.ErrFailedToResolveProject,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(envProjectDir, tt.env)

			result, err := projectDir(tt.config)

			require.ErrorIs(t, err, tt.expectedError)
			assert.Equal(t, tt.expected, result)
		})
	}
}
