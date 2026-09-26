package terminal

import (
	"math/rand/v2"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/harmonica"
)

// Blink animation constants
const (
	frameEmpty = "◯"
	frameFull  = IndicatorDot

	// blinkFPS is the spring frame rate, one frame per UI tick
	blinkFPS = UITicksPerSecond

	// blinkAngularFrequency and blinkDampingRatio tune the spring
	blinkAngularFrequency = 8.0
	blinkDampingRatio     = 0.7

	blinkSettleTicks   = 2
	blinkBeat1Ticks    = 1
	blinkMicroGapTicks = 1
	blinkBeat2Ticks    = 1
	blinkRecoveryTicks = 3

	// blinkCycleTicks is the length of one blink cycle
	blinkCycleTicks = blinkSettleTicks + blinkBeat1Ticks + blinkMicroGapTicks + blinkBeat2Ticks + blinkRecoveryTicks

	// blinkFrameThreshold is the position above which the full frame shows
	blinkFrameThreshold = 0.3

	// blinkPositionFull and blinkPositionEmpty are the spring targets
	blinkPositionFull  = 1.0
	blinkPositionEmpty = 0.0
)

// Blink creates smooth ping-like animations using spring physics
type Blink struct {
	spring    harmonica.Spring
	position  float64
	velocity  float64
	target    float64
	active    bool
	tickCount int
	state     state
}

// state represents the current phase of the blink animation
type state int

// Animation state phases
const (
	stateSettle state = iota
	stateBeat1
	stateMicroGap
	stateBeat2
	stateRecovery
)

// NewBlink creates a new blink animator with smooth spring physics and random initial offset
func NewBlink() *Blink {
	//nolint:gosec // weak random is fine for UI animation timing
	randomTickOffset := rand.IntN(blinkCycleTicks)

	return &Blink{
		spring:    harmonica.NewSpring(harmonica.FPS(blinkFPS), blinkAngularFrequency, blinkDampingRatio),
		position:  blinkPositionEmpty,
		velocity:  blinkPositionEmpty,
		target:    blinkPositionEmpty,
		active:    false,
		tickCount: randomTickOffset,
		state:     stateSettle,
	}
}

// Start begins the blinking animation
func (b *Blink) Start() {
	b.active = true
}

// Stop ends the blinking animation and resets to empty state
func (b *Blink) Stop() {
	b.active = false
	b.target = blinkPositionEmpty
	b.position = blinkPositionEmpty
	b.velocity = blinkPositionEmpty
	b.tickCount = 0
	b.state = stateSettle
}

// Update advances the animation (called on each UI tick)
func (b *Blink) Update() {
	if !b.active {
		return
	}

	b.tickCount++

	switch b.state {
	case stateSettle:
		if b.tickCount >= blinkSettleTicks {
			b.state = stateBeat1
			b.target = blinkPositionFull
			b.tickCount = 0
		}

	case stateBeat1:
		if b.tickCount >= blinkBeat1Ticks {
			b.state = stateMicroGap
			b.target = blinkPositionEmpty
			b.tickCount = 0
		}

	case stateMicroGap:
		if b.tickCount >= blinkMicroGapTicks {
			b.state = stateBeat2
			b.target = blinkPositionFull
			b.tickCount = 0
		}

	case stateBeat2:
		if b.tickCount >= blinkBeat2Ticks {
			b.state = stateRecovery
			b.target = blinkPositionEmpty
			b.tickCount = 0
		}

	case stateRecovery:
		if b.tickCount >= blinkRecoveryTicks {
			b.state = stateSettle
			b.tickCount = 0
		}
	}

	b.position, b.velocity = b.spring.Update(b.position, b.velocity, b.target)
}

// Frame returns the current frame based on the spring position
func (b *Blink) Frame() string {
	if !b.active {
		return frameEmpty
	}

	if b.position < blinkFrameThreshold {
		return frameEmpty
	}

	return frameFull
}

// Render returns the styled frame
func (b *Blink) Render(style lipgloss.Style) string {
	return style.Render(b.Frame())
}

// IsActive returns whether the animation is currently running
func (b *Blink) IsActive() bool {
	return b.active
}
