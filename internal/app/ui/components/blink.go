package components

import (
	"math/rand/v2"

	"github.com/charmbracelet/harmonica"
	"github.com/charmbracelet/lipgloss"
)

// Blink animation constants
const (
	empty = "◯"
	full  = "◉"

	// Animation timing derived from UI tick rate
	blinkFPS = UITicksPerSecond

	// Spring physics parameters
	blinkAngularFrequency = 8.0
	blinkDampingRatio     = 0.7

	blinkSettleTicks   = 2 // Settle phase: 200ms
	blinkBeat1Ticks    = 1 // First beat (lub): 100ms
	blinkMicroGapTicks = 1 // Micro-gap between beats: 100ms
	blinkBeat2Ticks    = 1 // Second beat (DUB): 100ms
	blinkRecoveryTicks = 3 // Recovery phase: 300ms

	// Full cycle duration
	blinkCycleTicks = blinkSettleTicks + blinkBeat1Ticks + blinkMicroGapTicks + blinkBeat2Ticks + blinkRecoveryTicks

	// Visual thresholds
	blinkFrameThreshold = 0.3 // Position threshold for frame switching (lower = triggers faster)

	// Position constants
	blinkPositionFull  = 1.0 // Full position value
	blinkPositionEmpty = 0.0 // Empty position value
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
	settle   state = iota // Settle phase (empty)
	beat1                 // First beat (full)
	microGap              // Micro-gap (empty)
	beat2                 // Second beat (full)
	recovery              // Recovery phase (empty, transitions to settle)
)

// NewBlink creates a new blink animator with smooth spring physics and random initial offset
func NewBlink() *Blink {
	// Random initial tick offset to desynchronize animations
	//nolint:gosec // weak random is fine for UI animation timing
	randomTickOffset := rand.IntN(blinkCycleTicks)

	return &Blink{
		spring:    harmonica.NewSpring(harmonica.FPS(blinkFPS), blinkAngularFrequency, blinkDampingRatio),
		position:  blinkPositionEmpty,
		velocity:  blinkPositionEmpty,
		target:    blinkPositionEmpty,
		active:    false,
		tickCount: randomTickOffset,
		state:     settle,
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
	b.state = settle
}

// Update advances the animation (called on each UI tick)
func (b *Blink) Update() {
	if !b.active {
		return
	}

	b.tickCount++

	// settle(200ms) → beat1(100ms) → micro-gap(100ms) → beat2(100ms) → recovery(300ms)
	switch b.state {
	case settle:
		b.target = blinkPositionEmpty
		if b.tickCount >= blinkSettleTicks {
			b.state = beat1
			b.target = blinkPositionFull
			b.tickCount = 0
		}

	case beat1:
		b.target = blinkPositionFull
		if b.tickCount >= blinkBeat1Ticks {
			b.state = microGap
			b.target = blinkPositionEmpty
			b.tickCount = 0
		}

	case microGap:
		b.target = blinkPositionEmpty
		if b.tickCount >= blinkMicroGapTicks {
			b.state = beat2
			b.target = blinkPositionFull
			b.tickCount = 0
		}

	case beat2:
		b.target = blinkPositionFull
		if b.tickCount >= blinkBeat2Ticks {
			b.state = recovery
			b.target = blinkPositionEmpty
			b.tickCount = 0
		}

	case recovery:
		b.target = blinkPositionEmpty
		if b.tickCount >= blinkRecoveryTicks {
			b.state = settle
			b.tickCount = 0
		}
	}

	b.position, b.velocity = b.spring.Update(b.position, b.velocity, b.target)
}

// Frame returns the current frame based on the spring position
func (b *Blink) Frame() string {
	if !b.active {
		return empty
	}

	if b.position < blinkFrameThreshold {
		return empty
	}

	return full
}

// Render returns the styled frame
func (b *Blink) Render(style lipgloss.Style) string {
	return style.Render(b.Frame())
}

// IsActive returns whether the animation is currently running
func (b *Blink) IsActive() bool {
	return b.active
}
