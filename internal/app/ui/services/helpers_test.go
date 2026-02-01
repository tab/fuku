package services

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"fuku/internal/app/errors"
)

func Test_renderError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: "",
		},
		{
			name:     "port already in use",
			err:      errors.ErrPortAlreadyInUse,
			expected: "port already in use",
		},
		{
			name:     "max retries exceeded",
			err:      errors.ErrMaxRetriesExceeded,
			expected: "max retries exceeded",
		},
		{
			name:     "process exited",
			err:      errors.ErrProcessExited,
			expected: "process exited",
		},
		{
			name:     "readiness timeout",
			err:      errors.ErrReadinessTimeout,
			expected: "readiness timeout",
		},
		{
			name:     "failed to start command",
			err:      errors.ErrFailedToStartCommand,
			expected: "failed to start",
		},
		{
			name:     "service not found",
			err:      errors.ErrServiceNotFound,
			expected: "service not found",
		},
		{
			name:     "service directory not exist",
			err:      errors.ErrServiceDirectoryNotExist,
			expected: "directory not found",
		},
		{
			name:     "unknown error returns message",
			err:      fmt.Errorf("custom error"),
			expected: "custom error",
		},
		{
			name:     "wrapped max retries",
			err:      fmt.Errorf("failed: %w", errors.ErrMaxRetriesExceeded),
			expected: "max retries exceeded",
		},
		{
			name:     "wrapped process exited",
			err:      fmt.Errorf("service api: %w", errors.ErrProcessExited),
			expected: "process exited",
		},
		{
			name:     "wrapped readiness timeout",
			err:      fmt.Errorf("check failed: %w", errors.ErrReadinessTimeout),
			expected: "readiness timeout",
		},
		{
			name:     "deeply wrapped error",
			err:      fmt.Errorf("outer: %w", fmt.Errorf("inner: %w", errors.ErrServiceNotFound)),
			expected: "service not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := renderError(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}
