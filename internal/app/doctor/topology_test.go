package doctor

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"fuku/internal/config"
)

func Test_checkTiers(t *testing.T) {
	tests := []struct {
		name     string
		topology *config.Topology
		expected Status
	}{
		{
			name:     "default tier only",
			topology: &config.Topology{HasDefaultOnly: true},
			expected: StatusIdle,
		},
		{
			name: "multiple tiers",
			topology: &config.Topology{
				Order:        []string{"backend", "frontend"},
				TierServices: map[string][]string{"backend": {"api"}, "frontend": {"web"}},
			},
			expected: StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := &Env{Config: config.DefaultConfig(), Topology: tt.topology}
			r := checkTiers(env)
			assert.Equal(t, tt.expected, r.Status)
		})
	}
}

func Test_checkProfileResolves(t *testing.T) {
	tests := []struct {
		name     string
		env      *Env
		expected Status
	}{
		{
			name:     "profile resolves to services",
			env:      &Env{Profile: config.Default, ProfileServices: []string{"api"}},
			expected: StatusOK,
		},
		{
			name:     "profile resolves to zero services",
			env:      &Env{Profile: config.Default, ProfileServices: nil},
			expected: StatusWarn,
		},
		{
			name:     "profile does not resolve",
			env:      &Env{Profile: "nonexistent", ProfileErr: assert.AnError},
			expected: StatusFail,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := checkProfileResolves(tt.env)
			assert.Equal(t, tt.expected, r.Status)
		})
	}
}
