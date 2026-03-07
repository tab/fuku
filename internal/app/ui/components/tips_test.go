package components

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_Tip_Render(t *testing.T) {
	theme := DefaultTheme()

	tests := []struct {
		name     string
		tip      Tip
		contains []string
	}{
		{
			name: "Key and description segments",
			tip: Tip{
				Segments: []TipSegment{
					desc("press "),
					key("q"),
					desc(" to quit"),
				},
			},
			contains: []string{"press", "q", "to quit"},
		},
		{
			name:     "Empty segments",
			tip:      Tip{Segments: []TipSegment{}},
			contains: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.tip.Render(theme)

			for _, c := range tt.contains {
				assert.Contains(t, result, c)
			}
		})
	}
}

func Test_Tips_NotEmpty(t *testing.T) {
	assert.NotEmpty(t, Tips)

	theme := DefaultTheme()

	for _, tip := range Tips {
		assert.NotEmpty(t, tip.Segments)
		assert.NotEmpty(t, tip.Render(theme))
	}
}
