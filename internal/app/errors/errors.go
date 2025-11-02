package errors

import (
	"errors"
)

var (
	ErrFailedToReadConfig  = errors.New("failed to read config file")
	ErrFailedToParseConfig = errors.New("failed to parse config file")
	ErrInvalidConfig       = errors.New("invalid configuration")

	ErrProfileNotFound          = errors.New("profile not found")
	ErrUnsupportedProfileFormat = errors.New("unsupported profile format")

	ErrServiceNotFound          = errors.New("service not found")
	ErrServiceDirectoryNotExist = errors.New("service directory does not exist")

	ErrInvalidTier              = errors.New("invalid tier value")
	ErrInvalidReadinessType     = errors.New("invalid readiness type")
	ErrReadinessTypeRequired    = errors.New("readiness type is required")
	ErrReadinessURLRequired     = errors.New("readiness type 'http' requires url field")
	ErrReadinessPatternRequired = errors.New("readiness type 'log' requires pattern field")
	ErrReadinessTimeout         = errors.New("readiness check timed out")
	ErrInvalidRegexPattern      = errors.New("invalid regex pattern")

	ErrFailedToGetWorkingDir = errors.New("failed to get working directory")
	ErrFailedToCreatePipe    = errors.New("failed to create pipe")
	ErrFailedToStartCommand  = errors.New("failed to start command")
	ErrFailedToCreateRequest = errors.New("failed to create request")
)

var (
	As  = errors.As
	New = errors.New
)
