package envfiles

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fuku/internal/model"
)

func Test_parse(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []model.Env
	}{
		{
			name:  "entries in declaration order",
			input: "FOO=bar\nBAZ=qux\n",
			expected: []model.Env{
				{Key: "FOO", Value: "bar"},
				{Key: "BAZ", Value: "qux"},
			},
		},
		{
			name:  "blank, comment and malformed lines are skipped",
			input: "\n# comment\nNOTAPAIR\nKEY=value\n",
			expected: []model.Env{
				{Key: "KEY", Value: "value"},
			},
		},
		{
			name:     "empty stream has no entries",
			input:    "",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parse(strings.NewReader(tt.input))

			require.NoError(t, err)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func Test_parse_LineTooLong(t *testing.T) {
	input := "KEY=" + strings.Repeat("x", 70*1024)

	got, err := parse(strings.NewReader(input))

	require.Error(t, err)
	assert.Nil(t, got)
}

func Test_parseLine(t *testing.T) {
	tests := []struct {
		name       string
		line       string
		expected   model.Env
		expectedOk bool
	}{
		{
			name:       "plain key value",
			line:       "FOO=bar",
			expected:   model.Env{Key: "FOO", Value: "bar"},
			expectedOk: true,
		},
		{
			name:       "value preserves spaces and special chars",
			line:       "URL=http://localhost:8080/path?q=1&r=2",
			expected:   model.Env{Key: "URL", Value: "http://localhost:8080/path?q=1&r=2"},
			expectedOk: true,
		},
		{
			name:       "export prefix is stripped",
			line:       "export FOO=bar",
			expected:   model.Env{Key: "FOO", Value: "bar"},
			expectedOk: true,
		},
		{
			name:       "leading whitespace is trimmed",
			line:       "   KEY=value",
			expected:   model.Env{Key: "KEY", Value: "value"},
			expectedOk: true,
		},
		{
			name:       "empty value is preserved",
			line:       "FLAG=",
			expected:   model.Env{Key: "FLAG", Value: ""},
			expectedOk: true,
		},
		{
			name:       "blank line skipped",
			line:       "",
			expectedOk: false,
		},
		{
			name:       "comment line skipped",
			line:       "# this is a comment",
			expectedOk: false,
		},
		{
			name:       "indented comment skipped",
			line:       "   # comment",
			expectedOk: false,
		},
		{
			name:       "line without equals skipped",
			line:       "NOTAPAIR",
			expectedOk: false,
		},
		{
			name:       "line with only equals skipped",
			line:       "=value",
			expectedOk: false,
		},
		{
			name:       "export with no key skipped",
			line:       "export =value",
			expectedOk: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseLine(tt.line)

			assert.Equal(t, tt.expectedOk, ok)
			assert.Equal(t, tt.expected, got)
		})
	}
}
