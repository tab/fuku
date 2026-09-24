package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_Topology_DefaultOnly(t *testing.T) {
	tests := []struct {
		name     string
		topology Topology
		expected bool
	}{
		{
			name:     "empty order",
			topology: Topology{},
			expected: true,
		},
		{
			name:     "default only",
			topology: Topology{Order: []string{TierDefault}},
			expected: true,
		},
		{
			name:     "other tiers",
			topology: Topology{Order: []string{"foundation", TierDefault}},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.topology.DefaultOnly()

			assert.Equal(t, tt.expected, result)
		})
	}
}
