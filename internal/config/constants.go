package config

import "time"

// Application metadata
const (
	AppName = "fuku"
	Version = "0.19.1"

	ConfigFile    = "fuku.yaml"
	ConfigFileAlt = "fuku.yml"

	OverrideConfigFile    = "fuku.override.yaml"
	OverrideConfigFileAlt = "fuku.override.yml"
)

// Default values
const (
	Default = "default"
)

// Environment names
const (
	EnvProduction  = "production"
	EnvDevelopment = "development"
	EnvTest        = "test"
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
	DefaultTimeout       = 30 * time.Second
	DefaultInterval      = 500 * time.Millisecond
	ShutdownTimeout      = 5 * time.Second
	PreFlightTimeout     = 100 * time.Millisecond
	PreFlightKillTimeout = 2 * time.Second
)

// Retry settings
const (
	RetryAttempts = 3
	RetryBackoff  = 500 * time.Millisecond
)

// Socket configuration
const (
	SocketDir             = "/tmp"
	SocketPrefix          = "fuku-"
	SocketSuffix          = ".sock"
	SocketDialTimeout     = 100 * time.Millisecond
	SocketWriteTimeout    = 5 * time.Second
	SocketLogsBufferSize  = 1000
	SocketLogsHistorySize = 5000
)

// Watch settings
const (
	WatchDebounce = 500 * time.Millisecond
)

// API server settings
const (
	APIPortRetries       = 10
	APIReadHeaderTimeout = 5 * time.Second
	StoreSampleInterval  = 2 * time.Second
	StoreSampleTimeout   = 200 * time.Millisecond
)

// Loopback hostnames (not available as stdlib constants)
const (
	LoopbackHostname     = "localhost"
	LoopbackIPv6Hostname = "ip6-localhost"
)
