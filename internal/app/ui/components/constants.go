package components

import "time"

// UI timing constants
const (
	UITickInterval   = 100 * time.Millisecond
	UITicksPerSecond = int(time.Second / UITickInterval) // 1000/100 = 10

	StatsPollingInterval = 1 * time.Second
	StatsCallTimeout     = 500 * time.Millisecond
	StatsBatchTimeout    = 900 * time.Millisecond
	StatsMaxConcurrency  = 50
)

// Layout constants
const (
	PanelHeightPadding = 8
	PanelBorderPadding = 2
	MinPanelHeight     = 10
)

// Header layout constants
const (
	HeaderSeparatorMinWidth = 4
	HeaderFixedChars        = 10
)

// Footer layout constants
const (
	FooterSeparatorMinWidth = 4
	FooterFixedChars        = 5
)

// Services view constants
const (
	FixedColumnsWidth    = 53
	ServiceNameMinWidth  = 20
	ViewportWidthPadding = 2
	RowWidthPadding      = 8
	Current              = "› "
	Empty                = "[ ]"
	Selected             = "[✓]"
)

// Logs view constants
const (
	LogBufferSize          = 1000
	LogServiceNameMaxWidth = 15
	LogMessageMinWidth     = 20
	DefaultViewportWidth   = 80
)

const (
	MBToGB = 1024
)
