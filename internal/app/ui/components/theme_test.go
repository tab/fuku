package components

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_NewTheme(t *testing.T) {
	tests := []struct {
		name   string
		isDark bool
	}{
		{
			name:   "Dark theme",
			isDark: true,
		},
		{
			name:   "Light theme",
			isDark: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			theme := NewTheme(tt.isDark)

			assert.Equal(t, tt.isDark, theme.IsDark)
			assert.Len(t, theme.ServiceColorPalette, 24)
			assert.NotNil(t, theme.FgMuted)
			assert.NotNil(t, theme.FgBorder)
			assert.NotNil(t, theme.FgStatusRunning)
			assert.NotNil(t, theme.FgStatusWarning)
			assert.NotNil(t, theme.FgStatusError)
			assert.NotNil(t, theme.FgStatusStopped)
			assert.NotNil(t, theme.BgSelection)
		})
	}
}

func Test_DefaultTheme(t *testing.T) {
	theme := DefaultTheme()

	assert.True(t, theme.IsDark)
	assert.Len(t, theme.ServiceColorPalette, 24)
}

func Test_NewLogsServiceNameStyle(t *testing.T) {
	theme := DefaultTheme()

	style := theme.NewLogsServiceNameStyle(theme.ServiceColorPalette[0])

	assert.NotEmpty(t, style.Render("api"))
}
