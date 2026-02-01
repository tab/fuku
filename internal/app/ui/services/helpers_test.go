package services

import (
	"fmt"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/stretchr/testify/assert"

	"fuku/internal/app/errors"
)

func isValidUTF8(s string) bool {
	for _, r := range s {
		if r == '\uFFFD' {
			return false
		}
	}

	return true
}

func Test_truncateErrorMessage(t *testing.T) {
	tests := []struct {
		name           string
		errorText      string
		availableWidth int
		expected       string
		checkWidth     bool
	}{
		{
			name:           "short error fits completely",
			errorText:      "connection timeout",
			availableWidth: 50,
			expected:       " (connection timeout)",
			checkWidth:     true,
		},
		{
			name:           "exact fit with wrapper",
			errorText:      "error",
			availableWidth: 9,
			expected:       " (error)",
			checkWidth:     true,
		},
		{
			name:           "long ASCII error truncated",
			errorText:      "failed to connect to database server at localhost:5432",
			availableWidth: 30,
			expected:       " (failed to connect to datab…)",
			checkWidth:     true,
		},
		{
			name:           "emoji in error message",
			errorText:      "connection failed 🔥 retry exhausted",
			availableWidth: 25,
			expected:       " (connection failed 🔥 …)",
			checkWidth:     true,
		},
		{
			name:           "CJK characters in error",
			errorText:      "数据库连接失败",
			availableWidth: 15,
			expected:       " (数据库连接…)",
			checkWidth:     true,
		},
		{
			name:           "mixed CJK and ASCII",
			errorText:      "Failed: 连接超时 timeout",
			availableWidth: 20,
			expected:       " (Failed: 连接超时…)",
			checkWidth:     true,
		},
		{
			name:           "very small available width returns ellipsis",
			errorText:      "error message",
			availableWidth: 4,
			expected:       " (…)",
			checkWidth:     true,
		},
		{
			name:           "minimal width just ellipsis",
			errorText:      "error message",
			availableWidth: 3,
			expected:       "…",
			checkWidth:     false,
		},
		{
			name:           "insufficient width for wrapper returns ellipsis",
			errorText:      "error",
			availableWidth: 2,
			expected:       "…",
			checkWidth:     false,
		},
		{
			name:           "zero width returns empty",
			errorText:      "error",
			availableWidth: 0,
			expected:       "",
			checkWidth:     false,
		},
		{
			name:           "negative width returns empty",
			errorText:      "error",
			availableWidth: -5,
			expected:       "",
			checkWidth:     false,
		},
		{
			name:           "empty error text",
			errorText:      "",
			availableWidth: 20,
			expected:       " ()",
			checkWidth:     true,
		},
		{
			name:           "accented characters",
			errorText:      "échec de connexion à l'année",
			availableWidth: 25,
			expected:       " (échec de connexion à …)",
			checkWidth:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateErrorMessage(tt.errorText, tt.availableWidth)
			assert.Equal(t, tt.expected, result)

			if tt.checkWidth {
				resultWidth := lipgloss.Width(result)
				assert.LessOrEqual(t, resultWidth, tt.availableWidth, "Result width should not exceed availableWidth")
			}
		})
	}
}

func Test_truncateErrorMessage_PreservesUTF8(t *testing.T) {
	errors := []string{
		"connection failed 🔥",
		"数据库错误",
		"échec système",
		"🌐 network error 💥",
		"混合mixed错误error",
	}

	for _, errText := range errors {
		t.Run(errText, func(t *testing.T) {
			result := truncateErrorMessage(errText, 20)

			assert.True(t, isValidUTF8(result), "Result should be valid UTF-8")

			width := lipgloss.Width(result)
			assert.LessOrEqual(t, width, 20, "Result width should not exceed availableWidth")
		})
	}
}

func Test_truncateErrorMessage_DisplayWidth(t *testing.T) {
	tests := []struct {
		name           string
		errorText      string
		availableWidth int
	}{
		{
			name:           "wide emoji error",
			errorText:      "🔥🔥🔥🔥🔥🔥",
			availableWidth: 15,
		},
		{
			name:           "CJK double-width error",
			errorText:      "測試錯誤訊息很長",
			availableWidth: 18,
		},
		{
			name:           "mixed width error",
			errorText:      "error-錯誤-🔥-failed",
			availableWidth: 20,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateErrorMessage(tt.errorText, tt.availableWidth)
			resultWidth := lipgloss.Width(result)

			t.Logf("Input: %q (width: %d)", tt.errorText, lipgloss.Width(tt.errorText))
			t.Logf("Result: %q (width: %d)", result, resultWidth)
			t.Logf("AvailableWidth: %d", tt.availableWidth)

			assert.LessOrEqual(t, resultWidth, tt.availableWidth, "Display width must not exceed availableWidth")
		})
	}
}

func Test_simplifyErrorMessage(t *testing.T) {
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
			expected: "not found",
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
			expected: "not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := simplifyErrorMessage(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}
