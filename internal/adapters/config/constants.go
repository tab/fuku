package config

import (
	"time"

	"fuku/internal/platform/logging"
)

// Config file names
const (
	ConfigFile    = "fuku.yaml"
	ConfigFileAlt = "fuku.yml"

	OverrideConfigFile    = "fuku.override.yaml"
	OverrideConfigFileAlt = "fuku.override.yml"
)

// Default service command
const (
	DefaultServiceCommand = "make run"
)

// Environment names
const (
	EnvProduction = "production"
)

// Logging defaults
const (
	DefaultLogLevel  = logging.LevelInfo
	DefaultLogFormat = logging.FormatConsole
)

// Concurrency settings
const (
	MaxWorkers = 5
)

// Readiness defaults
const (
	DefaultTimeout  = 30 * time.Second
	DefaultInterval = 500 * time.Millisecond
)

// Watch defaults
const (
	DefaultDebounce = 500 * time.Millisecond
)

// Env file defaults
const (
	DefaultEnvFile                 = ".env"
	DefaultEnvFileLocal            = ".env.local"
	DefaultEnvFileDevelopment      = ".env.development"
	DefaultEnvFileDevelopmentLocal = ".env.development.local"
)

// Retry settings
const (
	RetryAttempts = 3
	RetryBackoff  = 500 * time.Millisecond
)

// Log stream defaults
const (
	SocketLogsBufferSize  = 1000
	SocketLogsHistorySize = 5000
)

// Loopback hostnames (not available as stdlib constants)
const (
	LoopbackHostname     = "localhost"
	LoopbackIPv6Hostname = "ip6-localhost"
)

// YAML keys the topology parser reads from the raw document
const (
	keyDefaults = "defaults"
	keyServices = "services"
	keyTier     = "tier"
)
