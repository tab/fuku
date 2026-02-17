package logs

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"fuku/internal/config"
	"fuku/internal/config/logger"
)

func Test_NewLogFormatter(t *testing.T) {
	tests := []struct {
		name   string
		cfg    *config.Config
		format string
	}{
		{
			name: "Console format",
			cfg: &config.Config{
				Logging: struct {
					Level  string `yaml:"level"`
					Format string `yaml:"format"`
				}{
					Format: logger.ConsoleFormat,
				},
			},
			format: logger.ConsoleFormat,
		},
		{
			name: "JSON format",
			cfg: &config.Config{
				Logging: struct {
					Level  string `yaml:"level"`
					Format string `yaml:"format"`
				}{
					Format: logger.JSONFormat,
				},
			},
			format: logger.JSONFormat,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := NewLogFormatter(tt.cfg)

			assert.NotNil(t, f)
			assert.Equal(t, tt.format, f.format)
			assert.False(t, f.enabled)
		})
	}
}

func Test_LogFormatter_SetEnabled(t *testing.T) {
	cfg := &config.Config{
		Logging: struct {
			Level  string `yaml:"level"`
			Format string `yaml:"format"`
		}{
			Format: logger.ConsoleFormat,
		},
	}
	f := NewLogFormatter(cfg)

	assert.False(t, f.enabled)

	f.SetEnabled(true)
	assert.True(t, f.enabled)

	f.SetEnabled(false)
	assert.False(t, f.enabled)
}

func Test_LogFormatter_Write_Disabled(t *testing.T) {
	cfg := &config.Config{
		Logging: struct {
			Level  string `yaml:"level"`
			Format string `yaml:"format"`
		}{
			Format: logger.ConsoleFormat,
		},
	}
	f := NewLogFormatter(cfg)

	n, err := f.Write([]byte("test message"))

	assert.NoError(t, err)
	assert.Equal(t, 12, n)
}

func Test_LogFormatter_FormatMessage(t *testing.T) {
	tests := []struct {
		name     string
		format   string
		service  string
		message  string
		contains []string
	}{
		{
			name:     "Console format",
			format:   logger.ConsoleFormat,
			service:  "api",
			message:  "test message",
			contains: []string{"api", "|", "test message"},
		},
		{
			name:     "JSON format",
			format:   logger.JSONFormat,
			service:  "api",
			message:  "test message",
			contains: []string{`"service":"api"`, `"message":"test message"`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				Logging: struct {
					Level  string `yaml:"level"`
					Format string `yaml:"format"`
				}{
					Format: tt.format,
				},
			}
			f := NewLogFormatter(cfg)

			result := f.FormatMessage(tt.service, tt.message)

			for _, c := range tt.contains {
				assert.Contains(t, result, c)
			}
		})
	}
}

func Test_LogFormatter_WriteFormatted(t *testing.T) {
	cfg := &config.Config{
		Logging: struct {
			Level  string `yaml:"level"`
			Format string `yaml:"format"`
		}{
			Format: logger.ConsoleFormat,
		},
	}
	f := NewLogFormatter(cfg)

	var buf bytes.Buffer
	f.WriteFormatted(&buf, "test-service", "hello world")

	output := buf.String()
	assert.Contains(t, output, "test-service")
	assert.Contains(t, output, "hello world")
}

func Test_hashString(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect int
	}{
		{
			name:   "Empty string",
			input:  "",
			expect: 0,
		},
		{
			name:   "Simple string",
			input:  "api",
			expect: 96794,
		},
		{
			name:   "Longer string",
			input:  "authentication-service",
			expect: 8745230323607049312,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hashString(tt.input)

			assert.GreaterOrEqual(t, result, 0)
			assert.Equal(t, tt.expect, result)
		})
	}
}

func Test_hashString_Consistency(t *testing.T) {
	input := "test-service"

	result1 := hashString(input)
	result2 := hashString(input)

	assert.Equal(t, result1, result2)
}

func Test_LogFormatter_getServiceStyle(t *testing.T) {
	cfg := &config.Config{
		Logging: struct {
			Level  string `yaml:"level"`
			Format string `yaml:"format"`
		}{
			Format: logger.ConsoleFormat,
		},
	}
	f := NewLogFormatter(cfg)

	style1 := f.getServiceStyle("api")
	style2 := f.getServiceStyle("api")

	assert.Equal(t, style1, style2)

	style3 := f.getServiceStyle("db")
	assert.NotNil(t, style3)
}

func Test_LogFormatter_formatLine(t *testing.T) {
	cfg := &config.Config{
		Logging: struct {
			Level  string `yaml:"level"`
			Format string `yaml:"format"`
		}{
			Format: logger.ConsoleFormat,
		},
	}
	f := NewLogFormatter(cfg)

	line := f.formatLine("api", "test message")

	assert.Contains(t, line, "api")
	assert.Contains(t, line, "|")
	assert.Contains(t, line, "test message")
	assert.True(t, strings.HasSuffix(line, "\n"))
}

func Test_LogFormatter_formatLine_UpdatesMaxServiceLen(t *testing.T) {
	cfg := &config.Config{
		Logging: struct {
			Level  string `yaml:"level"`
			Format string `yaml:"format"`
		}{
			Format: logger.ConsoleFormat,
		},
	}
	f := NewLogFormatter(cfg)
	initialLen := f.maxServiceLen

	f.formatLine("very-long-service-name", "message")

	assert.Greater(t, f.maxServiceLen, initialLen)
}

func Test_RenderBanner(t *testing.T) {
	tests := []struct {
		name       string
		status     StatusMessage
		subscribed []string
		contains   []string
	}{
		{
			name: "All services",
			status: StatusMessage{
				Type:     MessageStatus,
				Version:  config.Version,
				Profile:  "default",
				Services: []string{"api", "db", "cache"},
			},
			subscribed: nil,
			contains: []string{
				"logs",
				"v" + config.Version,
				"default",
				"3 running",
				"showing:",
				"all",
				"ctrl+c",
				"╭", "╰", "│",
			},
		},
		{
			name: "Filtered services",
			status: StatusMessage{
				Type:     MessageStatus,
				Version:  config.Version,
				Profile:  "backend",
				Services: []string{"api", "db", "cache", "auth", "gateway"},
			},
			subscribed: []string{"api", "db"},
			contains: []string{
				"logs",
				"v" + config.Version,
				"backend",
				"5 running",
				"api, db",
				"ctrl+c",
			},
		},
		{
			name: "More than 5 filtered services truncated",
			status: StatusMessage{
				Type:     MessageStatus,
				Version:  config.Version,
				Profile:  "default",
				Services: []string{"s1", "s2", "s3", "s4", "s5", "s6", "s7"},
			},
			subscribed: []string{"s1", "s2", "s3", "s4", "s5", "s6", "s7"},
			contains: []string{
				"s1, s2, s3, s4, s5",
				"and 2 more",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.DefaultConfig()
			f := NewLogFormatter(cfg)

			var buf bytes.Buffer
			f.RenderBanner(&buf, tt.status, tt.subscribed)

			output := buf.String()
			for _, c := range tt.contains {
				assert.Contains(t, output, c)
			}
		})
	}
}
