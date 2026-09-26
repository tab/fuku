package tui

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fuku/internal/adapters/terminal"
	"fuku/internal/model"
)

func Test_Report_Render(t *testing.T) {
	theme := terminal.NewTheme(terminal.AppearanceDark)
	resolveTheme := func() terminal.Theme { return theme }

	report := &model.Report{
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
						Details:     []model.Detail{{Key: "auth/.env.local", Value: "MISSING"}, {Key: "サービス/.env", Value: "MISSING"}},
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

	renderer := NewReport(resolveTheme)

	var buf bytes.Buffer

	tests := []struct {
		name     string
		contains string
	}{
		{
			name:     "header includes version and platform",
			contains: "fuku doctor v0.99.0 · linux-amd64\n\n",
		},
		{
			name:     "notes block lists the non-ok results above the sections",
			contains: "Notes\n   " + styledGlyph(theme, model.SeverityWarn) + " services.dotenv 1 of 4 referenced .env files missing\n" + divider + "\n",
		},
		{
			name:     "section title rendered",
			contains: "\nConfiguration\n",
		},
		{
			name:     "section note appended with bullet separator",
			contains: "\nServices · active profile: dev · 5 services\n",
		},
		{
			name:     "ok row rendered with a styled glyph",
			contains: "  " + styledGlyph(theme, model.SeverityOK) + " config.file  loaded\n",
		},
		{
			name:     "idle row rendered with a styled glyph",
			contains: "  " + styledGlyph(theme, model.SeverityIdle) + " runtime.instance no other fuku running\n",
		},
		{
			name:     "result details rendered",
			contains: "      path                      fuku.yaml\n",
		},
		{
			name:     "wide detail key padded by display width",
			contains: "      サービス/.env             MISSING\n",
		},
		{
			name:     "remediation rendered",
			contains: "      remediation               create the missing .env files or update env.files\n",
		},
		{
			name:     "tally line rendered",
			contains: "\n" + divider + "\n2 ok · 1 idle · 0 notes · 1 warn · 0 fail\n",
		},
	}

	err := renderer.Render(&buf, report)

	require.NoError(t, err)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Contains(t, buf.String(), tt.contains)
		})
	}
}

func Test_Report_Render_NoNotes(t *testing.T) {
	theme := func() terminal.Theme { return terminal.NewTheme(terminal.AppearanceDark) }

	report := &model.Report{
		Version:  "1.0.0",
		Platform: "linux-amd64",
		Sections: []model.Section{
			{Title: "Environment", Results: []model.Result{
				{ID: model.CheckSystem, Severity: model.SeverityOK, Summary: "ok"},
			}},
			{Title: "Empty"},
		},
	}

	renderer := NewReport(theme)

	var buf bytes.Buffer

	err := renderer.Render(&buf, report)

	require.NoError(t, err)
	assert.NotContains(t, buf.String(), "Notes")
	assert.NotContains(t, buf.String(), "Empty")
}

func Test_Summary_Render(t *testing.T) {
	theme := terminal.NewTheme(terminal.AppearanceDark)
	resolveTheme := func() terminal.Theme { return theme }

	report := &model.Report{
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

	summary := NewSummary(resolveTheme)

	var buf bytes.Buffer

	err := summary.Render(&buf, report)

	require.NoError(t, err)
	assert.Equal(t, "fuku doctor v0.99.0 · linux-amd64\n"+
		"\n"+
		"\n"+
		"Configuration\n"+
		"  "+styledGlyph(theme, model.SeverityOK)+" config.file  loaded\n"+
		"\n"+
		"Services · active profile: dev · 5 services\n"+
		"  "+styledGlyph(theme, model.SeverityWarn)+" services.dotenv 1 of 4 referenced .env files missing\n"+
		"\n"+
		"Runtime\n"+
		"  "+styledGlyph(theme, model.SeverityOK)+" runtime.sockets no stale sockets\n"+
		"  "+styledGlyph(theme, model.SeverityIdle)+" runtime.instance no other fuku running\n"+
		"\n"+
		divider+"\n"+
		"2 ok · 1 idle · 0 notes · 1 warn · 0 fail\n", buf.String())
}

func Test_styledGlyph(t *testing.T) {
	theme := terminal.NewTheme(terminal.AppearanceDark)

	tests := []struct {
		name     string
		severity model.Severity
		expected string
	}{
		{
			name:     "ok glyph takes the ok style",
			severity: model.SeverityOK,
			expected: theme.DoctorGlyphOKStyle.Render("✓"),
		},
		{
			name:     "idle glyph takes the idle style",
			severity: model.SeverityIdle,
			expected: theme.DoctorGlyphIdleStyle.Render("○"),
		},
		{
			name:     "note glyph takes the note style",
			severity: model.SeverityNote,
			expected: theme.DoctorGlyphNoteStyle.Render("↑"),
		},
		{
			name:     "warn glyph takes the warn style",
			severity: model.SeverityWarn,
			expected: theme.DoctorGlyphWarnStyle.Render("⚠"),
		},
		{
			name:     "fail glyph takes the fail style",
			severity: model.SeverityFail,
			expected: theme.DoctorGlyphFailStyle.Render("✗"),
		},
		{
			name:     "unknown severity falls back to a plain glyph",
			severity: model.Severity(99),
			expected: "?",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := styledGlyph(theme, tt.severity)

			assert.Equal(t, tt.expected, result)
		})
	}
}
