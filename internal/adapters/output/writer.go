package output

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sync"

	"fuku/internal/platform/buildinfo"
	"fuku/internal/platform/logging"
)

// Formatter renders a service log line the way the terminal shows it
type Formatter interface {
	FormatServiceLine(service, message string) string
}

// Writer implements io.Writer for the main application logger output
type Writer struct {
	format    string
	formatter Formatter
	out       io.Writer
	mu        sync.Mutex
	enabled   bool
}

// NewWriter creates a new Writer for application logger output
func NewWriter(options Options, formatter Formatter, out io.Writer) *Writer {
	return &Writer{
		format:    options.Format,
		formatter: formatter,
		out:       out,
	}
}

// Write implements io.Writer for logger output
func (w *Writer) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.enabled {
		return len(p), nil
	}

	if w.format == logging.FormatJSON {
		return w.writeJSON(p), nil
	}

	return w.writeConsole(p), nil
}

// SetEnabled enables/disables log output
func (w *Writer) SetEnabled(enabled bool) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.enabled = enabled
}

// logEntry represents a parsed JSON log entry
type logEntry struct {
	Component string `json:"component"`
	Message   string `json:"message"`
	Service   string `json:"service"`
}

// writeJSON outputs raw JSON
func (w *Writer) writeJSON(p []byte) int {
	//nolint:errcheck // best-effort write to output
	w.out.Write(p)

	return len(p)
}

// writeConsole formats JSON as colored console output
func (w *Writer) writeConsole(p []byte) int {
	var entry logEntry
	if err := json.Unmarshal(bytes.TrimSpace(p), &entry); err != nil {
		//nolint:errcheck // best-effort write to output
		w.out.Write(p)

		return len(p)
	}

	serviceName := entry.Service
	if serviceName == "" {
		serviceName = buildinfo.AppName
	}

	message := entry.Message
	if entry.Component != "" {
		message = fmt.Sprintf("[%s] %s", entry.Component, message)
	}

	//nolint:errcheck // best-effort write to output
	io.WriteString(w.out, w.formatter.FormatServiceLine(serviceName, message))

	return len(p)
}
