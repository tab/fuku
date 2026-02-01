package components

import "time"

// UI timing constants
const (
	UITickInterval   = 100 * time.Millisecond
	UITicksPerSecond = int(time.Second / UITickInterval) // 1000/100 = 10
	TipRotationTicks = 10 * UITicksPerSecond

	StatsPollingInterval = 1 * time.Second
	StatsCallTimeout     = 500 * time.Millisecond
	StatsBatchTimeout    = 900 * time.Millisecond
	StatsMaxConcurrency  = 50
)

// Layout constants
const (
	PanelHeightPadding = 4
	PanelInnerPadding  = 2
	PanelBorderHeight  = 2
	MinPanelHeight     = 10
	BorderEdgeWidth    = 3
	SpacerWidth        = 2
)

// Border characters
const (
	BorderTopLeft     = "╭"
	BorderTopRight    = "╮"
	BorderBottomLeft  = "╰"
	BorderBottomRight = "╯"
	BorderHorizontal  = "─"
	BorderVertical    = "│"
)

// Indicator characters
const (
	IndicatorSelected = "›"
	IndicatorEmpty    = " "
	IndicatorWatch    = "◉"
)

// Services view constants
const (
	FixedColumnsWidth   = 50
	ServiceNameMinWidth = 20
	RowWidthPadding     = 8
	ErrorPadding        = "  "
)

// Column widths for services table
const (
	ColWidthIndicator = 1
	ColWidthStatus    = 10
	ColWidthCPU       = 6
	ColWidthMem       = 6
	ColWidthPID       = 8
	ColWidthUptime    = 8
)

// Unit conversion constants
const (
	MBToGB = 1024
)

// Log stream constants
const (
	DefaultMaxServiceLen = 12
)
