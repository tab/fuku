package doctor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fuku/internal/model"
)

func Test_topologySection(t *testing.T) {
	tests := []struct {
		name         string
		state        *state
		expectedNote string
		expectedIDs  []model.CheckID
	}{
		{
			name:         "config did not load",
			state:        &state{Config: model.Config{Error: assert.AnError}},
			expectedNote: "skipped (config did not load)",
			expectedIDs:  []model.CheckID{model.CheckTopologyTiers, model.CheckTopologyProfile},
		},
		{
			name:        "loaded config runs both checks",
			state:       &state{Config: model.Config{Topology: model.Topology{Order: []string{model.TierDefault}}}, services: []string{"api"}},
			expectedIDs: []model.CheckID{model.CheckTopologyTiers, model.CheckTopologyProfile},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			section := topologySection(tt.state)

			assert.Equal(t, "Topology", section.Title)
			assert.Equal(t, tt.expectedNote, section.Note)
			require.Len(t, section.Results, len(tt.expectedIDs))

			for i, id := range tt.expectedIDs {
				assert.Equal(t, id, section.Results[i].ID)
			}
		})
	}
}

func Test_checkTiers(t *testing.T) {
	tests := []struct {
		name     string
		topology model.Topology
		expected model.Severity
	}{
		{
			name:     "default tier only",
			topology: model.Topology{Order: []string{model.TierDefault}},
			expected: model.SeverityIdle,
		},
		{
			name: "multiple tiers",
			topology: model.Topology{
				Order:        []string{"backend", "frontend"},
				TierServices: map[string][]string{"backend": {"api"}, "frontend": {"web"}},
			},
			expected: model.SeverityOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := checkTiers(&state{Config: model.Config{Topology: tt.topology}})

			assert.Equal(t, tt.expected, r.Severity)
		})
	}
}

func Test_checkProfileResolves(t *testing.T) {
	tests := []struct {
		name     string
		state    *state
		expected model.Severity
	}{
		{
			name:     "profile resolves to services",
			state:    &state{Options: Options{Profile: model.ProfileDefault}, services: []string{"api"}},
			expected: model.SeverityOK,
		},
		{
			name:     "profile resolves to zero services",
			state:    &state{Options: Options{Profile: model.ProfileDefault}, services: nil},
			expected: model.SeverityWarn,
		},
		{
			name:     "profile does not resolve",
			state:    &state{Options: Options{Profile: "nonexistent"}, profileErr: assert.AnError},
			expected: model.SeverityFail,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := checkProfileResolves(tt.state)

			assert.Equal(t, tt.expected, r.Severity)
		})
	}
}
