package doctor

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_RenderText(t *testing.T) {
	report := sampleReport()

	var buf bytes.Buffer
	RenderText(&buf, report)

	out := buf.String()

	tests := []struct {
		name     string
		contains string
	}{
		{
			name:     "header includes version and platform",
			contains: "fuku doctor v0.99.0 · linux-amd64",
		},
		{
			name:     "notes header rendered when non-ok exists",
			contains: "Notes",
		},
		{
			name:     "non-ok results appear in notes block",
			contains: "services.dotenv",
		},
		{
			name:     "section title rendered",
			contains: "Configuration",
		},
		{
			name:     "section note appended with bullet separator",
			contains: "Services · active profile: dev · 5 services",
		},
		{
			name:     "ok glyph rendered",
			contains: "✓",
		},
		{
			name:     "warn glyph rendered",
			contains: "⚠",
		},
		{
			name:     "result details rendered",
			contains: "path                      fuku.yaml",
		},
		{
			name:     "remediation rendered",
			contains: "remediation               create the missing .env files",
		},
		{
			name:     "tally line rendered",
			contains: "2 ok · 1 idle · 0 notes · 1 warn · 0 fail",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Contains(t, out, tt.contains)
		})
	}
}

func Test_RenderText_NoNotes(t *testing.T) {
	report := &Report{
		FukuVersion: "1.0.0",
		Platform:    "linux-amd64",
		Sections: []Section{
			{Title: "Environment", Results: []Result{
				{ID: "system", Status: StatusOK, Summary: "ok"},
			}},
		},
	}

	var buf bytes.Buffer
	RenderText(&buf, report)

	assert.NotContains(t, buf.String(), "Notes")
}

func Test_RenderSummary(t *testing.T) {
	report := sampleReport()

	var buf bytes.Buffer
	RenderSummary(&buf, report)

	out := buf.String()

	tests := []struct {
		name           string
		contains       string
		notContains    string
		expectContains bool
	}{
		{
			name:           "header rendered",
			contains:       "fuku doctor v0.99.0",
			expectContains: true,
		},
		{
			name:           "result row rendered",
			contains:       "config.file",
			expectContains: true,
		},
		{
			name:           "tally rendered",
			contains:       "2 ok · 1 idle · 0 notes · 1 warn · 0 fail",
			expectContains: true,
		},
		{
			name:           "details suppressed",
			contains:       "path                      fuku.yaml",
			expectContains: false,
		},
		{
			name:           "notes block suppressed",
			contains:       "Notes\n",
			expectContains: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.expectContains {
				assert.Contains(t, out, tt.contains)
				return
			}

			assert.NotContains(t, out, tt.contains)
		})
	}
}

func Test_RenderJSON(t *testing.T) {
	report := sampleReport()

	var buf bytes.Buffer
	require.NoError(t, RenderJSON(&buf, report))

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

func Test_padRight(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		width    int
		expected string
	}{
		{
			name:     "shorter than width is padded",
			input:    "abc",
			width:    6,
			expected: "abc   ",
		},
		{
			name:     "exactly width is unchanged",
			input:    "abcdef",
			width:    6,
			expected: "abcdef",
		},
		{
			name:     "longer than width is unchanged",
			input:    "abcdefgh",
			width:    6,
			expected: "abcdefgh",
		},
		{
			name:     "empty string padded",
			input:    "",
			width:    3,
			expected: "   ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, padRight(tt.input, tt.width))
		})
	}
}

func Test_styledGlyph(t *testing.T) {
	tests := []struct {
		name     string
		status   Status
		contains string
	}{
		{
			name:     "ok glyph wrapped with ansi color",
			status:   StatusOK,
			contains: "\x1b[",
		},
		{
			name:     "fail glyph wrapped with ansi color",
			status:   StatusFail,
			contains: "\x1b[",
		},
		{
			name:     "ok glyph rune still present under color",
			status:   StatusOK,
			contains: "✓",
		},
		{
			name:     "unknown status falls back to plain glyph",
			status:   Status(99),
			contains: "?",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Contains(t, styledGlyph(tt.status), tt.contains)
		})
	}
}

func sampleReport() *Report {
	return &Report{
		SchemaVersion: 1,
		GeneratedAt:   time.Date(2026, 6, 6, 12, 0, 0, 0, time.UTC),
		FukuVersion:   "0.99.0",
		Platform:      "linux-amd64",
		Sections: []Section{
			{
				Title: "Configuration",
				Results: []Result{
					{
						ID:       "config.file",
						Category: "configuration",
						Status:   StatusOK,
						Summary:  "loaded",
						Details:  []Detail{{Key: "path", Value: "fuku.yaml"}},
					},
				},
			},
			{
				Title: "Services",
				Note:  "active profile: dev · 5 services",
				Results: []Result{
					{
						ID:          "services.dotenv",
						Category:    "services",
						Status:      StatusWarn,
						Summary:     "1 of 4 referenced .env files missing",
						Details:     []Detail{{Key: "auth/.env.local", Value: "MISSING"}},
						Remediation: "create the missing .env files or update env.files",
					},
				},
			},
			{
				Title: "Runtime",
				Results: []Result{
					{
						ID:       "runtime.sockets",
						Category: "runtime",
						Status:   StatusOK,
						Summary:  "no stale sockets",
					},
					{
						ID:       "runtime.instance",
						Category: "runtime",
						Status:   StatusIdle,
						Summary:  "no other fuku running",
					},
				},
			},
		},
	}
}
