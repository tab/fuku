package logs

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/stretchr/testify/assert"
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
			result := truncateServiceName(tt.input, tt.maxWidth)
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
			result := truncateServiceName(name, 10)

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
		{name: "wide emoji", input: "🔥🔥🔥🔥🔥🔥🔥🔥", maxWidth: 10},
		{name: "CJK double-width", input: "測試服務器名稱很長", maxWidth: 12},
		{name: "mixed width", input: "test-測試-🔥-service", maxWidth: 15},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateServiceName(tt.input, tt.maxWidth)
			resultWidth := lipgloss.Width(result)

			t.Logf("Input: %q (width: %d)", tt.input, lipgloss.Width(tt.input))
			t.Logf("Result: %q (width: %d)", result, resultWidth)
			t.Logf("MaxWidth: %d", tt.maxWidth)

			assert.LessOrEqual(t, resultWidth, tt.maxWidth, "Display width must not exceed maxWidth")
		})
	}
}
