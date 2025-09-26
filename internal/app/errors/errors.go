package errors

import (
	"errors"
)

var (
	ErrFailedToReadConfig   = errors.New("failed to read config file")
	ErrFailedToParseConfig  = errors.New("failed to parse config file")
	ErrServiceNotFound      = errors.New("service not found in configuration")
	ErrScopeNotFound        = errors.New("scope not found in configuration")
	ErrCircularDependency   = errors.New("circular dependency detected")
	ErrFailedToStartService = errors.New("failed to start service")
)
