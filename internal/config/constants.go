package config

import "time"

// app constants
const (
	DefaultLogLevel  = "info"
	DefaultLogFormat = "console"

	Version = "0.3.0"
)

// profile constants
const (
	DefaultProfile = "default"
)

// tier constants
const (
	Foundation = "foundation"
	Platform   = "platform"
	Edge       = "edge"
	Default    = "default"

	MaxWorkers = 3
)

// readiness constants
const (
	TypeHTTP = "http"
	TypeLog  = "log"

	DefaultTimeout  = 30 * time.Second
	DefaultInterval = 500 * time.Millisecond

	RetryAttempt = 3
	RetryBackoff = 2 * time.Second
)

// service constants
const (
	ShutdownTimeout = 5 * time.Second
)
