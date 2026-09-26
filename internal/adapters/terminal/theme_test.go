package terminal

import (
	"os"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_NewTheme(t *testing.T) {
	tests := []struct {
		name       string
		appearance Appearance
	}{
		{
			name:       "Dark theme",
			appearance: AppearanceDark,
		},
		{
			name:       "Light theme",
			appearance: AppearanceLight,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			theme := NewTheme(tt.appearance)

			assert.Equal(t, tt.appearance, theme.Appearance)
			assert.Len(t, theme.ServiceColorPalette, 24)
		})
	}
}

func Test_Appearance_Resolve(t *testing.T) {
	in, out, err := os.Pipe()
	require.NoError(t, err)

	closePipe := func() {
		in.Close()
		out.Close()
	}

	t.Cleanup(closePipe)

	tests := []struct {
		name       string
		appearance Appearance
		expected   Appearance
	}{
		{
			name:       "light stays light",
			appearance: AppearanceLight,
			expected:   AppearanceLight,
		},
		{
			name:       "dark stays dark",
			appearance: AppearanceDark,
			expected:   AppearanceDark,
		},
		{
			name:       "system without a terminal to ask falls back to dark",
			appearance: AppearanceSystem,
			expected:   AppearanceDark,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.appearance.Resolve(in, out)

			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_NewTheme_VersionStylesAdaptToMode(t *testing.T) {
	dark := NewTheme(AppearanceDark)
	light := NewTheme(AppearanceLight)

	assert.NotEqual(t,
		dark.CurrentVersionStyle.Render("v1.0.0"),
		light.CurrentVersionStyle.Render("v1.0.0"),
		"CurrentVersionStyle must render differently for light vs dark terminals")

	assert.NotEqual(t,
		dark.LatestVersionStyle.Render("v1.0.0"),
		light.LatestVersionStyle.Render("v1.0.0"),
		"LatestVersionStyle must render differently for light vs dark terminals")
}

func Test_NewTheme_AsideSectionTitleStyle(t *testing.T) {
	theme := NewTheme(AppearanceDark)

	assert.True(t, theme.AsideSectionTitleStyle.GetBold())
	assert.Equal(t, theme.PanelMutedStyle.GetForeground(), theme.AsideSectionTitleStyle.GetForeground())
}

func Test_NewTheme_BorderAndDoctorStylesAdaptToMode(t *testing.T) {
	dark := NewTheme(AppearanceDark)
	light := NewTheme(AppearanceLight)

	tests := []struct {
		name  string
		dark  lipgloss.Style
		light lipgloss.Style
	}{
		{
			name:  "muted panel border",
			dark:  dark.PanelMutedBorderStyle,
			light: light.PanelMutedBorderStyle,
		},
		{
			name:  "ok glyph",
			dark:  dark.DoctorGlyphOKStyle,
			light: light.DoctorGlyphOKStyle,
		},
		{
			name:  "idle glyph",
			dark:  dark.DoctorGlyphIdleStyle,
			light: light.DoctorGlyphIdleStyle,
		},
		{
			name:  "note glyph",
			dark:  dark.DoctorGlyphNoteStyle,
			light: light.DoctorGlyphNoteStyle,
		},
		{
			name:  "warn glyph",
			dark:  dark.DoctorGlyphWarnStyle,
			light: light.DoctorGlyphWarnStyle,
		},
		{
			name:  "fail glyph",
			dark:  dark.DoctorGlyphFailStyle,
			light: light.DoctorGlyphFailStyle,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotEqual(t, tt.dark.Render("x"), tt.light.Render("x"))
		})
	}
}

func Test_newLogsServiceNameStyle(t *testing.T) {
	theme := NewTheme(AppearanceDark)

	style := theme.newLogsServiceNameStyle(theme.ServiceColorPalette[0])

	assert.NotEmpty(t, style.Render("api"))
}
