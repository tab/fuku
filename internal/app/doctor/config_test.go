package doctor

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"fuku/internal/app/errors"
	"fuku/internal/config"
)

func Test_checkConfigFile(t *testing.T) {
	tests := []struct {
		name     string
		env      *Env
		expected Status
	}{
		{
			name:     "missing config",
			env:      &Env{},
			expected: StatusFail,
		},
		{
			name: "read or parse error",
			env: &Env{
				ConfigPath: "fuku.yaml",
				LoadErr:    assert.AnError,
			},
			expected: StatusFail,
		},
		{
			name: "validation error is not a load failure",
			env: &Env{
				ConfigPath: "fuku.yaml",
				LoadErr:    errors.ErrInvalidConfig,
			},
			expected: StatusOK,
		},
		{
			name: "loaded ok",
			env: &Env{
				ConfigPath: "fuku.yaml",
				Config:     config.DefaultConfig(),
			},
			expected: StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := checkConfigFile(tt.env)

			assert.Equal(t, tt.expected, r.Status)
			assert.Equal(t, "config.file", r.ID)
		})
	}
}

func Test_checkConfigOverride(t *testing.T) {
	tests := []struct {
		name            string
		env             *Env
		expectedStatus  Status
		expectedSummary string
	}{
		{
			name:            "no override file",
			env:             &Env{},
			expectedStatus:  StatusIdle,
			expectedSummary: "no override file present",
		},
		{
			name: "override applied with default load",
			env: &Env{
				OverridePath: "fuku.override.yaml",
				Config:       config.DefaultConfig(),
			},
			expectedStatus:  StatusOK,
			expectedSummary: "override applied",
		},
		{
			name: "explicit config bypasses override",
			env: &Env{
				OverridePath:   "fuku.override.yaml",
				Config:         config.DefaultConfig(),
				ExplicitConfig: true,
			},
			expectedStatus:  StatusNote,
			expectedSummary: "override file present but skipped (--config bypasses overrides)",
		},
		{
			name: "load error with override present is unknown",
			env: &Env{
				OverridePath: "fuku.override.yaml",
				LoadErr:      assert.AnError,
			},
			expectedStatus:  StatusIdle,
			expectedSummary: "override merge status unknown (config did not load)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := checkConfigOverride(tt.env)

			assert.Equal(t, tt.expectedStatus, r.Status)
			assert.Equal(t, tt.expectedSummary, r.Summary)
		})
	}
}

func Test_checkConfigValidate(t *testing.T) {
	tests := []struct {
		name     string
		env      *Env
		expected Status
	}{
		{
			name:     "config did not load skips as idle",
			env:      &Env{LoadErr: assert.AnError},
			expected: StatusIdle,
		},
		{
			name:     "validation error from load reports fail",
			env:      &Env{LoadErr: errors.ErrInvalidConfig},
			expected: StatusFail,
		},
		{
			name: "valid config",
			env: &Env{
				Config: config.DefaultConfig(),
			},
			expected: StatusOK,
		},
		{
			name: "invalid config supplied directly",
			env: &Env{
				Config: &config.Config{
					Concurrency: config.Concurrency{Workers: 0},
				},
			},
			expected: StatusFail,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := checkConfigValidate(tt.env)
			assert.Equal(t, tt.expected, r.Status)
		})
	}
}

func Test_checkConfigSettings(t *testing.T) {
	tests := []struct {
		name     string
		env      *Env
		expected Status
	}{
		{
			name:     "config nil reports idle",
			env:      &Env{},
			expected: StatusIdle,
		},
		{
			name:     "default config reports ok",
			env:      &Env{Config: config.DefaultConfig()},
			expected: StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := checkConfigSettings(tt.env)
			assert.Equal(t, tt.expected, r.Status)
		})
	}
}

// apiConfig builds a config with the built-in API enabled on the given address
func apiConfig(listen string) *config.Config {
	cfg := config.DefaultConfig()
	cfg.Server.Listen = listen
	cfg.Server.Auth.Token = "not-a-real-token"

	return cfg
}

func Test_checkConfigAPI(t *testing.T) {
	tests := []struct {
		name     string
		env      *Env
		expected Status
		summary  string
		remedy   bool
	}{
		{
			name:     "config nil reports idle",
			env:      &Env{},
			expected: StatusIdle,
			summary:  "skipped",
			remedy:   false,
		},
		{
			name:     "api not configured reports idle with a remediation",
			env:      &Env{Config: config.DefaultConfig()},
			expected: StatusIdle,
			summary:  "not configured",
			remedy:   true,
		},
		{
			name:     "api configured reports ok",
			env:      &Env{Config: apiConfig("127.0.0.1:9876")},
			expected: StatusOK,
			summary:  "enabled on 127.0.0.1:9876",
			remedy:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := checkConfigAPI(tt.env)

			assert.Equal(t, tt.expected, r.Status)
			assert.Equal(t, "config.api", r.ID)
			assert.Contains(t, r.Summary, tt.summary)
			assert.Equal(t, tt.remedy, r.Remediation != "")
		})
	}
}

func Test_checkConfigAPI_NeverReportsTheToken(t *testing.T) {
	token := "private-local-token"

	cfg := apiConfig("127.0.0.1:9876")
	cfg.Server.Auth.Token = token

	r := checkConfigAPI(&Env{Config: cfg})

	assert.NotContains(t, r.Summary, token)

	for _, detail := range r.Details {
		assert.NotContains(t, detail.Value, token)
	}

	assert.Contains(t, r.Details, Detail{Key: "auth token", Value: "set"})
}

func Test_checkConfigAPI_ReportsThePortFallbackRange(t *testing.T) {
	r := checkConfigAPI(&Env{Config: apiConfig("127.0.0.1:9876")})

	assert.Contains(t, r.Details, Detail{Key: "port fallback", Value: "9876-9885"})
}

func Test_portRange(t *testing.T) {
	tests := []struct {
		name     string
		listen   string
		expected string
	}{
		{name: "loopback address", listen: "127.0.0.1:9876", expected: "9876-9885"},
		{name: "any host", listen: ":1234", expected: "1234-1243"},
		{name: "missing port", listen: "127.0.0.1", expected: ""},
		{name: "named port", listen: "localhost:http", expected: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, portRange(tt.listen))
		})
	}
}
