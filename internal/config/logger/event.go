package logger

import (
	"bytes"

	"github.com/rs/zerolog"
)

// EventLogger creates zerolog loggers for formatting bus events as logfmt
type EventLogger interface {
	NewLogger(buf *bytes.Buffer) zerolog.Logger
}

type eventLogger struct{}

// NewEventLogger creates a new EventLogger
func NewEventLogger() EventLogger {
	return &eventLogger{}
}

// NewLogger creates a zerolog logger configured for logfmt output
func (e *eventLogger) NewLogger(buf *bytes.Buffer) zerolog.Logger {
	w := zerolog.ConsoleWriter{
		Out:        buf,
		NoColor:    true,
		PartsOrder: []string{zerolog.MessageFieldName},
	}

	return zerolog.New(w)
}
