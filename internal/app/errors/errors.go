package errors

import (
	"errors"
)

var (
	ErrFailedToReadConfig  = errors.New("failed to read config file")
	ErrFailedToParseConfig = errors.New("failed to parse config file")
	ErrUnknownCommand      = errors.New("unknown command")
)
