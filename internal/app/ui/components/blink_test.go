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
	assert.Equal(t, empty, b.Frame())
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

	assert.Equal(t, empty, b.Frame())
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

func Test_Blink_Repeats(t *testing.T) {
	b := NewBlink()
	b.Start()

	totalTicks := blinkSettleTicks + blinkBeat1Ticks + blinkMicroGapTicks + blinkBeat2Ticks + blinkRecoveryTicks
	cycleCount := 3

	for i := 0; i < totalTicks*cycleCount; i++ {
		b.Update()
	}

	assert.True(t, b.IsActive(), "Animation should still be active after multiple cycles")
}

func Test_Blink_CyclesMultipleTimes(t *testing.T) {
	b := NewBlink()
	b.tickCount = 0
	b.Start()

	stateChanges := 0
	lastState := b.state
	fullFrames := 0
	stateHistory := make(map[state]int)

	for i := 0; i < 100; i++ {
		b.Update()

		stateHistory[b.state]++

		if b.state != lastState {
			stateChanges++
			lastState = b.state
		}

		if b.Frame() == full {
			fullFrames++
		}
	}

	t.Logf("State changes: %d", stateChanges)
	t.Logf("Full frames: %d", fullFrames)
	t.Logf("State history: %+v", stateHistory)
	t.Logf("Final state: %v, tickCount: %d", b.state, b.tickCount)

	assert.Greater(t, stateChanges, 10, "Should see many state changes over 100 ticks")
	assert.Greater(t, fullFrames, 10, "Should see full frame many times")
}
