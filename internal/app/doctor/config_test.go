package doctor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"fuku/internal/contracts"
	"fuku/internal/model"
)

func Test_configSection(t *testing.T) {
	section := configSection(&state{Config: model.Config{Path: "fuku.yaml"}})

	assert.Equal(t, "Configuration", section.Title)
	assert.Equal(t, []model.CheckID{model.CheckConfigFile, model.CheckConfigOverride, model.CheckConfigValidate, model.CheckConfigSettings}, []model.CheckID{
		section.Results[0].ID, section.Results[1].ID, section.Results[2].ID, section.Results[3].ID,
	})
}

func Test_checkConfigFile(t *testing.T) {
	tests := []struct {
		name     string
		state    *state
		expected model.Severity
	}{
		{
			name:     "missing config",
			state:    &state{},
			expected: model.SeverityFail,
		},
		{
			name:     "read or parse error",
			state:    &state{Config: model.Config{Path: "fuku.yaml", Error: assert.AnError}},
			expected: model.SeverityFail,
		},
		{
			name:     "validation error is not a load failure",
			state:    &state{Config: model.Config{Path: "fuku.yaml", Error: contracts.ErrInvalidConfig}},
			expected: model.SeverityOK,
		},
		{
			name:     "loaded ok",
			state:    &state{Config: model.Config{Path: "fuku.yaml"}},
			expected: model.SeverityOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := checkConfigFile(tt.state)

			assert.Equal(t, model.CheckConfigFile, r.ID)
			assert.Equal(t, tt.expected, r.Severity)
		})
	}
}

func Test_checkConfigOverride(t *testing.T) {
	tests := []struct {
		name             string
		state            *state
		expectedSeverity model.Severity
		expectedSummary  string
	}{
		{
			name:             "no override file",
			state:            &state{},
			expectedSeverity: model.SeverityIdle,
			expectedSummary:  "no override file present",
		},
		{
			name:             "override applied with default load",
			state:            &state{Config: model.Config{OverridePath: "fuku.override.yaml"}},
			expectedSeverity: model.SeverityOK,
			expectedSummary:  "override applied",
		},
		{
			name:             "explicit config bypasses override",
			state:            &state{Options: Options{ExplicitConfig: true}, Config: model.Config{OverridePath: "fuku.override.yaml"}},
			expectedSeverity: model.SeverityNote,
			expectedSummary:  "override file present but skipped (--config bypasses overrides)",
		},
		{
			name:             "load error with override present is unknown",
			state:            &state{Config: model.Config{OverridePath: "fuku.override.yaml", Error: assert.AnError}},
			expectedSeverity: model.SeverityIdle,
			expectedSummary:  "override merge status unknown (config did not load)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := checkConfigOverride(tt.state)

			assert.Equal(t, tt.expectedSeverity, r.Severity)
			assert.Equal(t, tt.expectedSummary, r.Summary)
		})
	}
}

func Test_checkConfigValidate(t *testing.T) {
	tests := []struct {
		name     string
		state    *state
		expected model.Severity
	}{
		{
			name:     "config did not load skips as idle",
			state:    &state{Config: model.Config{Error: assert.AnError}},
			expected: model.SeverityIdle,
		},
		{
			name:     "validation error from load reports fail",
			state:    &state{Config: model.Config{Error: contracts.ErrInvalidConfig}},
			expected: model.SeverityFail,
		},
		{
			name:     "loaded config is valid",
			state:    &state{},
			expected: model.SeverityOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := checkConfigValidate(tt.state)

			assert.Equal(t, tt.expected, r.Severity)
		})
	}
}

func Test_checkConfigSettings(t *testing.T) {
	tests := []struct {
		name     string
		state    *state
		expected model.Severity
	}{
		{
			name:     "config did not load reports idle",
			state:    &state{Config: model.Config{Error: assert.AnError}},
			expected: model.SeverityIdle,
		},
		{
			name: "loaded settings report ok",
			state: &state{Config: model.Config{Project: model.Project{
				Logging:     model.Logging{Level: "info", Format: "console"},
				Concurrency: model.Concurrency{Workers: 5},
				Retry:       model.Retry{Attempts: 3, Backoff: 500 * time.Millisecond},
				Logs:        model.Logs{Buffer: 1000, History: 5000},
			}}},
			expected: model.SeverityOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := checkConfigSettings(tt.state)

			assert.Equal(t, tt.expected, r.Severity)
		})
	}
}
