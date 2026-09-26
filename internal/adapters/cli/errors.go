package cli

import "errors"

// Sentinels local to the cli adapter
var (
	ErrConfigFlagNotSupported = errors.New("--config flag is not supported for this command")
	ErrInvalidTail            = errors.New("--tail must be greater than zero")
)
