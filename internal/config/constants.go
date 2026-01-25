package config

import "time"

// Application metadata
const (
	AppName = "fuku"
	Version = "0.9.1"
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
	TypeLog  = "log"
)

// Timing constants
const (
	DefaultTimeout  = 30 * time.Second
	DefaultInterval = 500 * time.Millisecond
	ShutdownTimeout = 5 * time.Second
)

// Retry settings
const (
	RetryAttempts = 3
	RetryBackoff  = 500 * time.Millisecond
)

// Socket configuration
const (
	SocketDir         = "/tmp"
	SocketPrefix      = "fuku-"
	SocketSuffix      = ".sock"
	SocketDialTimeout = 100 * time.Millisecond
	LogsBufferSize    = 100
)
