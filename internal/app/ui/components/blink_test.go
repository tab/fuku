package components

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/stretchr/testify/assert"
)

func Test_NewBlink(t *testing.T) {
	b := NewBlink()

	assert.NotNil(t, b)
	assert.False(t, b.IsActive())
	// Frame can be either empty or full due to random initial state
	frame := b.Frame()
	assert.True(t, frame == empty || frame == full)
}

func Test_Blink_Start(t *testing.T) {
	b := NewBlink()
	b.Start()

	assert.True(t, b.IsActive())
}

func Test_Blink_Stop(t *testing.T) {
	b := NewBlink()
	b.Start()
	b.Stop()

	assert.False(t, b.IsActive())
	assert.Equal(t, empty, b.Frame())
}

func Test_Blink_Frame_WhenInactive(t *testing.T) {
	b := NewBlink()

	// Frame can be either empty or full due to random initial state
	frame := b.Frame()
	assert.True(t, frame == empty || frame == full)
}

func Test_Blink_Update_Progression(t *testing.T) {
	b := NewBlink()
	b.Start()

	initialFrame := b.Frame()

	for i := 0; i < 100; i++ {
		b.Update()
	}

	laterFrame := b.Frame()

	assert.True(t, b.IsActive())
	assert.NotEmpty(t, initialFrame)
	assert.NotEmpty(t, laterFrame)
}

func Test_Blink_Render(t *testing.T) {
	b := NewBlink()
	style := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFF00"))

	result := b.Render(style)

	assert.NotEmpty(t, result)
}

func Test_Blink_Frame_Transitions(t *testing.T) {
	b := NewBlink()
	b.Start()

	frames := make(map[string]bool)

	for i := 0; i < 200; i++ {
		b.Update()
		frame := b.Frame()
		frames[frame] = true
	}

	assert.True(t, len(frames) > 1, "Should see multiple frames during animation")
}

func Test_Blink_IsActive(t *testing.T) {
	b := NewBlink()

	assert.False(t, b.IsActive())

	b.Start()
	assert.True(t, b.IsActive())

	b.Stop()
	assert.False(t, b.IsActive())
}
