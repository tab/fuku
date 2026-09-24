package terminal

import (
	"fmt"
	"strings"
	"sync"

	"charm.land/lipgloss/v2"

	"fuku/internal/platform/logging"
)

// Log formats service log lines, aligning the service names it has seen so far
type Log struct {
	mu            sync.Mutex
	format        string
	maxServiceLen int
	theme         Theme
	serviceStyles map[string]lipgloss.Style
}

// NewLog creates the log line formatter
func NewLog(options Options, theme Theme) *Log {
	return &Log{
		format:        options.Format,
		maxServiceLen: LogStreamMaxServiceNameLen,
		theme:         theme,
		serviceStyles: make(map[string]lipgloss.Style),
	}
}

// FormatServiceLine formats a styled service log line for console output
func (l *Log) FormatServiceLine(service, message string) string {
	l.mu.Lock()
	defer l.mu.Unlock()

	style := l.getServiceStyle(service)

	if len(service) > l.maxServiceLen {
		l.maxServiceLen = len(service)
	}

	padding := l.maxServiceLen - len(service)
	paddedName := service + strings.Repeat(" ", padding)

	return style.Render(paddedName) + " " +
		l.theme.LogsSeparatorStyle.Render("|") + " " +
		message + "\n"
}

// FormatMessage formats a service log line in the configured format (JSON or the styled console line)
func (l *Log) FormatMessage(service, message string) string {
	if l.format == logging.FormatJSON {
		return FormatJSON(service, message)
	}

	return l.FormatServiceLine(service, message)
}

// getServiceStyle returns a consistent style for a service name
func (l *Log) getServiceStyle(service string) lipgloss.Style {
	if style, exists := l.serviceStyles[service]; exists {
		return style
	}

	colorIndex := hashString(service) % len(l.theme.ServiceColorPalette)
	c := l.theme.ServiceColorPalette[colorIndex]
	style := l.theme.newLogsServiceNameStyle(c)
	l.serviceStyles[service] = style

	return style
}

// hashString returns a non-negative hash of a string
func hashString(s string) int {
	var h uint64

	for _, c := range s {
		//nolint:gosec // rune values are always non-negative, safe to widen
		h = 31*h + uint64(c)
	}

	return int(h & 0x7fffffffffffffff)
}

// FormatJSON formats a service log line as JSON
func FormatJSON(service, message string) string {
	return fmt.Sprintf(`{"service":%q,"message":%q}`+"\n", service, message)
}
