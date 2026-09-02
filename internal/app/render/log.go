package render

import (
	"fmt"
	"io"
	"strings"
	"sync"

	"charm.land/lipgloss/v2"

	"fuku/internal/app/relay"
	"fuku/internal/app/ui/components"
	"fuku/internal/config/logger"
)

// Log handles service log line formatting and banner rendering
type Log struct {
	mu            sync.RWMutex
	maxServiceLen int
	theme         components.Theme
	serviceStyles map[string]lipgloss.Style
}

// NewLog creates a new Log renderer
func NewLog(isDark bool) *Log {
	theme := components.NewTheme(isDark)

	return &Log{
		maxServiceLen: components.LogStreamMaxServiceNameLen,
		theme:         theme,
		serviceStyles: make(map[string]lipgloss.Style),
	}
}

// FormatServiceLine formats a service log line for console output
func (l *Log) FormatServiceLine(service, message string) string {
	l.mu.Lock()
	defer l.mu.Unlock()

	return l.formatLine(service, message)
}

// WriteServiceLine writes a formatted service log line to the writer
func (l *Log) WriteServiceLine(w io.Writer, service, message string) {
	line := l.FormatServiceLine(service, message)
	fmt.Fprint(w, line)
}

// BannerOptions selects what the log connection banner reports
type BannerOptions struct {
	Width      int
	Status     relay.StatusMessage
	Subscribed []string
	Project    string // fingerprint of the local project, compared against the instance the socket serves
	Following  bool
}

// RenderBanner writes a connection banner to the given writer
func (l *Log) RenderBanner(w io.Writer, options BannerOptions) {
	status, subscribed := options.Status, options.Subscribed
	serviceCount := fmt.Sprintf("%d running", len(status.Services))

	const maxShown = 5

	var showing string

	switch {
	case len(subscribed) == 0:
		showing = "all"
	case len(subscribed) <= maxShown:
		showing = strings.Join(subscribed, ", ")
	default:
		showing = strings.Join(subscribed[:maxShown], ", ") + fmt.Sprintf(" and %d more", len(subscribed)-maxShown)
	}

	innerWidth := options.Width - components.PanelInnerPadding
	border := func(s string) string { return components.PanelBorderStyle.Render(s) }

	muted := l.theme.PanelMutedStyle.Render
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

	if servesAnotherProject(options) {
		warning := l.theme.StatusStartingStyle.Render("serves a different project")
		contentLines = append(contentLines, " "+muted("project:")+" "+warning)
	}

	lines := []string{topBorder}
	lines = components.AppendContentLines(lines, contentLines, innerWidth, border)

	versionText := l.theme.PanelMutedStyle.Render("v" + status.Version)
	bottomBorder := components.BuildBottomBorder(border, "", versionText, innerWidth)
	lines = append(lines, bottomBorder)

	if options.Following {
		footer := " " + l.theme.HelpKeyStyle.Render("ctrl+c") + " " + l.theme.HelpDescStyle.Render("exit")
		lines = append(lines, footer)
	}

	lines = append(lines, "")

	for _, line := range lines {
		fmt.Fprintln(w, line)
	}
}

// formatLine formats a single log line with colors
func (l *Log) formatLine(service, message string) string {
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

// getServiceStyle returns a consistent style for a service name
func (l *Log) getServiceStyle(service string) lipgloss.Style {
	if style, exists := l.serviceStyles[service]; exists {
		return style
	}

	colorIndex := hashString(service) % len(l.theme.ServiceColorPalette)
	c := l.theme.ServiceColorPalette[colorIndex]
	style := l.theme.NewLogsServiceNameStyle(c)
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

// Theme returns the theme used by this Log renderer
func (l *Log) Theme() components.Theme {
	return l.theme
}

// FormatJSON formats a service log line as JSON
func FormatJSON(service, message string) string {
	return fmt.Sprintf(`{"service":%q,"message":%q}`+"\n", service, message)
}

// FormatMessage formats a service log message based on format type
func (l *Log) FormatMessage(format, service, message string) string {
	if format == logger.JSONFormat {
		return FormatJSON(service, message)
	}

	return l.FormatServiceLine(service, message)
}

// servesAnotherProject reports whether the instance behind the socket belongs to a different project
// (an instance that does not report a project is treated as a match because it cannot be compared)
func servesAnotherProject(options BannerOptions) bool {
	if options.Project == "" || options.Status.Project == "" {
		return false
	}

	return options.Project != options.Status.Project
}
