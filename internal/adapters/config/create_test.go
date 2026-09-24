package config

import (
	"bytes"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fuku/internal/adapters/config/template"
)

func Test_Create(t *testing.T) {
	tests := []struct {
		name           string
		before         func(t *testing.T, dir string)
		file           string
		expectedExit   int
		expectedErr    error
		expectedOutput string
		expectedFile   string
	}{
		{
			name: "fuku.yaml already exists",
			before: func(t *testing.T, _ string) {
				require.NoError(t, os.WriteFile(ConfigFile, []byte("existing"), 0600))
			},
			file:           ConfigFile,
			expectedOutput: "fuku.yaml already exists\n",
			expectedFile:   "existing",
		},
		{
			name: "fuku.yml already exists",
			before: func(t *testing.T, _ string) {
				require.NoError(t, os.WriteFile(ConfigFileAlt, []byte("existing"), 0600))
			},
			file:           ConfigFileAlt,
			expectedOutput: "fuku.yml already exists\n",
			expectedFile:   "existing",
		},
		{
			name:           "created successfully",
			before:         func(*testing.T, string) {},
			file:           ConfigFile,
			expectedOutput: "Created fuku.yaml\n",
			expectedFile:   string(template.Content),
		},
		{
			name: "a directory it cannot search fails the check",
			before: func(t *testing.T, dir string) {
				require.NoError(t, os.Chmod(dir, 0444))
				t.Cleanup(func() { _ = os.Chmod(dir, 0755) })
			},
			file:         ConfigFile,
			expectedExit: 1,
			expectedErr:  os.ErrPermission,
		},
		{
			name: "a read-only directory fails the write",
			before: func(t *testing.T, dir string) {
				require.NoError(t, os.Chmod(dir, 0555))
				t.Cleanup(func() { _ = os.Chmod(dir, 0755) })
			},
			file:         ConfigFile,
			expectedExit: 1,
			expectedErr:  os.ErrPermission,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var output bytes.Buffer

			dir := t.TempDir()
			t.Chdir(dir)
			tt.before(t, dir)

			exitCode, err := Create(&output)
			content, _ := os.ReadFile(tt.file)

			require.ErrorIs(t, err, tt.expectedErr)
			assert.Equal(t, tt.expectedExit, exitCode)
			assert.Equal(t, tt.expectedOutput, output.String())
			assert.Equal(t, tt.expectedFile, string(content))
		})
	}
}
