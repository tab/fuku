package cli

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fuku/internal/model"
)

func Test_JSON_Render(t *testing.T) {
	var buf bytes.Buffer

	report := sampleReport()

	err := NewJSON().Render(&buf, report)

	require.NoError(t, err)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &decoded))

	assert.InDelta(t, float64(1), decoded["schemaVersion"], 0)
	assert.Equal(t, "0.99.0", decoded["fukuVersion"])
	assert.Equal(t, "linux-amd64", decoded["platform"])
	assert.Equal(t, "warn", decoded["overallStatus"])
	assert.Equal(t, "2026-06-06T12:00:00Z", decoded["generatedAt"])

	tally, ok := decoded["tally"].(map[string]any)
	require.True(t, ok)
	assert.InDelta(t, float64(2), tally["ok"], 0)
	assert.InDelta(t, float64(1), tally["warn"], 0)

	checks, ok := decoded["checks"].(map[string]any)
	require.True(t, ok)
	assert.Contains(t, checks, "services.dotenv")
	assert.Contains(t, checks, "config.file")
	assert.Contains(t, checks, "runtime.sockets")
}

func Test_toJSONReport(t *testing.T) {
	report := sampleReport()

	got := toJSONReport(report)

	assert.Equal(t, jsonTally{OK: 2, Idle: 1, Warn: 1}, got.Tally)
	assert.Equal(t, "warn", got.OverallStatus)
	assert.Equal(t, []jsonSectionReference{
		{Title: "Configuration", Checks: []string{"config.file"}},
		{Title: "Services", Note: "active profile: dev · 5 services", Checks: []string{"services.dotenv"}},
		{Title: "Runtime", Checks: []string{"runtime.sockets", "runtime.instance"}},
	}, got.Sections)
	assert.Equal(t, jsonCheck{
		ID:          "services.dotenv",
		Category:    "services",
		Status:      "warn",
		Summary:     "1 of 4 referenced .env files missing",
		Details:     map[string]string{"auth/.env.local": "MISSING"},
		Remediation: "create the missing .env files or update env.files",
	}, got.Checks["services.dotenv"])
	assert.Nil(t, got.Checks["runtime.sockets"].Details)
}

func Test_detailsToMap(t *testing.T) {
	tests := []struct {
		name     string
		details  []model.Detail
		expected map[string]string
	}{
		{
			name:     "no details is nil",
			details:  nil,
			expected: nil,
		},
		{
			name:     "details keyed by name",
			details:  []model.Detail{{Key: "path", Value: "fuku.yaml"}, {Key: "error", Value: "boom"}},
			expected: map[string]string{"path": "fuku.yaml", "error": "boom"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, detailsToMap(tt.details))
		})
	}
}

// sampleReport is the report fixture the JSON tests share
func sampleReport() *model.Report {
	return &model.Report{
		SchemaVersion: 1,
		GeneratedAt:   time.Date(2026, 6, 6, 12, 0, 0, 0, time.UTC),
		Version:       "0.99.0",
		Platform:      "linux-amd64",
		Sections: []model.Section{
			{
				Title: "Configuration",
				Results: []model.Result{
					{
						ID:       model.CheckConfigFile,
						Category: model.CategoryConfiguration,
						Severity: model.SeverityOK,
						Summary:  "loaded",
						Details:  []model.Detail{{Key: "path", Value: "fuku.yaml"}},
					},
				},
			},
			{
				Title: "Services",
				Note:  "active profile: dev · 5 services",
				Results: []model.Result{
					{
						ID:          model.CheckServicesDotenv,
						Category:    model.CategoryServices,
						Severity:    model.SeverityWarn,
						Summary:     "1 of 4 referenced .env files missing",
						Details:     []model.Detail{{Key: "auth/.env.local", Value: "MISSING"}},
						Remediation: "create the missing .env files or update env.files",
					},
				},
			},
			{
				Title: "Runtime",
				Results: []model.Result{
					{
						ID:       model.CheckRuntimeSockets,
						Category: model.CategoryRuntime,
						Severity: model.SeverityOK,
						Summary:  "no stale sockets",
					},
					{
						ID:       model.CheckRuntimeInstance,
						Category: model.CategoryRuntime,
						Severity: model.SeverityIdle,
						Summary:  "no other fuku running",
					},
				},
			},
		},
	}
}
