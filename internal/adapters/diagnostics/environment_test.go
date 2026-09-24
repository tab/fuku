package diagnostics

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_NewEnvironment(t *testing.T) {
	e := NewEnvironment()

	assert.NotNil(t, e)
}

func Test_Environment_Getenv(t *testing.T) {
	t.Setenv("FUKU_DIAGNOSTICS_SET", "value")
	t.Setenv("FUKU_DIAGNOSTICS_EMPTY", "")

	subject := NewEnvironment()

	tests := []struct {
		name     string
		key      string
		expected string
	}{
		{
			name:     "set variable",
			key:      "FUKU_DIAGNOSTICS_SET",
			expected: "value",
		},
		{
			name:     "empty variable",
			key:      "FUKU_DIAGNOSTICS_EMPTY",
			expected: "",
		},
		{
			name:     "unset variable",
			key:      "FUKU_DIAGNOSTICS_UNSET",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, subject.Getenv(tt.key))
		})
	}
}

func Test_Environment_Executable(t *testing.T) {
	subject := NewEnvironment()

	exe, err := subject.Executable()

	require.NoError(t, err)
	assert.True(t, filepath.IsAbs(exe))
}

func Test_Environment_PathExecutable(t *testing.T) {
	dir := t.TempDir()
	binary := filepath.Join(dir, "fuku")
	require.NoError(t, os.WriteFile(binary, []byte("#!/bin/sh\n"), 0o755))
	t.Setenv("PATH", dir)

	subject := NewEnvironment()

	got, err := subject.PathExecutable()

	require.NoError(t, err)
	assert.Equal(t, binary, got)
}

func Test_Environment_PathExecutable_Absent(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	subject := NewEnvironment()

	got, err := subject.PathExecutable()

	require.Error(t, err)
	assert.Empty(t, got)
}
