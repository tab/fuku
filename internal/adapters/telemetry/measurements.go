package telemetry

// Gauge metrics measure current values
const (
	MetricServiceCount    = "service_count"
	MetricTierCount       = "tier_count"
	MetricPreflightKilled = "preflight_killed"
	MetricAPIEnabled      = "api_enabled"
)

// Counter metrics track cumulative occurrences
const (
	MetricAppRun          = "app_run"
	MetricServiceFailed   = "service_failed"
	MetricServiceRestart  = "service_restart"
	MetricUnexpectedExit  = "unexpected_exit"
	MetricWatchRestart    = "watch_restart"
	MetricAPIRequests     = "api_requests"
	MetricAPIAuthFailures = "api_auth_failures"
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

// Distribution metrics for API request latency
const (
	MetricAPIRequestDuration = "api_request_duration"
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
	TagGoVersion    = "go_version"
	TagOS           = "os"
	TagProfile      = "profile"
	TagServiceCount = "service_count"
	TagType         = "type"
	TagUI           = "ui"
	TagMethod       = "method"
	TagPath         = "path"
	TagStatus       = "status"
)
