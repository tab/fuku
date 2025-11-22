package logs

import (
	"fmt"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/stretchr/testify/assert"

	"fuku/internal/app/ui"
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

func Test_updateContent_AppendOnly(t *testing.T) {
	m := NewModel()
	m.SetSize(80, 24)
	m.SetEnabled("api", true)
	m.SetEnabled("db", true)

	m.HandleLog(ui.LogEntry{Service: "api", Message: "Starting server"})
	m.HandleLog(ui.LogEntry{Service: "db", Message: "Connected"})

	firstContent := m.buildContent()
	assert.NotEmpty(t, firstContent)

	m.HandleLog(ui.LogEntry{Service: "api", Message: "Request processed"})

	secondContent := m.buildContent()
	assert.Contains(t, secondContent, firstContent, "New content should contain old content")
	assert.Contains(t, secondContent, "Request processed")
}

func Test_updateContent_FilterToggleTriggersRebuild(t *testing.T) {
	m := NewModel()
	m.SetSize(80, 24)
	m.SetEnabled("api", true)
	m.SetEnabled("db", false)

	m.HandleLog(ui.LogEntry{Service: "api", Message: "API log 1"})
	m.HandleLog(ui.LogEntry{Service: "db", Message: "DB log 1"})
	m.HandleLog(ui.LogEntry{Service: "api", Message: "API log 2"})

	contentWithoutDB := m.buildContent()
	assert.Contains(t, contentWithoutDB, "API log 1")
	assert.Contains(t, contentWithoutDB, "API log 2")
	assert.NotContains(t, contentWithoutDB, "DB log 1")

	m.SetEnabled("db", true)

	contentWithDB := m.buildContent()
	assert.Contains(t, contentWithDB, "API log 1")
	assert.Contains(t, contentWithDB, "API log 2")
	assert.Contains(t, contentWithDB, "DB log 1")
}

func Test_updateContent_WidthChangeTriggersRebuild(t *testing.T) {
	m := NewModel()
	m.SetSize(50, 24)
	m.SetEnabled("api", true)

	longMessage := "This is a very long log message that will definitely wrap at narrow widths but might fit on one line at wider terminal widths"
	m.HandleLog(ui.LogEntry{Service: "api", Message: longMessage})

	assert.Equal(t, 50, m.lastWidth)
	contentAt50 := m.buildContent()

	m.SetSize(200, 24)

	assert.Equal(t, 200, m.lastWidth)
	contentAt200 := m.buildContent()
	assert.NotEqual(t, contentAt50, contentAt200, "Content should be different after width change due to rewrap")
}

func Test_updateContent_NoDuplication(t *testing.T) {
	m := NewModel()
	m.SetSize(80, 24)
	m.SetEnabled("api", true)

	m.HandleLog(ui.LogEntry{Service: "api", Message: "Message 1"})
	m.HandleLog(ui.LogEntry{Service: "api", Message: "Message 2"})
	m.HandleLog(ui.LogEntry{Service: "api", Message: "Message 3"})

	content := m.buildContent()
	lines := countOccurrences(content, "Message 1")
	assert.Equal(t, 1, lines, "Each message should appear exactly once")
}

func Test_HandleLog_BufferTrimming(t *testing.T) {
	m := NewModel()
	m.SetSize(80, 24)
	m.SetEnabled("api", true)
	m.maxSize = 10
	m.entries = make([]Entry, 10)
	m.renderedLines = make([]string, 10)

	for i := 0; i < 15; i++ {
		entry := ui.LogEntry{
			Service: "api",
			Message: fmt.Sprintf("Log entry %d", i),
		}
		m.HandleLog(entry)
	}

	assert.Equal(t, 10, m.count, "Count should be capped at maxSize")
	assert.Equal(t, 10, m.maxSize, "MaxSize should be 10")

	assert.Contains(t, m.buildContent(), "Log entry 14", "Should contain newest entry")
	assert.NotContains(t, m.buildContent(), "Log entry 0", "Should not contain oldest trimmed entry")
	assert.NotContains(t, m.buildContent(), "Log entry 4", "Should not contain trimmed entries")

	oldestEntry := m.getEntry(0)
	assert.Contains(t, oldestEntry.Message, "Log entry 5", "Oldest entry should be #5 after trimming 5 entries")
}

func Test_updateContent_AppendOnlyPath(t *testing.T) {
	m := NewModel()
	m.SetSize(80, 24)
	m.SetEnabled("api", true)

	m.HandleLog(ui.LogEntry{Service: "api", Message: "First log"})
	assert.Equal(t, 0, m.lastRenderedIndex)

	initialContent := m.currentContent
	assert.Contains(t, initialContent, "First log")

	m.HandleLog(ui.LogEntry{Service: "api", Message: "Second log"})
	assert.Equal(t, 1, m.lastRenderedIndex)

	finalContent := m.currentContent
	assert.Contains(t, finalContent, initialContent, "Append-only: new content should contain old content")
	assert.Contains(t, finalContent, "Second log")

	occurrences := countOccurrences(finalContent, "First log")
	assert.Equal(t, 1, occurrences, "First log should appear exactly once (not duplicated)")
}

func Test_updateContent_RebuildOnFilterChange(t *testing.T) {
	m := NewModel()
	m.SetSize(80, 24)
	m.SetEnabled("api", true)
	m.SetEnabled("db", false)

	m.HandleLog(ui.LogEntry{Service: "api", Message: "API log"})
	m.HandleLog(ui.LogEntry{Service: "db", Message: "DB log"})

	contentBeforeFilter := m.currentContent
	assert.Contains(t, contentBeforeFilter, "API log")
	assert.NotContains(t, contentBeforeFilter, "DB log")
	assert.Equal(t, 1, m.lastRenderedIndex)

	m.SetEnabled("db", true)

	assert.Equal(t, 1, m.lastRenderedIndex, "After rebuild, lastProcessed should be count-1")
	contentAfterFilter := m.currentContent
	assert.Contains(t, contentAfterFilter, "API log")
	assert.Contains(t, contentAfterFilter, "DB log")
}

func Test_updateContent_RebuildOnWidthChange(t *testing.T) {
	m := NewModel()
	m.SetSize(50, 24)
	m.SetEnabled("api", true)

	longMsg := "This is a very long message that will wrap differently at different widths for testing purposes and to ensure proper behavior"
	m.HandleLog(ui.LogEntry{Service: "api", Message: longMsg})

	contentAt50 := m.currentContent
	assert.NotEmpty(t, contentAt50)
	assert.Equal(t, 0, m.lastRenderedIndex)

	m.SetSize(200, 24)

	contentAt200 := m.currentContent
	assert.NotEqual(t, contentAt50, contentAt200, "Width change should trigger rebuild with different wrapping")
	assert.Equal(t, 0, m.lastRenderedIndex, "After rebuild, lastProcessed should be count-1")
}

func Test_updateContent_RebuildOnBufferEviction(t *testing.T) {
	m := NewModel()
	m.SetSize(80, 24)
	m.SetEnabled("api", true)
	m.maxSize = 5
	m.entries = make([]Entry, 5)
	m.renderedLines = make([]string, 5)
	m.contentLines = make([]string, 5)

	for i := 0; i < 3; i++ {
		m.HandleLog(ui.LogEntry{Service: "api", Message: fmt.Sprintf("Log %d", i)})
	}

	assert.Equal(t, 3, m.count)
	assert.Equal(t, 2, m.lastRenderedIndex)
	assert.Contains(t, m.currentContent, "Log 0")

	for i := 3; i < 7; i++ {
		m.HandleLog(ui.LogEntry{Service: "api", Message: fmt.Sprintf("Log %d", i)})
	}

	assert.Equal(t, 5, m.count, "Buffer should be capped at maxSize")
	assert.NotContains(t, m.currentContent, "Log 0", "Evicted entry should not be in content after rebuild")
	assert.NotContains(t, m.currentContent, "Log 1", "Evicted entry should not be in content after rebuild")
	assert.Contains(t, m.currentContent, "Log 6", "Latest entry should be in content")
}

func countOccurrences(text, substr string) int {
	count := 0
	idx := 0

	for {
		pos := indexFrom(text, substr, idx)
		if pos == -1 {
			break
		}

		count++
		idx = pos + len(substr)
	}

	return count
}

func indexFrom(text, substr string, start int) int {
	if start >= len(text) {
		return -1
	}

	idx := 0

	for i := start; i <= len(text)-len(substr); i++ {
		if text[i:i+len(substr)] == substr {
			return i
		}

		idx++
	}

	return -1
}

func Benchmark_HandleLog_IncrementalAppend(b *testing.B) {
	m := NewModel()
	m.SetSize(80, 24)
	m.SetEnabled("api", true)

	for i := 0; i < 500; i++ {
		m.HandleLog(ui.LogEntry{Service: "api", Message: fmt.Sprintf("Initial log %d", i)})
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		m.HandleLog(ui.LogEntry{Service: "api", Message: fmt.Sprintf("Benchmark log %d", i)})
	}
}

func Benchmark_HandleLog_FullRebuild(b *testing.B) {
	m := NewModel()
	m.SetSize(80, 24)
	m.SetEnabled("api", true)

	for i := 0; i < 500; i++ {
		m.HandleLog(ui.LogEntry{Service: "api", Message: fmt.Sprintf("Initial log %d", i)})
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		m.invalidateContent()
		m.HandleLog(ui.LogEntry{Service: "api", Message: fmt.Sprintf("Rebuild log %d", i)})
	}
}

func Benchmark_HandleLog_LargeBuffer(b *testing.B) {
	m := NewModel()
	m.SetSize(80, 24)
	m.SetEnabled("api", true)

	for i := 0; i < 1000; i++ {
		m.HandleLog(ui.LogEntry{Service: "api", Message: fmt.Sprintf("Fill buffer %d", i)})
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		m.HandleLog(ui.LogEntry{Service: "api", Message: fmt.Sprintf("Benchmark log %d", i)})
	}
}
