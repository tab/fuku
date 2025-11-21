package logs

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_WrapText(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		maxWidth int
		expected []string
	}{
		{
			name:     "short text no wrap",
			text:     "Hello world",
			maxWidth: 20,
			expected: []string{"Hello world"},
		},
		{
			name:     "exact fit no wrap",
			text:     "Hello",
			maxWidth: 5,
			expected: []string{"Hello"},
		},
		{
			name:     "wrap at whitespace boundary",
			text:     "Hello world this is a test",
			maxWidth: 15,
			expected: []string{"Hello world", "this is a test"},
		},
		{
			name:     "wrap long text at whitespace",
			text:     "X-Trace-ID=1e2f3a4b-5c6d-7e8f-9a0b Session-ID=sess_auth_xyz789",
			maxWidth: 20,
			expected: []string{"X-Trace-ID=1e2f3a4b-", "5c6d-7e8f-9a0b", "Session-ID=sess_auth", "_xyz789"},
		},
		{
			name:     "empty text",
			text:     "",
			maxWidth: 10,
			expected: []string{""},
		},
		{
			name:     "single long word",
			text:     "verylongword",
			maxWidth: 5,
			expected: []string{"veryl", "ongwo", "rd"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := wrapText(tt.text, tt.maxWidth)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_WrapText_LogExample(t *testing.T) {
	logMessage := "Authentication successful for user X-Trace-ID=1e2f3a4b-5c6d-7e8f-9a0b-1c2d3e4f5g6h Session-ID=sess_auth_xyz789 User-ID=user_john_doe Email=john.doe@example.com IP=10.0.1.42"
	maxWidth := 60

	result := wrapText(logMessage, maxWidth)

	assert.Greater(t, len(result), 1)

	for _, line := range result {
		assert.Equal(t, strings.TrimRight(line, " \t"), line, "Line should not have trailing whitespace")
	}
}

func Test_WrapText_WithAnsiCodes(t *testing.T) {
	styledText := "\x1b[31mERROR\x1b[0m: Failed authentication attempt X-Trace-ID=2f3a4b5c-6d7e-8f9a-0b1c-2d3e4f5g6h7i Session-ID=sess_fail_abc123"
	maxWidth := 80

	result := wrapText(styledText, maxWidth)

	assert.Greater(t, len(result), 1)

	for _, line := range result {
		assert.Equal(t, strings.TrimRight(line, " \t"), line, "Line should not have trailing whitespace")
	}
}

func Test_WrapText_PlainTextNoAnsi(t *testing.T) {
	plainText := "Database connection timeout X-Trace-ID=8b9c0d1e-2f3a-4b5c-6d7e-8f9a0b1c2d3e Session-ID=sess_def456uvw789 Request-ID=req_12345 Endpoint=/api/v1/orders User-ID=user_98765 Duration=5023ms Error=connection_timeout"
	maxWidth := 80

	result := wrapText(plainText, maxWidth)

	assert.Greater(t, len(result), 1)

	for _, line := range result {
		assert.Equal(t, strings.TrimRight(line, " \t"), line, "Line should not have trailing whitespace")
	}
}

func Test_WrapText_FillsLines(t *testing.T) {
	text := "Session-ID=sess_def456uvw789 Request-ID=req_12345 Endpoint=/api/v1/orders User-ID=user_98765 Duration=5023ms Error=connection_timeout"
	maxWidth := 80

	result := wrapText(text, maxWidth)

	t.Logf("Wrapped lines (maxWidth=%d):", maxWidth)

	for i, line := range result {
		t.Logf("  Line %d (width=%d): %q", i, len(line), line)
	}

	assert.Greater(t, len(result), 1, "Text should wrap into multiple lines")

	for _, line := range result {
		assert.Equal(t, strings.TrimRight(line, " \t"), line, "Lines should not have trailing whitespace")
	}
}

func Test_WrapText_SmartWhitespaceBreaking(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		maxWidth int
		check    func(t *testing.T, result []string)
	}{
		{
			name:     "breaks at close whitespace",
			text:     "Session-ID=sess_def456uvw789 Request-ID=req_12345 Endpoint=/api/v1/orders User-ID=user_98765",
			maxWidth: 80,
			check: func(t *testing.T, result []string) {
				t.Logf("Result: %v", result)

				for i, line := range result {
					if i < len(result)-1 {
						assert.GreaterOrEqual(t, len(line), 60, "Line %d should use most of maxWidth", i)
					}
				}
			},
		},
		{
			name:     "rejects distant whitespace",
			text:     "short VeryLongTokenThatWouldWasteLotsOfSpaceIfWeBreakAtEarlySpace MoreText",
			maxWidth: 60,
			check: func(t *testing.T, result []string) {
				t.Logf("Result: %v", result)
				firstLineWidth := len(result[0])
				assert.GreaterOrEqual(t, firstLineWidth, 40, "Should not waste more than 20 chars by breaking at distant space")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := wrapText(tt.text, tt.maxWidth)
			tt.check(t, result)
		})
	}
}

func Test_WrapText_WideCharacters(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		maxWidth int
	}{
		{
			name:     "emoji wider than maxWidth",
			text:     "🔥🔥🔥🔥🔥",
			maxWidth: 3,
		},
		{
			name:     "single wide emoji",
			text:     "🔥",
			maxWidth: 1,
		},
		{
			name:     "text with emoji",
			text:     "Hello 🔥 world",
			maxWidth: 10,
		},
		{
			name:     "mixed ASCII and emoji",
			text:     "Error: 💥 Failed to connect 🌐",
			maxWidth: 15,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := wrapText(tt.text, tt.maxWidth)
			assert.NotEmpty(t, result, "Should not return empty result")
			assert.NotEqual(t, []string{""}, result, "Should not return single empty string")

			for _, line := range result {
				assert.Equal(t, strings.TrimRight(line, " \t"), line, "Line should not have trailing whitespace")
			}
		})
	}
}

func Benchmark_WrapText_ShortLine(b *testing.B) {
	text := "Session-ID=sess_auth_xyz789 User-ID=user_john_doe Email=john.doe@example.com"
	maxWidth := 80

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = wrapText(text, maxWidth)
	}
}

func Benchmark_WrapText_LongLine(b *testing.B) {
	text := strings.Repeat("X-Trace-ID=1e2f3a4b-5c6d-7e8f-9a0b-1c2d3e4f5g6h Session-ID=sess_auth_xyz789 User-ID=user_john_doe Email=john.doe@example.com IP=10.0.1.42 ", 10)
	maxWidth := 80

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = wrapText(text, maxWidth)
	}
}

func Benchmark_WrapText_WithANSI(b *testing.B) {
	text := "\x1b[31mERROR\x1b[0m: Failed authentication attempt X-Trace-ID=2f3a4b5c-6d7e-8f9a-0b1c-2d3e4f5g6h7i Session-ID=sess_fail_abc123 \x1b[33mUser-ID=user_99999\x1b[0m Email=test@example.com"
	maxWidth := 80

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = wrapText(text, maxWidth)
	}
}
