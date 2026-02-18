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
	"github.com/charmbracelet/x/term"

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

// SetEnabled enables/disables log output
func (f *LogFormatter) SetEnabled(enabled bool) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.enabled = enabled
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

// WriteFormatted writes a formatted message to the given writer
func (f *LogFormatter) WriteFormatted(w io.Writer, service, message string) {
	line := f.FormatMessage(service, message)
	fmt.Fprint(w, line)
}

// RenderBanner writes a connection banner to the given writer
func (f *LogFormatter) RenderBanner(w io.Writer, status StatusMessage, subscribed []string) {
	serviceCount := fmt.Sprintf("%d running", len(status.Services))

	showing := "all"

	if len(subscribed) > 0 {
		const maxShown = 5
		if len(subscribed) <= maxShown {
			showing = strings.Join(subscribed, ", ")
		} else {
			showing = strings.Join(subscribed[:maxShown], ", ") + fmt.Sprintf(" and %d more", len(subscribed)-maxShown)
		}
	}

	termWidth, _, err := term.GetSize(os.Stdout.Fd())
	if err != nil || termWidth < 40 {
		termWidth = 80
	}

	innerWidth := termWidth - components.PanelInnerPadding
	border := func(s string) string { return components.PanelBorderStyle.Render(s) }

	muted := components.PanelMutedStyle.Render
	bold := components.BoldStyle.Render

	field := func(label, value string) string {
		return " " + muted(label) + " " + bold(value)
	}

	titleText := components.PanelTitleStyle.Render("logs")
	topBorder := components.BuildTopBorder(border, titleText, "", innerWidth)

	contentLines := []string{
		field("profile:", status.Profile),
		field("services:", serviceCount),
		field("showing:", showing),
	}

	lines := []string{topBorder}
	lines = components.AppendContentLines(lines, contentLines, innerWidth, border)

	bottomBorder := components.BuildBottomBorder(border, "", "v"+status.Version, innerWidth)
	lines = append(lines, bottomBorder)

	footer := " " + components.HelpKeyStyle.Render("ctrl+c") + " " + components.HelpDescStyle.Render("exit")
	lines = append(lines, footer, "")

	for _, line := range lines {
		fmt.Fprintln(w, line)
	}
}

// logEntry represents a parsed JSON log entry
type logEntry struct {
	Component string `json:"component"`
	Message   string `json:"message"`
	Service   string `json:"service"`
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

	message := entry.Message
	if entry.Component != "" {
		message = fmt.Sprintf("[%s] %s", entry.Component, message)
	}

	line := f.formatLine(serviceName, message)
	os.Stdout.Write([]byte(line))

	return len(p)
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
