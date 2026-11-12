package mcp

import "errors"

// Sentinels local to the mcp adapter
var (
	errAPIUnauthorized = errors.New("the API rejected the token from the loaded config; it is missing or wrong. An explicit config skips fuku.override.yaml")
	errUnknownService  = errors.New("unknown service")
	errAPIOff          = errors.New("the fuku REST API is off in this project; restart the whole profile with stop and run")
)
