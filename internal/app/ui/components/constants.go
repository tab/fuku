package components

import "time"

// UI timing constants
const (
	// UITickInterval is the base tick rate for the services UI
	// This must match the tick interval in services/update.go
	UITickInterval = 100 * time.Millisecond

	// Derived FPS for animations (ticks per second)
	// Calculated as: 1000ms / UITickInterval
	UITicksPerSecond = int(time.Second / UITickInterval) // 1000/100 = 10

	// StatsPollingInterval is how often the background worker polls process stats
	StatsPollingInterval = 1 * time.Second

	// StatsCallTimeout is the per-call timeout for GetStats to prevent hangs
	StatsCallTimeout = 500 * time.Millisecond

	// StatsBatchTimeout is the overall timeout for collecting all service stats
	StatsBatchTimeout = 900 * time.Millisecond

	// StatsMaxConcurrency limits concurrent GetStats calls for high-scale scenarios
	StatsMaxConcurrency = 50
)

// Generic layout constants
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
	FixedColumnsWidth    = 45
	ServiceNameMinWidth  = 20
	ServiceRowPadding    = 8
	ErrorMessageMinWidth = 20
	ViewportWidthPadding = 0
	RowWidthPadding      = 8
)

// Logs view constants
const (
	LogBufferSize          = 1000
	LogServiceNameMaxWidth = 15
	LogMessageMinWidth     = 20
	LogPrefixSpacing       = 3
	DefaultViewportWidth   = 80
)
