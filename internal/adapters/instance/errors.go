package instance

import "errors"

// Sentinels local to the instance adapter
var (
	ErrFailedToResolveProject = errors.New("failed to resolve project directory")
	ErrFailedToLockProject    = errors.New("failed to lock the project")
)
