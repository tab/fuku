package components

import (
	"math/rand/v2"

	"github.com/charmbracelet/harmonica"
	"github.com/charmbracelet/lipgloss"
)

const (
	empty = "◯"
	full  = "◉"

	// Animation timing constants
	blinkFPS              = 3   // Updates per second (must match tick interval)
	blinkAngularFrequency = 4.5 // Spring stiffness
	blinkDampingRatio     = 0.6 // Spring damping
	blinkFrameThreshold   = 0.5 // Position threshold for frame switching
	blinkWaitingTicks     = 4   // Ticks to wait before pulsing (~1.2s at 300ms ticks)
	blinkPulsingTicks     = 3   // Ticks to pulse before waiting (~900ms at 300ms ticks)
	blinkPositionFull     = 1.0 // Full position value
	blinkPositionEmpty    = 0.0 // Empty position value
	blinkTargetThreshold  = 0.7 // Spring position threshold for state transitions
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

type state int

const (
	waiting state = iota // Waiting / empty state
	pulsing              // Pulsing / full state
)

// NewBlink creates a new blink animator with smooth spring physics and random initial state
func NewBlink() *Blink {
	// Random initial state to desynchronize animations
	//nolint:gosec // weak random is fine for UI animation timing
	randomState := state(rand.IntN(2))

	var initialPosition, initialTarget float64
	if randomState == pulsing {
		initialPosition = blinkPositionFull
		initialTarget = blinkPositionFull
	}

	return &Blink{
		spring:    harmonica.NewSpring(harmonica.FPS(blinkFPS), blinkAngularFrequency, blinkDampingRatio),
		position:  initialPosition,
		velocity:  blinkPositionEmpty,
		target:    initialTarget,
		active:    false,
		tickCount: 0,
		state:     randomState,
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
	b.state = waiting
}

// Update advances the animation - designed for ~3 FPS (300ms ticks)
func (b *Blink) Update() {
	if !b.active {
		return
	}

	b.tickCount++

	// Two-state blink: waiting (empty) <-> pulsing (full)
	switch b.state {
	case waiting:
		b.target = blinkPositionEmpty
		if b.tickCount >= blinkWaitingTicks {
			b.state = pulsing
			b.target = blinkPositionFull
			b.tickCount = 0
		}

	case pulsing:
		b.target = blinkPositionFull
		if b.tickCount >= blinkPulsingTicks {
			b.state = waiting
			b.target = blinkPositionEmpty
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
