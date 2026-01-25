package logger

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/pkgerrors"

	"fuku/internal/config"
)

// Logger configuration constants
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
	Debug() *zerolog.Event
	Info() *zerolog.Event
	Warn() *zerolog.Event
	Error() *zerolog.Event
	WithComponent(name string) Logger
}

// AppLogger represents a logger implementation using zerolog
type AppLogger struct {
	log zerolog.Logger
}

// NewLogger creates a new logger instance
func NewLogger(cfg *config.Config) Logger {
	return NewLoggerWithOutput(cfg, nil)
}

// NewLoggerWithOutput creates a new logger instance with a custom output writer
func NewLoggerWithOutput(cfg *config.Config, customOutput io.Writer) Logger {
	zerolog.ErrorStackMarshaler = pkgerrors.MarshalStack
	zerolog.TimeFieldFormat = time.RFC3339

	var (
		level  zerolog.Level
		output io.Writer
	)

	if cfg.Logging.Level == "" {
		cfg.Logging.Level = InfoLevel
	}

	if cfg.Logging.Format == "" {
		cfg.Logging.Format = ConsoleFormat
	}

	level = getLogLevel(cfg.Logging.Level)

	if customOutput != nil {
		output = customOutput
	} else {
		switch cfg.Logging.Format {
		case JSONFormat:
			output = os.Stdout
		case ConsoleFormat:
			output = newConsoleWriter()
		default:
			output = newConsoleWriter()
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
func (l *AppLogger) Debug() *zerolog.Event {
	return l.log.Debug()
}

// Info returns an info level Event for logging informational messages
func (l *AppLogger) Info() *zerolog.Event {
	return l.log.Info()
}

// Warn returns a warn level Event for logging warning messages
func (l *AppLogger) Warn() *zerolog.Event {
	return l.log.Warn()
}

// Error returns an error level Event for logging error messages
func (l *AppLogger) Error() *zerolog.Event {
	return l.log.Error()
}

// WithComponent creates a new logger with a component name for contextual logging
func (l *AppLogger) WithComponent(name string) Logger {
	return &AppLogger{
		log: l.log.With().Str("component", name).Logger(),
	}
}

// newConsoleWriter creates a console writer with component formatting
func newConsoleWriter() zerolog.ConsoleWriter {
	return zerolog.ConsoleWriter{
		Out:        os.Stdout,
		TimeFormat: TimeFormat,
		FormatFieldName: func(i interface{}) string {
			if s, ok := i.(string); ok && s == "component" {
				return ""
			}

			return fmt.Sprintf("%s=", i)
		},
		FormatPrepare: func(m map[string]interface{}) error {
			if component, ok := m["component"].(string); ok {
				m["component"] = fmt.Sprintf("[%s]", component)
			}

			return nil
		},
		PartsOrder: []string{
			zerolog.TimestampFieldName,
			zerolog.LevelFieldName,
			"component",
			zerolog.CallerFieldName,
			zerolog.MessageFieldName,
		},
	}
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
