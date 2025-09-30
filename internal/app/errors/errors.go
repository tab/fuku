package errors

import (
	"errors"
)

var (
	ErrFailedToReadConfig  = errors.New("failed to read config file")
	ErrFailedToParseConfig = errors.New("failed to parse config file")

	ErrInvalidRegexPattern = errors.New("invalid regex pattern")

	ErrReadinessCheckFailed = errors.New("readiness check failed")
)
