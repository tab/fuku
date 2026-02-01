package services

import (
	"fuku/internal/app/errors"
)

// renderError returns a user-friendly error message
func renderError(err error) string {
	if err == nil {
		return ""
	}

	switch {
	case errors.Is(err, errors.ErrPortAlreadyInUse):
		return "port already in use"
	case errors.Is(err, errors.ErrMaxRetriesExceeded):
		return "max retries exceeded"
	case errors.Is(err, errors.ErrProcessExited):
		return "process exited"
	case errors.Is(err, errors.ErrReadinessTimeout):
		return "readiness timeout"
	case errors.Is(err, errors.ErrFailedToStartCommand):
		return "failed to start"
	case errors.Is(err, errors.ErrServiceNotFound):
		return "service not found"
	case errors.Is(err, errors.ErrServiceDirectoryNotExist):
		return "directory not found"
	default:
		return err.Error()
	}
}
