package logger

import (
	"errors"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"

	"fuku/internal/config"
)

func Test_NewLogger(t *testing.T) {
	type result struct {
		level  zerolog.Level
		format string
	}

	tests := []struct {
		name     string
		cfg      *config.Config
		expected result
	}{
		{
			name: "Default",
			cfg: func() *config.Config {
				cfg := config.DefaultConfig()
				return cfg
			}(),
			expected: result{
				level:  zerolog.InfoLevel,
				format: ConsoleFormat,
			},
		},
		{
			name: "Debug level",
			cfg: func() *config.Config {
				cfg := config.DefaultConfig()
				cfg.Logging.Level = DebugLevel
				return cfg
			}(),
			expected: result{
				level:  zerolog.DebugLevel,
				format: ConsoleFormat,
			},
		},
		{
			name: "Warn level and json format",
			cfg: func() *config.Config {
				cfg := config.DefaultConfig()
				cfg.Logging.Level = WarnLevel
				cfg.Logging.Format = JSONFormat
				return cfg
			}(),
			expected: result{
				level:  zerolog.WarnLevel,
				format: JSONFormat,
			},
		},
		{
			name: "Empty level and format (defaults)",
			cfg: func() *config.Config {
				cfg := config.DefaultConfig()
				cfg.Logging.Level = ""
				cfg.Logging.Format = ""
				return cfg
			}(),
			expected: result{
				level:  zerolog.InfoLevel,
				format: ConsoleFormat,
			},
		},
		{
			name: "Error level",
			cfg: func() *config.Config {
				cfg := config.DefaultConfig()
				cfg.Logging.Level = ErrorLevel
				return cfg
			}(),
			expected: result{
				level:  zerolog.ErrorLevel,
				format: ConsoleFormat,
			},
		},
		{
			name: "Fatal level",
			cfg: func() *config.Config {
				cfg := config.DefaultConfig()
				cfg.Logging.Level = FatalLevel
				return cfg
			}(),
			expected: result{
				level:  zerolog.FatalLevel,
				format: ConsoleFormat,
			},
		},
		{
			name: "Panic level",
			cfg: func() *config.Config {
				cfg := config.DefaultConfig()
				cfg.Logging.Level = PanicLevel
				return cfg
			}(),
			expected: result{
				level:  zerolog.PanicLevel,
				format: ConsoleFormat,
			},
		},
		{
			name: "Trace level",
			cfg: func() *config.Config {
				cfg := config.DefaultConfig()
				cfg.Logging.Level = TraceLevel
				return cfg
			}(),
			expected: result{
				level:  zerolog.TraceLevel,
				format: ConsoleFormat,
			},
		},
		{
			name: "Unknown format (defaults to console)",
			cfg: func() *config.Config {
				cfg := config.DefaultConfig()
				cfg.Logging.Format = "unknown"
				return cfg
			}(),
			expected: result{
				level:  zerolog.InfoLevel,
				format: ConsoleFormat,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := NewLogger(tt.cfg)
			assert.NotNil(t, logger)

			appLogger, ok := logger.(*AppLogger)
			assert.True(t, ok)

			assert.Equal(t, tt.expected.level, appLogger.log.GetLevel())
		})
	}
}

func Test_Logger_Debug(t *testing.T) {
	cfg := &config.Config{
		Logging: struct {
			Level  string `yaml:"level"`
			Format string `yaml:"format"`
		}{
			Level:  DebugLevel,
			Format: ConsoleFormat,
		},
	}

	logger := NewLogger(cfg)
	logger.Debug().Msg("debug message")

	assert.NotNil(t, logger)
}

func Test_Logger_Info(t *testing.T) {
	cfg := &config.Config{
		Logging: struct {
			Level  string `yaml:"level"`
			Format string `yaml:"format"`
		}{
			Level:  InfoLevel,
			Format: ConsoleFormat,
		},
	}

	logger := NewLogger(cfg)
	logger.Info().Msg("info message")

	assert.NotNil(t, logger)
}

func Test_Logger_Warn(t *testing.T) {
	cfg := &config.Config{
		Logging: struct {
			Level  string `yaml:"level"`
			Format string `yaml:"format"`
		}{
			Level:  WarnLevel,
			Format: ConsoleFormat,
		},
	}

	logger := NewLogger(cfg)
	logger.Warn().Msg("warn message")

	assert.NotNil(t, logger)
}

func Test_Logger_Error(t *testing.T) {
	cfg := &config.Config{
		Logging: struct {
			Level  string `yaml:"level"`
			Format string `yaml:"format"`
		}{
			Level:  ErrorLevel,
			Format: ConsoleFormat,
		},
	}

	logger := NewLogger(cfg)
	logger.Error().Msg("error message")

	assert.NotNil(t, logger)
}

func Test_getLogLevel(t *testing.T) {
	tests := []struct {
		name     string
		level    string
		expected zerolog.Level
	}{
		{
			name:     "Debug",
			level:    DebugLevel,
			expected: zerolog.DebugLevel,
		},
		{
			name:     "Info",
			level:    InfoLevel,
			expected: zerolog.InfoLevel,
		},
		{
			name:     "Warn",
			level:    WarnLevel,
			expected: zerolog.WarnLevel,
		},
		{
			name:     "Error",
			level:    ErrorLevel,
			expected: zerolog.ErrorLevel,
		},
		{
			name:     "Fatal",
			level:    FatalLevel,
			expected: zerolog.FatalLevel,
		},
		{
			name:     "Panic",
			level:    PanicLevel,
			expected: zerolog.PanicLevel,
		},
		{
			name:     "Trace",
			level:    TraceLevel,
			expected: zerolog.TraceLevel,
		},
		{
			name:     "Unknown",
			level:    "unknown",
			expected: zerolog.InfoLevel,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			level := getLogLevel(tt.level)
			assert.Equal(t, tt.expected, level)
		})
	}
}

func Test_Module(t *testing.T) {
	assert.NotNil(t, Module)
}

func Test_AppLogger_AllMethods(t *testing.T) {
	cfg := &config.Config{
		Logging: struct {
			Level  string `yaml:"level"`
			Format string `yaml:"format"`
		}{
			Level:  DebugLevel,
			Format: JSONFormat,
		},
	}

	logger := NewLogger(cfg)
	assert.NotNil(t, logger)

	debugEvent := logger.Debug()
	assert.NotNil(t, debugEvent)

	infoEvent := logger.Info()
	assert.NotNil(t, infoEvent)

	warnEvent := logger.Warn()
	assert.NotNil(t, warnEvent)

	errorEvent := logger.Error()
	assert.NotNil(t, errorEvent)
}

func Test_NewLogger_AllFormats(t *testing.T) {
	tests := []struct {
		name   string
		format string
	}{
		{"console format", ConsoleFormat},
		{"json format", JSONFormat},
		{"empty format", ""},
		{"unknown format", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				Logging: struct {
					Level  string `yaml:"level"`
					Format string `yaml:"format"`
				}{
					Level:  InfoLevel,
					Format: tt.format,
				},
			}

			logger := NewLogger(cfg)
			assert.NotNil(t, logger)

			appLogger, ok := logger.(*AppLogger)
			assert.True(t, ok)
			assert.NotNil(t, appLogger.log)
		})
	}
}

func Test_NewLogger_AllLevels(t *testing.T) {
	tests := []struct {
		name  string
		level string
	}{
		{"debug level", DebugLevel},
		{"info level", InfoLevel},
		{"warn level", WarnLevel},
		{"error level", ErrorLevel},
		{"fatal level", FatalLevel},
		{"panic level", PanicLevel},
		{"trace level", TraceLevel},
		{"empty level", ""},
		{"unknown level", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				Logging: struct {
					Level  string `yaml:"level"`
					Format string `yaml:"format"`
				}{
					Level:  tt.level,
					Format: ConsoleFormat,
				},
			}

			logger := NewLogger(cfg)
			assert.NotNil(t, logger)

			appLogger, ok := logger.(*AppLogger)
			assert.True(t, ok)
			assert.NotNil(t, appLogger.log)
		})
	}
}

func Test_zerologEvent(t *testing.T) {
	cfg := config.DefaultConfig()
	logger := NewLogger(cfg)

	t.Run("Event chaining", func(t *testing.T) {
		event := logger.Debug()

		result := event.Str("key", "value")
		assert.NotNil(t, result)

		result = event.Int("count", 42)
		assert.NotNil(t, result)

		result = event.Dur("duration", time.Second)
		assert.NotNil(t, result)

		result = event.Err(errors.New("test error"))
		assert.NotNil(t, result)

		event.Msg("test message")
		event.Msgf("test %s", "formatted")
	})

	t.Run("All log levels", func(t *testing.T) {
		debug := logger.Debug()
		assert.NotNil(t, debug)
		debug.Msg("debug test")

		info := logger.Info()
		assert.NotNil(t, info)
		info.Msg("info test")

		warn := logger.Warn()
		assert.NotNil(t, warn)
		warn.Msg("warn test")

		err := logger.Error()
		assert.NotNil(t, err)
		err.Msg("error test")
	})
}

func Test_NoopEvent(t *testing.T) {
	event := &NoopEvent{}

	t.Run("All methods return self or do nothing", func(t *testing.T) {
		result := event.Str("key", "value")
		assert.Equal(t, event, result)

		result = event.Int("count", 42)
		assert.Equal(t, event, result)

		result = event.Dur("duration", time.Second)
		assert.Equal(t, event, result)

		result = event.Err(errors.New("test error"))
		assert.Equal(t, event, result)

		event.Msg("test message")
		event.Msgf("test %s", "formatted")
	})
}
