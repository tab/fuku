package components

import "time"

// UI timing constants
const (
	UITickInterval     = 100 * time.Millisecond
	UITicksPerSecond   = int(time.Second / UITickInterval) // 1000/100 = 10
	UITipRotationTicks = 10 * UITicksPerSecond
	UIStatsCallTimeout = 100 * time.Millisecond
)

// Panel layout constants
const (
	PanelHeightPadding = 4
	PanelInnerPadding  = 2
	PanelBorderHeight  = 2
	PanelMinHeight     = 10
)

// Border constants
const (
	BorderEdgeWidth   = 3
	BorderSpacerWidth = 2

	BorderTopLeft     = "╭"
	BorderTopRight    = "╮"
	BorderBottomLeft  = "╰"
	BorderBottomRight = "╯"
	BorderHorizontal  = "─"
	BorderVertical    = "│"
)

// Aside layout constants
const (
	AsideMinWidth     = 30
	AsideMinMainWidth = 42
)

// Indicator constants
const (
	IndicatorSelected    = "›"
	IndicatorEmpty       = " "
	IndicatorDot         = "◉"
	IndicatorColumnWidth = 2
)

// Row layout constants
const (
	RowWidthPadding      = 8
	RowHorizontalPadding = 4
	RowErrorPadding      = "  "
)

// Status column constants
const (
	StatusMaxWidth     = 16
	StatusCompactWidth = 10
	StatusWidthDivisor = 5
)

// Metric column constants
const (
	MetricMaxWidth        = 12
	MetricFullColumnCount = 4
	MetricWidthDivisor    = 10
)

// Service name column constants (buckets picked by the longest service name length)
const (
	ServiceNameWidthShort   = 16
	ServiceNameWidthMedium  = 32
	ServiceNameWidthLong    = 48
	ServiceNameMinTextWidth = 24
	ServiceNameMinWidth     = IndicatorColumnWidth + ServiceNameMinTextWidth
	ServiceNameTrailingGap  = 1
)

// Timeline layout constants
const (
	TimelineDefaultSlots = 16
	TimelineMinWidth     = 8
	TimelineGap          = 1
	TimelineBlock        = "▮"
)

// Log stream constants
const (
	LogStreamMaxServiceNameLen = 12
)

// Unit conversion constants
const (
	MBToGB = 1024
)
