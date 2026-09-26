package logging

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"regexp"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_NewHandler(t *testing.T) {
	var buf bytes.Buffer

	timeField := regexp.MustCompile(`"time":"[^"]+"`)
	log := slog.New(NewHandler(Options{Level: LevelDebug, Version: "1.2.3"}, &buf))

	tests := []struct {
		name   string
		group  string
		with   []any
		msg    string
		args   []any
		expect string
	}{
		{
			name:   "message only",
			msg:    "All services stopped",
			expect: `{"level":"info","version":"1.2.3","time":"<t>","message":"All services stopped"}` + "\n",
		},
		{
			name:   "attributes from With precede the record attributes",
			with:   []any{"component", "RUNNER"},
			msg:    "slow",
			args:   []any{"service", "api"},
			expect: `{"level":"info","version":"1.2.3","component":"RUNNER","service":"api","time":"<t>","message":"slow"}` + "\n",
		},
		{
			name:   "error attribute renders its text",
			msg:    "failed",
			args:   []any{"error", errors.New("boom")},
			expect: `{"level":"info","version":"1.2.3","error":"boom","time":"<t>","message":"failed"}` + "\n",
		},
		{
			name:   "bool, int and string attributes",
			msg:    "detected",
			args:   []any{"isDark", true, "pid", 42, "color", "#000"},
			expect: `{"level":"info","version":"1.2.3","isDark":true,"pid":42,"color":"#000","time":"<t>","message":"detected"}` + "\n",
		},
		{
			name:   "other kinds marshal as values",
			msg:    "sized",
			args:   []any{"ratio", 0.5, "files", []string{"a", "b"}},
			expect: `{"level":"info","version":"1.2.3","ratio":0.5,"files":["a","b"],"time":"<t>","message":"sized"}` + "\n",
		},
		{
			name:   "group prefixes the keys that follow it",
			group:  "req",
			with:   []any{"id", "1"},
			msg:    "done",
			args:   []any{"path", "/"},
			expect: `{"level":"info","version":"1.2.3","req.id":"1","req.path":"/","time":"<t>","message":"done"}` + "\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf.Reset()

			log.WithGroup(tt.group).With(tt.with...).Info(tt.msg, tt.args...)

			assert.Equal(t, tt.expect, timeField.ReplaceAllString(buf.String(), `"time":"<t>"`))
		})
	}
}

func Test_NewHandler_AttrsBeforeGroup(t *testing.T) {
	var buf bytes.Buffer

	timeField := regexp.MustCompile(`"time":"[^"]+"`)
	log := slog.New(NewHandler(Options{Level: LevelDebug, Version: "1.2.3"}, &buf))
	expected := `{"level":"info","version":"1.2.3","component":"APP","req.id":"1","req.path":"/","time":"<t>","message":"done"}` + "\n"

	log.With("component", "APP").WithGroup("req").With("id", "1").Info("done", "path", "/")

	assert.Equal(t, expected, timeField.ReplaceAllString(buf.String(), `"time":"<t>"`))
}

func Test_Handler_WithGroup_EmptyName(t *testing.T) {
	var buf bytes.Buffer

	handler := NewHandler(Options{Level: LevelInfo}, &buf)

	result := handler.WithGroup("")

	assert.Same(t, handler, result)
}

func Test_NewHandler_Level(t *testing.T) {
	tests := []struct {
		name   string
		level  string
		expect []string
	}{
		{
			name:   "trace",
			level:  LevelTrace,
			expect: []string{"debug", "info", "warn", "error"},
		},
		{
			name:   "debug",
			level:  LevelDebug,
			expect: []string{"debug", "info", "warn", "error"},
		},
		{
			name:   "info",
			level:  LevelInfo,
			expect: []string{"info", "warn", "error"},
		},
		{
			name:   "warn",
			level:  LevelWarn,
			expect: []string{"warn", "error"},
		},
		{
			name:   "error",
			level:  LevelError,
			expect: []string{"error"},
		},
		{
			name:   "fatal",
			level:  LevelFatal,
			expect: []string{},
		},
		{
			name:   "panic",
			level:  LevelPanic,
			expect: []string{},
		},
		{
			name:   "empty defaults to info",
			level:  "",
			expect: []string{"info", "warn", "error"},
		},
		{
			name:   "unknown defaults to info",
			level:  "unknown",
			expect: []string{"info", "warn", "error"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer

			log := slog.New(NewHandler(Options{Level: tt.level}, &buf))

			log.Debug("debug")
			log.Info("info")
			log.Warn("warn")
			log.Error("error")

			written := make([]string, 0, len(tt.expect))

			for line := range strings.FieldsSeq(buf.String()) {
				var entry struct {
					Level string `json:"level"`
				}

				require.NoError(t, json.Unmarshal([]byte(line), &entry))

				written = append(written, entry.Level)
			}

			assert.Equal(t, tt.expect, written)
		})
	}
}

func Test_toZerolog(t *testing.T) {
	tests := []struct {
		name   string
		level  slog.Level
		expect zerolog.Level
	}{
		{
			name:   "debug",
			level:  slog.LevelDebug,
			expect: zerolog.DebugLevel,
		},
		{
			name:   "info",
			level:  slog.LevelInfo,
			expect: zerolog.InfoLevel,
		},
		{
			name:   "warn",
			level:  slog.LevelWarn,
			expect: zerolog.WarnLevel,
		},
		{
			name:   "error",
			level:  slog.LevelError,
			expect: zerolog.ErrorLevel,
		},
		{
			name:   "custom level defaults to info",
			level:  slog.LevelInfo + 1,
			expect: zerolog.InfoLevel,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expect, toZerolog(tt.level))
		})
	}
}

func Test_parseLevel(t *testing.T) {
	tests := []struct {
		name   string
		level  string
		expect zerolog.Level
	}{
		{
			name:   "trace",
			level:  LevelTrace,
			expect: zerolog.TraceLevel,
		},
		{
			name:   "debug",
			level:  LevelDebug,
			expect: zerolog.DebugLevel,
		},
		{
			name:   "info",
			level:  LevelInfo,
			expect: zerolog.InfoLevel,
		},
		{
			name:   "warn",
			level:  LevelWarn,
			expect: zerolog.WarnLevel,
		},
		{
			name:   "error",
			level:  LevelError,
			expect: zerolog.ErrorLevel,
		},
		{
			name:   "fatal",
			level:  LevelFatal,
			expect: zerolog.FatalLevel,
		},
		{
			name:   "panic",
			level:  LevelPanic,
			expect: zerolog.PanicLevel,
		},
		{
			name:   "unknown",
			level:  "unknown",
			expect: zerolog.InfoLevel,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expect, parseLevel(tt.level))
		})
	}
}
