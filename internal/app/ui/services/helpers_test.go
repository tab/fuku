package services

import (
	"fmt"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/stretchr/testify/assert"

	"fuku/internal/app/errors"
	"fuku/internal/app/ui/components"
)

func Test_truncateServiceName(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		maxWidth   int
		expected   string
		checkWidth bool
	}{
		{
			name:       "short name no truncation",
			input:      "api",
			maxWidth:   15,
			expected:   "api",
			checkWidth: true,
		},
		{
			name:       "exact fit no truncation",
			input:      "exact-fit-name",
			maxWidth:   14,
			expected:   "exact-fit-name",
			checkWidth: true,
		},
		{
			name:       "ASCII truncation",
			input:      "very-long-service-name",
			maxWidth:   15,
			expected:   "very-long-serv…",
			checkWidth: true,
		},
		{
			name:       "emoji truncation preserves UTF-8",
			input:      "test-🔥-service",
			maxWidth:   10,
			expected:   "test-🔥-s…",
			checkWidth: true,
		},
		{
			name:       "emoji at boundary",
			input:      "service-🔥🔥🔥",
			maxWidth:   12,
			expected:   "service-🔥…",
			checkWidth: true,
		},
		{
			name:       "CJK characters",
			input:      "测试服务器名称",
			maxWidth:   10,
			expected:   "测试服务…",
			checkWidth: true,
		},
		{
			name:       "mixed CJK and ASCII",
			input:      "api-测试-service",
			maxWidth:   12,
			expected:   "api-测试-se…",
			checkWidth: true,
		},
		{
			name:       "accented characters",
			input:      "café-service-année",
			maxWidth:   15,
			expected:   "café-service-a…",
			checkWidth: true,
		},
		{
			name:       "maxWidth smaller than ellipsis",
			input:      "service",
			maxWidth:   0,
			expected:   "…",
			checkWidth: false,
		},
		{
			name:       "maxWidth equals ellipsis width",
			input:      "service-name",
			maxWidth:   1,
			expected:   "…",
			checkWidth: false,
		},
		{
			name:       "very small maxWidth",
			input:      "service-name",
			maxWidth:   3,
			expected:   "se…",
			checkWidth: true,
		},
		{
			name:       "empty string",
			input:      "",
			maxWidth:   10,
			expected:   "",
			checkWidth: true,
		},
		{
			name:       "only emoji",
			input:      "🔥🔥🔥🔥🔥",
			maxWidth:   6,
			expected:   "🔥🔥…",
			checkWidth: true,
		},
		{
			name:       "wide chars exceed maxWidth",
			input:      "測試",
			maxWidth:   3,
			expected:   "測…",
			checkWidth: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := components.Truncate(tt.input, tt.maxWidth)
			assert.Equal(t, tt.expected, result)

			if tt.checkWidth {
				resultWidth := lipgloss.Width(result)
				assert.LessOrEqual(t, resultWidth, tt.maxWidth, "Result width should not exceed maxWidth")
			}
		})
	}
}

func Test_truncateServiceName_PreservesUTF8(t *testing.T) {
	names := []string{
		"service-🔥-api",
		"测试服务",
		"café-år",
		"🌐🔥💥",
		"混合mixed文字text",
	}

	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			result := components.Truncate(name, 10)

			// Result should be valid UTF-8
			assert.True(t, isValidUTF8(result), "Result should be valid UTF-8")

			// Result should not exceed maxWidth
			width := lipgloss.Width(result)
			assert.LessOrEqual(t, width, 10, "Result width should not exceed maxWidth")
		})
	}
}

func isValidUTF8(s string) bool {
	for _, r := range s {
		if r == '\uFFFD' {
			// Check if this is a legitimate replacement character in input
			// or a result of invalid UTF-8
			return false
		}
	}

	return true
}

func Test_truncateServiceName_DisplayWidth(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxWidth int
	}{
		{
			name:     "wide emoji",
			input:    "🔥🔥🔥🔥🔥🔥🔥🔥",
			maxWidth: 10,
		},
		{
			name:     "CJK double-width",
			input:    "測試服務器名稱很長",
			maxWidth: 12,
		},
		{
			name:     "mixed width",
			input:    "test-測試-🔥-service",
			maxWidth: 15,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := components.Truncate(tt.input, tt.maxWidth)
			resultWidth := lipgloss.Width(result)

			t.Logf("Input: %q (width: %d)", tt.input, lipgloss.Width(tt.input))
			t.Logf("Result: %q (width: %d)", result, resultWidth)
			t.Logf("MaxWidth: %d", tt.maxWidth)

			assert.LessOrEqual(t, resultWidth, tt.maxWidth, "Display width must not exceed maxWidth")
		})
	}
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

			// Result should be valid UTF-8
			assert.True(t, isValidUTF8(result), "Result should be valid UTF-8")

			// Result should not exceed availableWidth
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

func Test_padServiceName(t *testing.T) {
	tests := []struct {
		name        string
		serviceName string
		maxWidth    int
		wantWidth   int
	}{
		{
			name:        "ASCII name padded to width",
			serviceName: "api",
			maxWidth:    20,
			wantWidth:   20,
		},
		{
			name:        "emoji name padded correctly",
			serviceName: "api-🔥",
			maxWidth:    20,
			wantWidth:   20,
		},
		{
			name:        "CJK name padded correctly",
			serviceName: "测试服务",
			maxWidth:    20,
			wantWidth:   20,
		},
		{
			name:        "mixed width name padded correctly",
			serviceName: "svc-测试-🔥",
			maxWidth:    25,
			wantWidth:   25,
		},
		{
			name:        "exact fit no padding",
			serviceName: "exact-fit-service123",
			maxWidth:    20,
			wantWidth:   20,
		},
		{
			name:        "name wider than maxWidth returns as-is",
			serviceName: "very-long-service-name",
			maxWidth:    10,
			wantWidth:   22,
		},
		{
			name:        "empty name padded to full width",
			serviceName: "",
			maxWidth:    15,
			wantWidth:   15,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := padServiceName(tt.serviceName, tt.maxWidth)
			resultWidth := lipgloss.Width(result)

			assert.Equal(t, tt.wantWidth, resultWidth, "Padded width should match expected")

			// Verify name is preserved at start
			assert.True(t, len(result) >= len(tt.serviceName), "Result should contain original name")

			if len(tt.serviceName) > 0 {
				assert.Equal(t, tt.serviceName, result[:len(tt.serviceName)], "Original name should be preserved")
			}
		})
	}
}

func Test_padServiceName_AlignmentConsistency(t *testing.T) {
	// Test that names of different widths but same display width get same padding
	names := []struct {
		name         string
		displayWidth int
	}{
		{"service", 7}, // 7 ASCII chars = 7 display width
		{"api-🔥", 6},   // 4 ASCII + 1 emoji (width 2) = 6 display width
		{"测试服", 6},     // 3 CJK chars (width 2 each) = 6 display width
	}

	maxWidth := 20

	for _, tt := range names {
		t.Run(tt.name, func(t *testing.T) {
			result := padServiceName(tt.name, maxWidth)
			resultWidth := lipgloss.Width(result)

			assert.Equal(t, maxWidth, resultWidth, "All names should pad to same display width")
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
			result := simplifyErrorMessage(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}
