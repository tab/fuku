package mcp

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_Binary_Execute(t *testing.T) {
	dir := t.TempDir()

	subject := NewBinary()

	tests := []struct {
		name          string
		args          []string
		expected      string
		expectedError string
	}{
		{
			name:     "the binary exits zero",
			args:     []string{"-test.run=^$"},
			expected: "PASS",
		},
		{
			name:          "the binary exits non-zero",
			args:          []string{"-test.unknown-flag"},
			expectedError: "fuku -test.unknown-flag failed: exit status 2: flag provided but not defined: -test.unknown-flag",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, err := subject.Execute(t.Context(), dir, tt.args...)

			if tt.expectedError != "" {
				assert.ErrorContains(t, err, tt.expectedError)

				return
			}

			require.NoError(t, err)
			assert.Contains(t, string(stdout), tt.expected)
		})
	}
}
