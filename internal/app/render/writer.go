package render

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sync"

	"fuku/internal/config"
	"fuku/internal/config/logger"
)

// Writer implements io.Writer for the main application logger output
type Writer struct {
	mu      sync.RWMutex
	format  string
	enabled bool
	log     *Log
	out     io.Writer
}

// NewWriter creates a new Writer for application logger output
func NewWriter(cfg *config.Config, log *Log, out io.Writer) *Writer {
	return &Writer{
		format: cfg.Logging.Format,
		log:    log,
		out:    out,
	}
}

// Write implements io.Writer for logger output
func (w *Writer) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.enabled {
		return len(p), nil
	}

	if w.format == logger.JSONFormat {
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
		serviceName = config.AppName
	}

	message := entry.Message
	if entry.Component != "" {
		message = fmt.Sprintf("[%s] %s", entry.Component, message)
	}

	w.log.WriteServiceLine(w.out, serviceName, message)

	return len(p)
}
