package config

import "errors"

// Sentinels local to the config adapter
var (
	ErrFailedToParseConfig       = errors.New("failed to parse config file")
	ErrInvalidLogLevel           = errors.New("invalid logging level (must be 'debug', 'info', 'warn' or 'error')")
	ErrInvalidConcurrencyWorkers = errors.New("concurrency workers must be greater than 0")
	ErrInvalidRetryAttempts      = errors.New("retry attempts must be greater than 0")
	ErrInvalidRetryBackoff       = errors.New("retry backoff must not be negative")
	ErrInvalidLogsBuffer         = errors.New("logs buffer must be greater than 0")
	ErrInvalidLogsHistory        = errors.New("logs history must be greater than 0")
	ErrUnsupportedProfileFormat  = errors.New("unsupported profile format")
	ErrProfileReferenceUndefined = errors.New("profile references undefined service")
	ErrInvalidReadinessType      = errors.New("invalid readiness type")
	ErrReadinessTypeRequired     = errors.New("readiness type is required")
	ErrReadinessURLRequired      = errors.New("readiness type 'http' requires url field")
	ErrReadinessAddressRequired  = errors.New("readiness type 'tcp' requires address field")
	ErrReadinessPatternRequired  = errors.New("readiness type 'log' requires pattern field")
	ErrEmptyService              = errors.New("service body must not be empty")
	ErrInvalidCommand            = errors.New("command must not be whitespace-only when provided")
	ErrWatchIncludeRequired      = errors.New("watch configuration requires include field")
	ErrInvalidLogsOutput         = errors.New("invalid service logs output value (must be 'stdout' or 'stderr')")
	ErrAPIInvalidListen          = errors.New("server.listen must be a valid host:port address")
	ErrAPINotLoopback            = errors.New("server.listen must bind to a loopback address")
	ErrAPITokenRequired          = errors.New("server.auth.token is required when server.listen is set")
)
