package terminal

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
)

func Test_ComputeTableLayout(t *testing.T) {
	tests := []struct {
		name              string
		contentWidth      int
		preferredNameText int
		metricColumns     int
		want              TableLayout
	}{
		{
			name:              "negative contentWidth clamped to zero",
			contentWidth:      -5,
			preferredNameText: ServiceNameWidthLong,
			metricColumns:     MetricFullColumnCount,
			want:              TableLayout{ContentWidth: 0, ServiceNameWidth: 0, TimelineWidth: 0, StatusWidth: 0, MetricWidth: 0, MetricColumns: MetricFullColumnCount},
		},
		{
			name:              "zero contentWidth",
			contentWidth:      0,
			preferredNameText: ServiceNameWidthLong,
			metricColumns:     MetricFullColumnCount,
			want:              TableLayout{ContentWidth: 0, ServiceNameWidth: 0, TimelineWidth: 0, StatusWidth: 0, MetricWidth: 0, MetricColumns: MetricFullColumnCount},
		},
		{
			name:              "long bucket - narrow terminal (72 cols) - timeline hidden",
			contentWidth:      66,
			preferredNameText: ServiceNameWidthLong,
			metricColumns:     MetricFullColumnCount,
			want:              TableLayout{ContentWidth: 66, ServiceNameWidth: 29, TimelineWidth: 0, StatusWidth: 13, MetricWidth: 6, MetricColumns: MetricFullColumnCount},
		},
		{
			name:              "long bucket - medium terminal (97 cols) - timeline at minimum",
			contentWidth:      97,
			preferredNameText: ServiceNameWidthLong,
			metricColumns:     MetricFullColumnCount,
			want:              TableLayout{ContentWidth: 97, ServiceNameWidth: 36, TimelineWidth: 8, TimelineGapWidth: 1, StatusWidth: 16, MetricWidth: 9, MetricColumns: MetricFullColumnCount},
		},
		{
			name:              "long bucket - medium terminal (100 cols) - timeline at minimum",
			contentWidth:      100,
			preferredNameText: ServiceNameWidthLong,
			metricColumns:     MetricFullColumnCount,
			want:              TableLayout{ContentWidth: 100, ServiceNameWidth: 35, TimelineWidth: 8, TimelineGapWidth: 1, StatusWidth: 16, MetricWidth: 10, MetricColumns: MetricFullColumnCount},
		},
		{
			name:              "long bucket - wide terminal (114 cols) - timeline at minimum",
			contentWidth:      114,
			preferredNameText: ServiceNameWidthLong,
			metricColumns:     MetricFullColumnCount,
			want:              TableLayout{ContentWidth: 114, ServiceNameWidth: 45, TimelineWidth: 8, TimelineGapWidth: 1, StatusWidth: 16, MetricWidth: 11, MetricColumns: MetricFullColumnCount},
		},
		{
			name:              "long bucket - wide terminal (120 cols) - timeline at minimum",
			contentWidth:      120,
			preferredNameText: ServiceNameWidthLong,
			metricColumns:     MetricFullColumnCount,
			want:              TableLayout{ContentWidth: 120, ServiceNameWidth: 47, TimelineWidth: 8, TimelineGapWidth: 1, StatusWidth: 16, MetricWidth: 12, MetricColumns: MetricFullColumnCount},
		},
		{
			name:              "long bucket - ultra-wide terminal - large surplus split into flex gaps",
			contentWidth:      200,
			preferredNameText: ServiceNameWidthLong,
			metricColumns:     MetricFullColumnCount,
			want:              TableLayout{ContentWidth: 200, ServiceNameWidth: 50, LeftFlexWidth: 34, TimelineWidth: 16, TimelineGapWidth: 1, StatusWidth: 16, RightFlexWidth: 35, MetricWidth: 12, MetricColumns: MetricFullColumnCount},
		},
		{
			name:              "short bucket - wide terminal (114 cols) - surplus split into flex gaps",
			contentWidth:      114,
			preferredNameText: ServiceNameWidthShort,
			metricColumns:     MetricFullColumnCount,
			want:              TableLayout{ContentWidth: 114, ServiceNameWidth: 18, LeftFlexWidth: 9, TimelineWidth: 16, TimelineGapWidth: 1, StatusWidth: 16, RightFlexWidth: 10, MetricWidth: 11, MetricColumns: MetricFullColumnCount},
		},
		{
			name:              "medium bucket - wide terminal (114 cols)",
			contentWidth:      114,
			preferredNameText: ServiceNameWidthMedium,
			metricColumns:     MetricFullColumnCount,
			want:              TableLayout{ContentWidth: 114, ServiceNameWidth: 34, LeftFlexWidth: 1, TimelineWidth: 16, TimelineGapWidth: 1, StatusWidth: 16, RightFlexWidth: 2, MetricWidth: 11, MetricColumns: MetricFullColumnCount},
		},
		{
			name:              "short bucket - narrow terminal (72 cols) - timeline appears",
			contentWidth:      66,
			preferredNameText: ServiceNameWidthShort,
			metricColumns:     MetricFullColumnCount,
			want:              TableLayout{ContentWidth: 66, ServiceNameWidth: 18, TimelineWidth: 10, TimelineGapWidth: 1, StatusWidth: 13, MetricWidth: 6, MetricColumns: MetricFullColumnCount},
		},
		{
			name:              "metrics hidden - surplus flows to flex gaps",
			contentWidth:      80,
			preferredNameText: ServiceNameWidthShort,
			metricColumns:     0,
			want:              TableLayout{ContentWidth: 80, ServiceNameWidth: 18, LeftFlexWidth: 14, TimelineWidth: 16, TimelineGapWidth: 1, StatusWidth: 16, RightFlexWidth: 15, MetricWidth: 0},
		},
		{
			name:              "negative metric columns clamped to zero",
			contentWidth:      80,
			preferredNameText: ServiceNameWidthShort,
			metricColumns:     -1,
			want:              TableLayout{ContentWidth: 80, ServiceNameWidth: 18, LeftFlexWidth: 14, TimelineWidth: 16, TimelineGapWidth: 1, StatusWidth: 16, RightFlexWidth: 15, MetricWidth: 0},
		},
		{
			name:              "metrics hidden - narrow terminal grows name to absorb surplus",
			contentWidth:      60,
			preferredNameText: ServiceNameWidthLong,
			metricColumns:     0,
			want:              TableLayout{ContentWidth: 60, ServiceNameWidth: 39, TimelineWidth: 8, TimelineGapWidth: 1, StatusWidth: 12, MetricWidth: 0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ComputeTableLayout(tt.contentWidth, tt.preferredNameText, tt.metricColumns)
			assert.Equal(t, tt.want, result)
		})
	}
}

func Test_ComputeCompactLayout(t *testing.T) {
	tests := []struct {
		name              string
		contentWidth      int
		preferredNameText int
		expected          TableLayout
	}{
		{
			name:              "negative contentWidth clamped to zero",
			contentWidth:      -5,
			preferredNameText: ServiceNameWidthShort,
			expected:          TableLayout{ContentWidth: 0, ServiceNameWidth: 0, LeftFlexWidth: 0, StatusWidth: 0},
		},
		{
			name:              "name and status fit - surplus goes to the left flex",
			contentWidth:      60,
			preferredNameText: ServiceNameWidthShort,
			expected:          TableLayout{ContentWidth: 60, ServiceNameWidth: 19, LeftFlexWidth: 31, StatusWidth: 10},
		},
		{
			name:              "narrow width - name shrinks to what the status leaves",
			contentWidth:      25,
			preferredNameText: ServiceNameWidthMedium,
			expected:          TableLayout{ContentWidth: 25, ServiceNameWidth: 15, LeftFlexWidth: 0, StatusWidth: 10},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ComputeCompactLayout(tt.contentWidth, tt.preferredNameText)

			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_PreferredNameTextWidth(t *testing.T) {
	tests := []struct {
		name        string
		longestName int
		want        int
	}{
		{
			name:        "no services (zero)",
			longestName: 0,
			want:        ServiceNameWidthShort,
		},
		{
			name:        "short names (api, web)",
			longestName: 8,
			want:        ServiceNameWidthShort,
		},
		{
			name:        "exactly at short bucket boundary",
			longestName: 16,
			want:        ServiceNameWidthShort,
		},
		{
			name:        "just over short bucket - medium",
			longestName: 17,
			want:        ServiceNameWidthMedium,
		},
		{
			name:        "medium names (loki-frontend-web)",
			longestName: 17,
			want:        ServiceNameWidthMedium,
		},
		{
			name:        "exactly at medium bucket boundary",
			longestName: 32,
			want:        ServiceNameWidthMedium,
		},
		{
			name:        "just over medium bucket - long",
			longestName: 33,
			want:        ServiceNameWidthLong,
		},
		{
			name:        "long names (card-reference-numbers-management-service)",
			longestName: 41,
			want:        ServiceNameWidthLong,
		},
		{
			name:        "exactly at long bucket boundary",
			longestName: 48,
			want:        ServiceNameWidthLong,
		},
		{
			name:        "just over long bucket - grows to fit longest name plus trailing gap",
			longestName: 49,
			want:        49 + ServiceNameTrailingGap,
		},
		{
			name:        "very long names grow past cap to remain distinguishable",
			longestName: 100,
			want:        100 + ServiceNameTrailingGap,
		},
		{
			name:        "9-cell Cyrillic name (display width, not byte length)",
			longestName: 9,
			want:        ServiceNameWidthShort,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := PreferredNameTextWidth(tt.longestName)
			assert.Equal(t, tt.want, result)
		})
	}
}

func Test_RenderPanelLines(t *testing.T) {
	tests := []struct {
		name string
		opts PanelOptions
	}{
		{
			name: "renders basic panel",
			opts: PanelOptions{
				Title:   "Test",
				Content: "Content",
				Status:  "Info",
				Version: "v1.0",
				Height:  10,
				Width:   40,
			},
		},
		{
			name: "handles minimum dimensions",
			opts: PanelOptions{
				Title:   "T",
				Content: "C",
				Height:  2,
				Width:   10,
			},
		},
		{
			name: "handles empty content",
			opts: PanelOptions{
				Title:  "Title",
				Height: 5,
				Width:  20,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := strings.Join(RenderPanelLines(tt.opts), "\n")

			assert.Contains(t, result, tt.opts.Title)
		})
	}
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
		{
			name:   "wide rune wider than the room keeps only the ellipsis",
			input:  "日本語",
			width:  2,
			expect: "… ",
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
	tests := []struct {
		name   string
		input  string
		width  int
		expect string
	}{
		{
			name:   "negative width returns ellipsis",
			input:  "hello",
			width:  -1,
			expect: "…",
		},
		{
			name:   "single character string exact width",
			input:  "a",
			width:  1,
			expect: "a",
		},
		{
			name:   "all spaces",
			input:  "   ",
			width:  5,
			expect: "     ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TruncateAndPad(tt.input, tt.width)
			assert.Equal(t, tt.expect, result)
		})
	}
}

func Test_BuildTopBorder(t *testing.T) {
	border := func(s string) string { return s }

	tests := []struct {
		name     string
		title    string
		right    string
		width    int
		contains []string
	}{
		{
			name:     "with title and right text",
			title:    "Title",
			right:    "Info",
			width:    40,
			contains: []string{"Title", "Info", BorderTopLeft, BorderTopRight},
		},
		{
			name:     "with empty right text",
			title:    "logs",
			right:    "",
			width:    40,
			contains: []string{"logs", BorderTopLeft, BorderTopRight},
		},
		{
			name:     "with empty title",
			title:    "",
			right:    "",
			width:    20,
			contains: []string{BorderTopLeft, BorderTopRight},
		},
		{
			name:     "with empty title and right text",
			title:    "",
			right:    "ok",
			width:    20,
			contains: []string{BorderTopLeft, BorderTopRight, "ok"},
		},
		{
			name:     "with empty title and overflowing right text",
			title:    "",
			right:    "very-long-status-text-that-overflows",
			width:    20,
			contains: []string{BorderTopLeft, BorderTopRight, "…"},
		},
		{
			name:     "truncates title and right when both overflow",
			title:    "profile • default",
			right:    "starting… 0/0 ready",
			width:    38,
			contains: []string{BorderTopLeft, BorderTopRight, "…"},
		},
		{
			name:     "truncates only right when title fits",
			title:    "p",
			right:    "very-long-status-text-that-overflows",
			width:    20,
			contains: []string{BorderTopLeft, BorderTopRight, "p", "…"},
		},
		{
			name:     "truncates only title when right fits",
			title:    "very-long-title-text-that-overflows",
			right:    "ok",
			width:    20,
			contains: []string{BorderTopLeft, BorderTopRight, "ok", "…"},
		},
		{
			name:     "truncates the title when there is no right text",
			title:    "very-long-title-text-that-overflows",
			right:    "",
			width:    20,
			contains: []string{BorderTopLeft, BorderTopRight, "very-long", "…"},
		},
		{
			name:     "drops the title when the width leaves no room",
			title:    "logs",
			right:    "",
			width:    5,
			contains: []string{BorderTopLeft + strings.Repeat(BorderHorizontal, 5) + BorderTopRight},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := BuildTopBorder(border, tt.title, tt.right, tt.width)
			assert.Equal(t, tt.width+2, lipgloss.Width(result), "rendered width must equal innerWidth plus two border corners")

			for _, s := range tt.contains {
				assert.Contains(t, result, s)
			}
		})
	}
}

func Test_BuildBottomBorder(t *testing.T) {
	border := func(s string) string { return s }

	tests := []struct {
		name     string
		info     string
		version  string
		width    int
		contains []string
	}{
		{
			name:     "with version only",
			info:     "",
			version:  "v1.0",
			width:    40,
			contains: []string{BorderBottomLeft, BorderBottomRight},
		},
		{
			name:     "with info and version",
			info:     "cpu 0.5% mem 12MB",
			version:  "v1.0",
			width:    60,
			contains: []string{BorderBottomLeft, BorderBottomRight, "cpu 0.5% mem 12MB", "v1.0"},
		},
		{
			name:     "handles minimum width",
			info:     "",
			version:  "very-long-version-text",
			width:    10,
			contains: []string{BorderBottomLeft, "…"},
		},
		{
			name:    "handles empty text",
			info:    "",
			version: "",
			width:   20,
		},
		{
			name:     "with info only",
			info:     "cpu 0.5%",
			version:  "",
			width:    40,
			contains: []string{BorderBottomLeft, BorderBottomRight, "cpu 0.5%"},
		},
		{
			name:     "truncates info and version when both overflow",
			info:     "/ filter • cpu 0.5 • mem 12MB",
			version:  "v1.2.3 - ↑ 1.2.4",
			width:    38,
			contains: []string{BorderBottomLeft, BorderBottomRight, "…"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := BuildBottomBorder(border, tt.info, tt.version, tt.width)
			assert.Equal(t, tt.width+2, lipgloss.Width(result), "rendered width must equal innerWidth plus two border corners")

			for _, s := range tt.contains {
				assert.Contains(t, result, s)
			}
		})
	}
}

func Test_SplitAndPadContent(t *testing.T) {
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
			result := SplitAndPadContent(tt.content, tt.height)
			assert.Equal(t, tt.expect, result)
		})
	}
}

func Test_AppendContentLines(t *testing.T) {
	border := func(s string) string { return "[" + s + "]" }

	tests := []struct {
		name         string
		lines        []string
		contentLines []string
		innerWidth   int
		expected     []string
	}{
		{
			name:         "appends bordered lines",
			lines:        []string{"header"},
			contentLines: []string{"content1", "content2"},
			innerWidth:   10,
			expected:     []string{"header", "[│]content1  [│]", "[│]content2  [│]"},
		},
		{
			name:         "handles empty content lines",
			lines:        []string{},
			contentLines: []string{""},
			innerWidth:   5,
			expected:     []string{"[│]     [│]"},
		},
		{
			name:         "handles negative padding",
			lines:        []string{},
			contentLines: []string{"very long content line"},
			innerWidth:   5,
			expected:     []string{"[│]very long content line[│]"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := AppendContentLines(tt.lines, tt.contentLines, tt.innerWidth, border)

			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_AppendPrePaddedContentLines(t *testing.T) {
	border := func(s string) string { return "[" + s + "]" }

	tests := []struct {
		name         string
		lines        []string
		contentLines []string
		expected     []string
	}{
		{
			name:         "wraps each line in the vertical border without padding",
			lines:        []string{"header"},
			contentLines: []string{"ab", "cd  "},
			expected:     []string{"header", "[│]ab[│]", "[│]cd  [│]"},
		},
		{
			name:         "no content leaves the lines unchanged",
			lines:        []string{"header"},
			contentLines: nil,
			expected:     []string{"header"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := AppendPrePaddedContentLines(tt.lines, tt.contentLines, border)

			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_RenderFooter(t *testing.T) {
	tests := []struct {
		name     string
		help     string
		tips     string
		width    int
		expected string
	}{
		{
			name:     "help only when tips empty",
			help:     "Help",
			tips:     "",
			width:    80,
			expected: strings.Repeat(" ", 5) + "\n Help",
		},
		{
			name:     "both help and tips",
			help:     "Help",
			tips:     "Tip",
			width:    80,
			expected: strings.Repeat(" ", 80) + "\n Help" + strings.Repeat(" ", 71) + "Tip ",
		},
		{
			name:     "narrow width hides tips",
			help:     "Help text here",
			tips:     "Long tip",
			width:    10,
			expected: strings.Repeat(" ", 15) + "\n Help text here",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RenderFooter(tt.help, tt.tips, tt.width)

			assert.Equal(t, tt.expected, ansi.Strip(result))
		})
	}
}
