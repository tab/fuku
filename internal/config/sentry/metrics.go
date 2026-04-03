package sentry

import (
	gosentry "github.com/getsentry/sentry-go"
	"github.com/getsentry/sentry-go/attribute"
)

// Re-exported Meter API types from sentry-go
type (
	Meter       = gosentry.Meter
	MeterOption = gosentry.MeterOption
)

// Re-exported Meter API functions from sentry-go
var (
	NewMeter       = gosentry.NewMeter
	WithUnit       = gosentry.WithUnit
	WithAttributes = gosentry.WithAttributes
)

// Re-exported unit constants from sentry-go
const (
	UnitMillisecond = gosentry.UnitMillisecond
	UnitSecond      = gosentry.UnitSecond
	UnitPercent     = gosentry.UnitPercent
	UnitMegabyte    = gosentry.UnitMegabyte
)

// Re-exported attribute builders from sentry-go/attribute
var (
	StringAttr  = attribute.String
	IntAttr     = attribute.Int
	BoolAttr    = attribute.Bool
	Float64Attr = attribute.Float64
)

// Gauge metrics measure current values
const (
	MetricServiceCount    = "service_count"
	MetricTierCount       = "tier_count"
	MetricPreflightKilled = "preflight_killed"
)

// Counter metrics track cumulative occurrences
const (
	MetricAppRun         = "app_run"
	MetricServiceFailed  = "service_failed"
	MetricServiceRestart = "service_restart"
	MetricUnexpectedExit = "unexpected_exit"
	MetricWatchRestart   = "watch_restart"
	MetricAPIEnabled     = "api_enabled"
)

// Distribution metrics track timing data in milliseconds
const (
	MetricDiscoveryDuration      = "discovery_duration"
	MetricPreflightDuration      = "preflight_duration"
	MetricReadinessDuration      = "readiness_duration"
	MetricServiceStartupDuration = "service_startup_duration"
	MetricShutdownDuration       = "shutdown_duration"
	MetricStartupDuration        = "startup_duration"
	MetricTierStartupDuration    = "tier_startup_duration"
)

// Distribution metrics for fuku process resource usage
const (
	MetricFukuCPU    = "fuku_cpu"
	MetricFukuMemory = "fuku_memory"
)

// Tag and attribute keys for Sentry scope tags and metric annotations
const (
	TagArch         = "arch"
	TagCommand      = "command"
	TagEnv          = "env"
	TagGoVersion    = "go_version"
	TagOS           = "os"
	TagProfile      = "profile"
	TagServiceCount = "service_count"
	TagType         = "type"
	TagUI           = "ui"
)
