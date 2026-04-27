package components

import "time"

// UI timing constants
const (
	UITickInterval   = 100 * time.Millisecond
	UITicksPerSecond = int(time.Second / UITickInterval) // 1000/100 = 10
	TipRotationTicks = 10 * UITicksPerSecond

	StatsCallTimeout = 100 * time.Millisecond
)

// Panel layout constants
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
	IndicatorDot      = "◉"
)

// Table layout constants
const (
	MaxMetricWidth       = 12
	MaxStatusWidth       = 16
	MetricColumnCount    = 4
	StatusWidthDivisor   = 5
	MetricWidthDivisor   = 10
	IndicatorColumnWidth = 2
	NameTrailingGap      = 1
	RowWidthPadding      = 8
	RowHorizontalPadding = 4
	ErrorPadding         = "  "
)

// Timeline layout constants
const (
	DefaultTimelineSlots    = 16
	MinServiceNameTextWidth = 24
	MinServiceNameWidth     = IndicatorColumnWidth + MinServiceNameTextWidth
	MinTimelineWidth        = 8
	TimelineGap             = 1
	TimelineBlock           = "▮"
)

// Service name bucket constants picked by the longest service name length
const (
	NameWidthShort  = 16
	NameWidthMedium = 32
	NameWidthLong   = 48
)

// Unit conversion constants
const (
	MBToGB = 1024
)

// Log stream constants
const (
	DefaultMaxServiceLen = 12
)
