package terminal

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_Blink_Update(t *testing.T) {
	tests := []struct {
		name          string
		tickCount     int
		ticks         int
		expectedState state
		expectedFrame string
	}{
		{
			name:          "settles before the first beat",
			tickCount:     0,
			ticks:         1,
			expectedState: stateSettle,
			expectedFrame: frameEmpty,
		},
		{
			name:          "first beat pulls the spring toward full",
			tickCount:     0,
			ticks:         2,
			expectedState: stateBeat1,
			expectedFrame: frameEmpty,
		},
		{
			name:          "micro gap shows the full frame the spring lags into",
			tickCount:     0,
			ticks:         3,
			expectedState: stateMicroGap,
			expectedFrame: frameFull,
		},
		{
			name:          "second beat keeps the full frame",
			tickCount:     0,
			ticks:         4,
			expectedState: stateBeat2,
			expectedFrame: frameFull,
		},
		{
			name:          "recovery fades the frame",
			tickCount:     0,
			ticks:         7,
			expectedState: stateRecovery,
			expectedFrame: frameEmpty,
		},
		{
			name:          "a full cycle returns to settle",
			tickCount:     0,
			ticks:         blinkCycleTicks + 1,
			expectedState: stateSettle,
			expectedFrame: frameEmpty,
		},
		{
			name:          "the next cycle beats again",
			tickCount:     0,
			ticks:         blinkCycleTicks + 3,
			expectedState: stateMicroGap,
			expectedFrame: frameFull,
		},
		{
			name:          "an offset shortens the first settle",
			tickCount:     1,
			ticks:         1,
			expectedState: stateBeat1,
			expectedFrame: frameEmpty,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			blink := NewBlink()
			blink.tickCount = tt.tickCount
			blink.Start()

			for range tt.ticks {
				blink.Update()
			}

			assert.Equal(t, tt.expectedState, blink.state)
			assert.Equal(t, tt.expectedFrame, blink.Frame())
		})
	}
}

func Test_Blink_Stop(t *testing.T) {
	blink := NewBlink()
	blink.tickCount = 0
	blink.Start()

	for range 3 {
		blink.Update()
	}

	blink.Stop()
	blink.Update()

	assert.False(t, blink.IsActive())
	assert.Equal(t, stateSettle, blink.state)
	assert.Equal(t, 0, blink.tickCount)
	assert.Zero(t, blink.position)
	assert.Equal(t, frameEmpty, blink.Frame())
}

func Test_Blink_Render(t *testing.T) {
	blink := NewBlink()
	blink.tickCount = 0
	blink.Start()

	for range 3 {
		blink.Update()
	}

	result := blink.Render(BoldStyle)

	assert.Equal(t, BoldStyle.Render(frameFull), result)
}
