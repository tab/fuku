package logs

import (
	"regexp"
	"strings"

	"fuku/internal/app/ui/components"
)

type Highlighter struct {
	levelPattern *regexp.Regexp
	uuidPattern  *regexp.Regexp
	styleMap     map[string]func(s string) string
}

func newHighlighter() Highlighter {
	return Highlighter{
		levelPattern: regexp.MustCompile(`(?i)(\b(?:ERROR|FATAL|ERR|WARNING|WARN|INFO|INF|DEBUG)\b|\[?(?:ERROR|FATAL|ERR|WARNING|WARN|INFO|INF|DEBUG)\]?|level=(?:error|fatal|warning|warn|info|debug))`),
		uuidPattern:  regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`),
		styleMap: map[string]func(s string) string{
			"error":   func(s string) string { return components.LogLevelErrorStyle.Render(s) },
			"err":     func(s string) string { return components.LogLevelErrorStyle.Render(s) },
			"warning": func(s string) string { return components.LogLevelWarnStyle.Render(s) },
			"warn":    func(s string) string { return components.LogLevelWarnStyle.Render(s) },
			"info":    func(s string) string { return components.LogLevelInfoStyle.Render(s) },
			"inf":     func(s string) string { return components.LogLevelInfoStyle.Render(s) },
			"debug":   func(s string) string { return components.LogLevelDebugStyle.Render(s) },
			"fatal":   func(s string) string { return components.LogLevelErrorStyle.Render(s) },
		},
	}
}

var defaultHighlighter = newHighlighter()

func (h Highlighter) highlight(message string) string {
	if !containsLevelKeyword(message) && !strings.Contains(message, "-") {
		return message
	}

	result := message

	if containsLevelKeyword(message) {
		result = h.levelPattern.ReplaceAllStringFunc(result, func(match string) string {
			normalized := normalizeLevel(match)
			if style, ok := h.styleMap[normalized]; ok {
				var upper string
				if strings.HasPrefix(strings.ToLower(match), "level=") {
					upper = "level=" + strings.ToUpper(strings.TrimPrefix(strings.ToLower(match), "level="))
				} else {
					upper = strings.ToUpper(match)
				}

				return style(upper)
			}

			return match
		})
	}

	if strings.Contains(result, "-") {
		result = h.uuidPattern.ReplaceAllStringFunc(result, func(match string) string {
			return components.UUIDStyle.Render(match)
		})
	}

	return result
}

func containsLevelKeyword(s string) bool {
	lower := strings.ToLower(s)

	return strings.Contains(lower, "error") ||
		strings.Contains(lower, "err") ||
		strings.Contains(lower, "warn") ||
		strings.Contains(lower, "info") ||
		strings.Contains(lower, "inf") ||
		strings.Contains(lower, "debug") ||
		strings.Contains(lower, "fatal") ||
		strings.Contains(lower, "level=")
}

func normalizeLevel(match string) string {
	lower := strings.ToLower(match)

	if strings.HasPrefix(lower, "level=") {
		return strings.TrimPrefix(lower, "level=")
	}

	lower = strings.TrimPrefix(lower, "[")
	lower = strings.TrimSuffix(lower, "]")

	return lower
}

// highlightLogLevel applies color to log level keywords in the message
func highlightLogLevel(message string) string {
	return defaultHighlighter.highlight(message)
}
