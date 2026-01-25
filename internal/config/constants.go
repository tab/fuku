package config

import "time"

// app constants
const (
	AppName = "fuku"

	DefaultLogLevel  = "info"
	DefaultLogFormat = "console"

	Version = "0.9.1"
)

// profile constants
const (
	DefaultProfile = "default"
)

// tier constants
const (
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
	RetryBackoff = 500 * time.Millisecond
)

// service constants
const (
	ShutdownTimeout = 5 * time.Second
)

// socket constants
const (
	SocketDir         = "/tmp"
	SocketPrefix      = "fuku-"
	SocketSuffix      = ".sock"
	SocketDialTimeout = 100 * time.Millisecond
	LogsBufferSize    = 100
)
