package logs

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/charmbracelet/lipgloss"

	"fuku/internal/app/ui/components"
	"fuku/internal/config"
	"fuku/internal/config/logger"
)

// LogFormatter formats log output based on configuration
type LogFormatter struct {
	mu             sync.RWMutex
	format         string
	enabled        bool
	maxServiceLen  int
	separatorStyle lipgloss.Style
	messageStyle   lipgloss.Style
	serviceStyles  map[string]lipgloss.Style
}

// NewLogFormatter creates a new LogFormatter
func NewLogFormatter(cfg *config.Config) *LogFormatter {
	return &LogFormatter{
		format:         cfg.Logging.Format,
		enabled:        false,
		maxServiceLen:  components.DefaultMaxServiceLen,
		separatorStyle: lipgloss.NewStyle().Foreground(components.LogSeparatorColor),
		messageStyle:   lipgloss.NewStyle(),
		serviceStyles:  make(map[string]lipgloss.Style),
	}
}

// logEntry represents a parsed JSON log entry
type logEntry struct {
	Message string `json:"message"`
	Service string `json:"service"`
}

// Write implements io.Writer for logger output
func (f *LogFormatter) Write(p []byte) (n int, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if !f.enabled {
		return len(p), nil
	}

	if f.format == logger.JSONFormat {
		return f.writeJSON(p), nil
	}

	return f.writeConsole(p), nil
}

// writeJSON outputs raw JSON
func (f *LogFormatter) writeJSON(p []byte) int {
	os.Stdout.Write(p)

	return len(p)
}

// writeConsole formats JSON as colored console output
func (f *LogFormatter) writeConsole(p []byte) int {
	var entry logEntry
	if err := json.Unmarshal(bytes.TrimSpace(p), &entry); err != nil {
		os.Stdout.Write(p)

		return len(p)
	}

	serviceName := entry.Service
	if serviceName == "" {
		serviceName = config.AppName
	}

	line := f.formatLine(serviceName, entry.Message)
	os.Stdout.Write([]byte(line))

	return len(p)
}

// FormatMessage formats a service log message (used by logs client)
func (f *LogFormatter) FormatMessage(service, message string) string {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.format == logger.JSONFormat {
		data, err := json.Marshal(map[string]string{
			"service": service,
			"message": message,
		})
		if err != nil {
			return fmt.Sprintf(`{"service":%q,"message":%q}`+"\n", service, message)
		}

		return string(data) + "\n"
	}

	return f.formatLine(service, message)
}

// formatLine formats a single log line with colors
func (f *LogFormatter) formatLine(service, message string) string {
	style := f.getServiceStyle(service)

	if len(service) > f.maxServiceLen {
		f.maxServiceLen = len(service)
	}

	padding := f.maxServiceLen - len(service)
	paddedName := service + strings.Repeat(" ", padding)

	return style.Render(paddedName) + " " +
		f.separatorStyle.Render("|") + " " +
		f.messageStyle.Render(message) + "\n"
}

// getServiceStyle returns a consistent style for a service name
func (f *LogFormatter) getServiceStyle(service string) lipgloss.Style {
	if style, exists := f.serviceStyles[service]; exists {
		return style
	}

	colorIndex := hashString(service) % len(components.ServiceColorPalette)
	color := components.ServiceColorPalette[colorIndex]
	style := lipgloss.NewStyle().Foreground(color).Bold(true)
	f.serviceStyles[service] = style

	return style
}

// hashString returns a simple hash of a string
func hashString(s string) int {
	h := 0
	for _, c := range s {
		h = 31*h + int(c)
	}

	if h < 0 {
		h = -h
	}

	return h
}

// SetEnabled enables/disables log output
func (f *LogFormatter) SetEnabled(enabled bool) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.enabled = enabled
}

// WriteFormatted writes a formatted message to the given writer
func (f *LogFormatter) WriteFormatted(w io.Writer, service, message string) {
	line := f.FormatMessage(service, message)
	fmt.Fprint(w, line)
}
