package config

import "time"

// Application metadata
const (
	AppName = "fuku"
	Version = "0.12.0"

	ConfigFile = "fuku.yaml"
)

// Default values
const (
	Default = "default"
)

// Logging defaults
const (
	LogLevel  = "info"
	LogFormat = "console"
)

// Concurrency settings
const (
	MaxWorkers = 5
)

// Readiness check types
const (
	TypeHTTP = "http"
	TypeTCP  = "tcp"
	TypeLog  = "log"
)

// Timing constants
const (
	DefaultTimeout   = 30 * time.Second
	DefaultInterval  = 500 * time.Millisecond
	ShutdownTimeout  = 5 * time.Second
	PreFlightTimeout = 100 * time.Millisecond
)

// Retry settings
const (
	RetryAttempts = 3
	RetryBackoff  = 500 * time.Millisecond
)

// Socket configuration
const (
	SocketDir            = "/tmp"
	SocketPrefix         = "fuku-"
	SocketSuffix         = ".sock"
	SocketDialTimeout    = 100 * time.Millisecond
	SocketWriteTimeout   = 5 * time.Second
	SocketLogsBufferSize = 1000
)

// Watch settings
const (
	WatchDebounce = 500 * time.Millisecond
)
