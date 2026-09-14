package bootstrap

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_Run(t *testing.T) {
	tests := []struct {
		name   string
		args   []string
		expect int
	}{
		{
			name:   "parse error",
			args:   []string{"--no-such-flag"},
			expect: 1,
		},
		{
			name:   "standalone version",
			args:   []string{"version"},
			expect: 0,
		},
		{
			name:   "doctor without config",
			args:   []string{"doctor", "--summary"},
			expect: 2,
		},
		{
			name:   "run without services",
			args:   []string{"run"},
			expect: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Chdir(t.TempDir())

			result := Run(tt.args, "")

			assert.Equal(t, tt.expect, result)
		})
	}
}
