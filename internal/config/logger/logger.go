//go:generate mockgen -source=logger.go -destination=logger_mock.go -package=logger
package logger

import (
	"io"
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/pkgerrors"

	"fuku/internal/config"
)

const (
	DebugLevel = "debug"
	InfoLevel  = "info"
	WarnLevel  = "warn"
	ErrorLevel = "error"
	FatalLevel = "fatal"
	PanicLevel = "panic"
	TraceLevel = "trace"

	ConsoleFormat = "console"
	JSONFormat    = "json"

	TimeFormat = "02.01.2006 15:04:05"
)

// Logger interface for application logging
type Logger interface {
	Debug() Event
	Info() Event
	Warn() Event
	Error() Event
}

type Event interface {
	Msg(msg string)
	Msgf(format string, v ...interface{})
	Str(key, value string) Event
	Int(key string, value int) Event
	Dur(key string, value time.Duration) Event
	Err(err error) Event
}

// zerologEvent wraps zerolog.Event to implement our Event interface
type zerologEvent struct {
	event *zerolog.Event
}

func (e *zerologEvent) Msg(msg string) {
	e.event.Msg(msg)
}

func (e *zerologEvent) Msgf(format string, v ...interface{}) {
	e.event.Msgf(format, v...)
}

func (e *zerologEvent) Str(key, value string) Event {
	return &zerologEvent{event: e.event.Str(key, value)}
}

func (e *zerologEvent) Int(key string, value int) Event {
	return &zerologEvent{event: e.event.Int(key, value)}
}

func (e *zerologEvent) Dur(key string, value time.Duration) Event {
	return &zerologEvent{event: e.event.Dur(key, value)}
}

func (e *zerologEvent) Err(err error) Event {
	return &zerologEvent{event: e.event.Err(err)}
}

// NoopEvent is a simple no-op implementation
type NoopEvent struct{}

func (n *NoopEvent) Msg(msg string)                            {}
func (n *NoopEvent) Msgf(format string, v ...interface{})      {}
func (n *NoopEvent) Str(key, value string) Event               { return n }
func (n *NoopEvent) Int(key string, value int) Event           { return n }
func (n *NoopEvent) Dur(key string, value time.Duration) Event { return n }
func (n *NoopEvent) Err(err error) Event                       { return n }

// AppLogger represents a logger implementation using zerolog
type AppLogger struct {
	log zerolog.Logger
}

// NewLogger creates a new logger instance
func NewLogger(cfg *config.Config) Logger {
	zerolog.ErrorStackMarshaler = pkgerrors.MarshalStack
	zerolog.TimeFieldFormat = time.RFC3339

	var level zerolog.Level
	var output io.Writer

	if cfg.Logging.Level == "" {
		cfg.Logging.Level = InfoLevel
	}

	if cfg.Logging.Format == "" {
		cfg.Logging.Format = ConsoleFormat
	}

	level = getLogLevel(cfg.Logging.Level)

	switch cfg.Logging.Format {
	case JSONFormat:
		output = os.Stdout
	case ConsoleFormat:
		output = zerolog.ConsoleWriter{
			Out:        os.Stdout,
			TimeFormat: TimeFormat,
		}
	default:
		output = zerolog.ConsoleWriter{
			Out:        os.Stdout,
			TimeFormat: TimeFormat,
		}
	}

	logger := zerolog.
		New(output).
		Level(level).
		With().
		Timestamp().
		Str("version", config.Version).
		Logger()

	return &AppLogger{log: logger}
}

// Debug returns a debug level Event for logging debug messages
func (l *AppLogger) Debug() Event {
	return &zerologEvent{event: l.log.Debug()}
}

// Info returns an info level Event for logging informational messages
func (l *AppLogger) Info() Event {
	return &zerologEvent{event: l.log.Info()}
}

// Warn returns a warn level Event for logging warning messages
func (l *AppLogger) Warn() Event {
	return &zerologEvent{event: l.log.Warn()}
}

// Error returns an error level Event for logging error messages
func (l *AppLogger) Error() Event {
	return &zerologEvent{event: l.log.Error()}
}

// getLogLevel converts string level to zerolog.Level
func getLogLevel(level string) zerolog.Level {
	switch level {
	case DebugLevel:
		return zerolog.DebugLevel
	case InfoLevel:
		return zerolog.InfoLevel
	case WarnLevel:
		return zerolog.WarnLevel
	case ErrorLevel:
		return zerolog.ErrorLevel
	case FatalLevel:
		return zerolog.FatalLevel
	case PanicLevel:
		return zerolog.PanicLevel
	case TraceLevel:
		return zerolog.TraceLevel
	default:
		return zerolog.InfoLevel
	}
}
