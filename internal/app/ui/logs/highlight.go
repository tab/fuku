package logs

import (
	"regexp"
	"strings"

	"fuku/internal/app/ui/components"
)

type highlightPattern struct {
	pattern *regexp.Regexp
	style   func(s string) string
}

type Highlighter struct {
	patterns    []highlightPattern
	uuidPattern *regexp.Regexp
}

func newHighlighter() Highlighter {
	return Highlighter{
		patterns: []highlightPattern{
			{
				pattern: regexp.MustCompile(`(?i)\b(ERROR|FATAL|ERR)\b`),
				style:   func(s string) string { return components.LogLevelErrorStyle.Render(s) },
			},
			{
				pattern: regexp.MustCompile(`(?i)\b(WARNING|WARN)\b`),
				style:   func(s string) string { return components.LogLevelWarnStyle.Render(s) },
			},
			{
				pattern: regexp.MustCompile(`(?i)\b(INFO|INF)\b`),
				style:   func(s string) string { return components.LogLevelInfoStyle.Render(s) },
			},
			{
				pattern: regexp.MustCompile(`(?i)\b(DEBUG)\b`),
				style:   func(s string) string { return components.LogLevelDebugStyle.Render(s) },
			},
			{
				pattern: regexp.MustCompile(`\[?(ERROR|FATAL|ERR)\]?`),
				style:   func(s string) string { return components.LogLevelErrorStyle.Render(s) },
			},
			{
				pattern: regexp.MustCompile(`\[?(WARNING|WARN)\]?`),
				style:   func(s string) string { return components.LogLevelWarnStyle.Render(s) },
			},
			{
				pattern: regexp.MustCompile(`\[?(INFO|INF)\]?`),
				style:   func(s string) string { return components.LogLevelInfoStyle.Render(s) },
			},
			{
				pattern: regexp.MustCompile(`\[?(DEBUG)\]?`),
				style:   func(s string) string { return components.LogLevelDebugStyle.Render(s) },
			},
			{
				pattern: regexp.MustCompile(`level=(error|fatal)`),
				style:   func(s string) string { return components.LogLevelErrorStyle.Render(s) },
			},
			{
				pattern: regexp.MustCompile(`level=(warning|warn)`),
				style:   func(s string) string { return components.LogLevelWarnStyle.Render(s) },
			},
			{
				pattern: regexp.MustCompile(`level=info`),
				style:   func(s string) string { return components.LogLevelInfoStyle.Render(s) },
			},
			{
				pattern: regexp.MustCompile(`level=debug`),
				style:   func(s string) string { return components.LogLevelDebugStyle.Render(s) },
			},
		},
		uuidPattern: regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`),
	}
}

var defaultHighlighter = newHighlighter()

func (h Highlighter) highlight(message string) string {
	result := message

	for _, p := range h.patterns {
		result = p.pattern.ReplaceAllStringFunc(result, func(match string) string {
			return p.style(strings.ToUpper(match))
		})
	}

	result = h.uuidPattern.ReplaceAllStringFunc(result, func(match string) string {
		return components.UUIDStyle.Render(match)
	})

	return result
}

// highlightLogLevel applies color to log level keywords in the message
func highlightLogLevel(message string) string {
	return defaultHighlighter.highlight(message)
}
