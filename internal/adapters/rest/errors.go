package rest

import "errors"

// Sentinels local to the rest adapter
var (
	ErrAPIUnauthorized    = errors.New("unauthorized")
	ErrAPIForbidden       = errors.New("forbidden")
	ErrAPIServiceNotFound = errors.New("service not found")
	ErrAPINotStartable    = errors.New("service cannot be started")
	ErrAPINotRunning      = errors.New("service is not running")
	ErrAPINotRestartable  = errors.New("service cannot be restarted")
	ErrAPINotAccepting    = errors.New("instance is not accepting actions")
	ErrAPIOverloaded      = errors.New("instance is overloaded")
)
