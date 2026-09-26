package logging

import (
	"context"
	"io"
	"log/slog"

	"github.com/rs/zerolog"
)

// Handler is a slog.Handler that writes one zerolog JSON line per record
type Handler struct {
	log   zerolog.Logger
	level zerolog.Level
	attrs []slog.Attr
	group string
}

// NewHandler creates a handler that writes to out at the configured level
func NewHandler(opts Options, out io.Writer) *Handler {
	return &Handler{
		log:   zerolog.New(out).With().Timestamp().Str("version", opts.Version).Logger(),
		level: parseLevel(opts.Level),
	}
}

// Enabled reports whether records at level reach the output
func (h *Handler) Enabled(_ context.Context, level slog.Level) bool {
	return toZerolog(level) >= h.level
}

// Handle writes the record as one event carrying the handler's attributes (already qualified) before the record's own
func (h *Handler) Handle(_ context.Context, record slog.Record) error {
	event := h.log.WithLevel(toZerolog(record.Level))

	for _, attr := range h.attrs {
		addAttr(event, "", attr)
	}

	record.Attrs(func(attr slog.Attr) bool {
		addAttr(event, h.group, attr)

		return true
	})

	event.Msg(record.Message)

	return nil
}

// WithAttrs returns a handler that adds attrs to every record, qualified by the groups opened before them
func (h *Handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	next := *h
	next.attrs = append([]slog.Attr(nil), h.attrs...)

	for _, attr := range attrs {
		next.attrs = append(next.attrs, slog.Attr{Key: h.group + attr.Key, Value: attr.Value})
	}

	return &next
}

// WithGroup returns a handler that prefixes the keys of later attributes with name
func (h *Handler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}

	next := *h
	next.group = h.group + name + "."

	return &next
}

// addAttr appends one attribute to the event under its prefixed key
func addAttr(event *zerolog.Event, group string, attr slog.Attr) {
	key := group + attr.Key
	value := attr.Value

	switch value.Kind() {
	case slog.KindString:
		event.Str(key, value.String())
	case slog.KindBool:
		event.Bool(key, value.Bool())
	case slog.KindInt64:
		event.Int64(key, value.Int64())
	case slog.KindAny:
		if err, ok := value.Any().(error); ok {
			event.AnErr(key, err)

			return
		}

		event.Interface(key, value.Any())
	default:
		event.Interface(key, value.Any())
	}
}

// toZerolog maps a slog level onto the zerolog level of the same name
func toZerolog(level slog.Level) zerolog.Level {
	switch level {
	case slog.LevelDebug:
		return zerolog.DebugLevel
	case slog.LevelInfo:
		return zerolog.InfoLevel
	case slog.LevelWarn:
		return zerolog.WarnLevel
	case slog.LevelError:
		return zerolog.ErrorLevel
	default:
		return zerolog.InfoLevel
	}
}

// parseLevel converts a configured level name to a zerolog level (info for an unknown name)
func parseLevel(level string) zerolog.Level {
	switch level {
	case LevelTrace:
		return zerolog.TraceLevel
	case LevelDebug:
		return zerolog.DebugLevel
	case LevelInfo:
		return zerolog.InfoLevel
	case LevelWarn:
		return zerolog.WarnLevel
	case LevelError:
		return zerolog.ErrorLevel
	case LevelFatal:
		return zerolog.FatalLevel
	case LevelPanic:
		return zerolog.PanicLevel
	default:
		return zerolog.InfoLevel
	}
}
