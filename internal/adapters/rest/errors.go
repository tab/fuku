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
	ErrAPIInvalidBody     = errors.New("body must be a JSON object with an optional wait")
	ErrAPIInvalidWait     = errors.New("wait must be whole seconds from 1 to 60")
	ErrClientNotLoopback  = errors.New("the fuku REST API address is not a loopback address")
	ErrClientFailed       = errors.New("the fuku REST API failed")
	ErrClientNotFuku      = errors.New("the address does not serve the fuku REST API")
)
