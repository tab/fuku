package logger

import (
	"testing"

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
