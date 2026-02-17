package components

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_RenderPanel(t *testing.T) {
	t.Run("renders basic panel", func(t *testing.T) {
		opts := PanelOptions{
			Title:   "Test",
			Content: "Content",
			Help:    "Help",
			Status:  "Info",
			Version: "v1.0",
			Height:  10,
			Width:   40,
		}

		result := RenderPanel(opts)

		assert.NotEmpty(t, result)
		assert.Contains(t, result, "Test")
	})

	t.Run("handles minimum dimensions", func(t *testing.T) {
		opts := PanelOptions{
			Title:   "T",
			Content: "C",
			Height:  2,
			Width:   10,
		}

		result := RenderPanel(opts)

		assert.NotEmpty(t, result)
	})

	t.Run("handles empty content", func(t *testing.T) {
		opts := PanelOptions{
			Title:  "Title",
			Height: 5,
			Width:  20,
		}

		result := RenderPanel(opts)

		assert.NotEmpty(t, result)
	})
}

func Test_PadRight(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		width  int
		expect string
	}{
		{
			name:   "empty string pad to 5",
			input:  "",
			width:  5,
			expect: "     ",
		},
		{
			name:   "short string pad to 10",
			input:  "hello",
			width:  10,
			expect: "hello     ",
		},
		{
			name:   "exact width no padding",
			input:  "hello",
			width:  5,
			expect: "hello",
		},
		{
			name:   "longer than width no change",
			input:  "hello world",
			width:  5,
			expect: "hello world",
		},
		{
			name:   "unicode text padding",
			input:  "日本",
			width:  10,
			expect: "日本      ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := PadRight(tt.input, tt.width)
			assert.Equal(t, tt.expect, result)
		})
	}
}

func Test_PadRight_WithStyles(t *testing.T) {
	t.Run("handles ANSI escape sequences", func(t *testing.T) {
		styled := "\x1b[31mred\x1b[0m"
		result := PadRight(styled, 10)
		assert.True(t, strings.HasPrefix(result, "\x1b[31mred\x1b[0m"))
	})
}

func Test_TruncateAndPad(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		width  int
		expect string
	}{
		{
			name:   "exact width no change",
			input:  "hello",
			width:  5,
			expect: "hello",
		},
		{
			name:   "shorter than width pads",
			input:  "hi",
			width:  5,
			expect: "hi   ",
		},
		{
			name:   "longer than width truncates",
			input:  "hello world",
			width:  8,
			expect: "hello w…",
		},
		{
			name:   "empty string pads",
			input:  "",
			width:  3,
			expect: "   ",
		},
		{
			name:   "width 1 returns ellipsis",
			input:  "hello",
			width:  1,
			expect: "…",
		},
		{
			name:   "width 0 returns ellipsis",
			input:  "hello",
			width:  0,
			expect: "…",
		},
		{
			name:   "unicode truncation",
			input:  "日本語テスト",
			width:  6,
			expect: "日本… ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TruncateAndPad(tt.input, tt.width)
			assert.Equal(t, tt.expect, result)
		})
	}
}

func Test_TruncateAndPad_EdgeCases(t *testing.T) {
	t.Run("negative width returns ellipsis", func(t *testing.T) {
		result := TruncateAndPad("hello", -1)
		assert.Equal(t, "…", result)
	})

	t.Run("single character string exact width", func(t *testing.T) {
		result := TruncateAndPad("a", 1)
		assert.Equal(t, "a", result)
	})

	t.Run("all spaces", func(t *testing.T) {
		result := TruncateAndPad("   ", 5)
		assert.Equal(t, "     ", result)
	})
}

func Test_BuildTopBorder(t *testing.T) {
	border := func(s string) string { return s }

	t.Run("builds top border with title and right text", func(t *testing.T) {
		result := BuildTopBorder(border, "Title", "Info", 40)

		assert.NotEmpty(t, result)
		assert.Contains(t, result, "Title")
		assert.Contains(t, result, "Info")
		assert.Contains(t, result, BorderTopLeft)
		assert.Contains(t, result, BorderTopRight)
	})

	t.Run("builds top border with empty right text", func(t *testing.T) {
		result := BuildTopBorder(border, "logs", "", 40)

		assert.NotEmpty(t, result)
		assert.Contains(t, result, "logs")
		assert.Contains(t, result, BorderTopLeft)
		assert.Contains(t, result, BorderTopRight)
	})

	t.Run("builds top border with empty title", func(t *testing.T) {
		result := BuildTopBorder(border, "", "", 20)

		assert.NotEmpty(t, result)
		assert.Contains(t, result, BorderTopLeft)
		assert.Contains(t, result, BorderTopRight)
	})
}

func Test_BuildBottomBorder(t *testing.T) {
	border := func(s string) string { return s }

	t.Run("builds bottom border with version only", func(t *testing.T) {
		result := BuildBottomBorder(border, "", "v1.0", 40)

		assert.NotEmpty(t, result)
		assert.Contains(t, result, BorderBottomLeft)
		assert.Contains(t, result, BorderBottomRight)
	})

	t.Run("builds bottom border with info and version", func(t *testing.T) {
		result := BuildBottomBorder(border, "cpu 0.5% mem 12MB", "v1.0", 60)

		assert.NotEmpty(t, result)
		assert.Contains(t, result, BorderBottomLeft)
		assert.Contains(t, result, BorderBottomRight)
		assert.Contains(t, result, "cpu 0.5% mem 12MB")
		assert.Contains(t, result, "v1.0")
	})

	t.Run("handles minimum width", func(t *testing.T) {
		result := BuildBottomBorder(border, "", "very-long-version-text", 10)

		assert.NotEmpty(t, result)
		assert.Contains(t, result, BorderBottomLeft)
	})

	t.Run("handles empty text", func(t *testing.T) {
		result := BuildBottomBorder(border, "", "", 20)

		assert.NotEmpty(t, result)
	})
}

func Test_splitAndPadContent(t *testing.T) {
	tests := []struct {
		name    string
		content string
		height  int
		expect  []string
	}{
		{
			name:    "empty content pads to height",
			content: "",
			height:  3,
			expect:  []string{"", "", ""},
		},
		{
			name:    "single line pads to height",
			content: "line1",
			height:  3,
			expect:  []string{"line1", "", ""},
		},
		{
			name:    "exact lines no padding",
			content: "line1\nline2",
			height:  2,
			expect:  []string{"line1", "line2"},
		},
		{
			name:    "more lines than height truncates",
			content: "line1\nline2\nline3",
			height:  2,
			expect:  []string{"line1", "line2"},
		},
		{
			name:    "newline only content",
			content: "\n\n",
			height:  2,
			expect:  []string{"", ""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := splitAndPadContent(tt.content, tt.height)
			assert.Equal(t, tt.expect, result)
		})
	}
}

func Test_AppendContentLines(t *testing.T) {
	border := func(s string) string { return "[" + s + "]" }

	t.Run("appends bordered lines", func(t *testing.T) {
		lines := []string{"header"}
		contentLines := []string{"content1", "content2"}
		innerWidth := 10

		result := AppendContentLines(lines, contentLines, innerWidth, border)

		assert.Len(t, result, 3)
		assert.Equal(t, "header", result[0])
		assert.Contains(t, result[1], "[│]")
		assert.Contains(t, result[2], "[│]")
	})

	t.Run("handles empty content lines", func(t *testing.T) {
		lines := []string{}
		contentLines := []string{""}
		innerWidth := 5

		result := AppendContentLines(lines, contentLines, innerWidth, border)

		assert.Len(t, result, 1)
	})

	t.Run("handles negative padding", func(t *testing.T) {
		lines := []string{}
		contentLines := []string{"very long content line"}
		innerWidth := 5

		result := AppendContentLines(lines, contentLines, innerWidth, border)

		assert.Len(t, result, 1)
	})
}

func Test_SplitAtDisplayWidth(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantLeft  string
		wantRight string
	}{
		{
			name:      "empty string",
			input:     "",
			wantLeft:  "",
			wantRight: "",
		},
		{
			name:      "single character",
			input:     "a",
			wantLeft:  "",
			wantRight: "a",
		},
		{
			name:      "two ASCII chars",
			input:     "ab",
			wantLeft:  "a",
			wantRight: "b",
		},
		{
			name:      "three ASCII chars",
			input:     "abc",
			wantLeft:  "a",
			wantRight: "bc",
		},
		{
			name:      "four ASCII chars",
			input:     "abcd",
			wantLeft:  "ab",
			wantRight: "cd",
		},
		{
			name:      "two spaces",
			input:     "  ",
			wantLeft:  " ",
			wantRight: " ",
		},
		{
			name:      "four spaces",
			input:     "    ",
			wantLeft:  "  ",
			wantRight: "  ",
		},
		{
			name:      "unicode text",
			input:     "日本語",
			wantLeft:  "日",
			wantRight: "本語",
		},
		{
			name:      "mixed ASCII and unicode",
			input:     "ab日本",
			wantLeft:  "ab",
			wantRight: "日本",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			left, right := SplitAtDisplayWidth(tt.input)
			assert.Equal(t, tt.wantLeft, left)
			assert.Equal(t, tt.wantRight, right)
		})
	}
}

func Test_SplitAtDisplayWidth_WithWideCharacters(t *testing.T) {
	t.Run("handles emoji", func(t *testing.T) {
		left, right := SplitAtDisplayWidth("🎉🎊")
		combined := left + right
		assert.Equal(t, "🎉🎊", combined)
	})

	t.Run("handles CJK characters", func(t *testing.T) {
		input := "中文字"
		left, right := SplitAtDisplayWidth(input)
		assert.Equal(t, input, left+right)
	})
}

func Test_renderFooter(t *testing.T) {
	tests := []struct {
		name     string
		help     string
		tips     string
		width    int
		wantHelp bool
		wantTips bool
	}{
		{
			name:     "help only when tips empty",
			help:     "Help",
			tips:     "",
			width:    80,
			wantHelp: true,
			wantTips: false,
		},
		{
			name:     "both help and tips",
			help:     "Help",
			tips:     "Tip",
			width:    80,
			wantHelp: true,
			wantTips: true,
		},
		{
			name:     "narrow width hides tips",
			help:     "Help text here",
			tips:     "Long tip",
			width:    10,
			wantHelp: true,
			wantTips: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := renderFooter(tt.help, tt.tips, tt.width)

			if tt.wantHelp {
				assert.Contains(t, result, tt.help)
			}

			if tt.wantTips {
				assert.Contains(t, result, tt.tips)
			}
		})
	}
}
