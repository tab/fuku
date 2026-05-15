package updater

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_isNewer(t *testing.T) {
	tests := []struct {
		name    string
		current string
		latest  string
		expect  bool
	}{
		{
			name:    "newer minor",
			current: "v0.19.1",
			latest:  "v0.20.0",
			expect:  true,
		},
		{
			name:    "newer patch",
			current: "v0.19.1",
			latest:  "v0.19.2",
			expect:  true,
		},
		{
			name:    "newer major",
			current: "v0.19.1",
			latest:  "v1.0.0",
			expect:  true,
		},
		{
			name:    "equal versions",
			current: "v0.19.1",
			latest:  "v0.19.1",
			expect:  false,
		},
		{
			name:    "older latest",
			current: "v0.19.1",
			latest:  "v0.19.0",
			expect:  false,
		},
		{
			name:    "missing v prefix on both",
			current: "0.19.1",
			latest:  "0.20.0",
			expect:  true,
		},
		{
			name:    "missing v prefix on current",
			current: "0.19.1",
			latest:  "v0.20.0",
			expect:  true,
		},
		{
			name:    "missing v prefix on latest",
			current: "v0.19.1",
			latest:  "0.20.0",
			expect:  true,
		},
		{
			name:    "invalid current",
			current: "not-a-version",
			latest:  "v0.20.0",
			expect:  false,
		},
		{
			name:    "invalid latest",
			current: "v0.19.1",
			latest:  "garbage",
			expect:  false,
		},
		{
			name:    "empty current",
			current: "",
			latest:  "v0.20.0",
			expect:  false,
		},
		{
			name:    "empty latest",
			current: "v0.19.1",
			latest:  "",
			expect:  false,
		},
		{
			name:    "both empty",
			current: "",
			latest:  "",
			expect:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expect, isNewer(tt.current, tt.latest))
		})
	}
}
