package render

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fuku/internal/app/relay"
	"fuku/internal/config/logger"
)

func Test_NewLog(t *testing.T) {
	log := NewLog(false)

	require.NotNil(t, log)
	assert.NotNil(t, log.serviceStyles)
	assert.NotNil(t, log.theme.ServiceColorPalette)
}

func Test_Log_FormatServiceLine(t *testing.T) {
	log := NewLog(false)

	result := log.FormatServiceLine("api", "hello world")

	assert.Contains(t, result, "api")
	assert.Contains(t, result, "|")
	assert.Contains(t, result, "hello world")
	assert.True(t, strings.HasSuffix(result, "\n"))
}

func Test_Log_WriteServiceLine(t *testing.T) {
	log := NewLog(false)

	var buf bytes.Buffer

	log.WriteServiceLine(&buf, "api", "test message")

	output := buf.String()
	assert.Contains(t, output, "api")
	assert.Contains(t, output, "test message")
	assert.NotEmpty(t, output)
}

func Test_Log_RenderBanner(t *testing.T) {
	tests := []struct {
		name       string
		status     relay.StatusMessage
		subscribed []string
		expect     func(t *testing.T, output string)
	}{
		{
			name: "all services",
			status: relay.StatusMessage{
				Profile:  "default",
				Version:  "1.0.0",
				Services: []string{"api", "web", "worker"},
			},
			subscribed: nil,
			expect: func(t *testing.T, output string) {
				assert.Contains(t, output, "logs")
				assert.Contains(t, output, "default")
				assert.Contains(t, output, "3 running")
				assert.Contains(t, output, "all")
			},
		},
		{
			name: "filtered services",
			status: relay.StatusMessage{
				Profile:  "backend",
				Version:  "1.0.0",
				Services: []string{"api", "web", "worker"},
			},
			subscribed: []string{"api", "web"},
			expect: func(t *testing.T, output string) {
				assert.Contains(t, output, "backend")
				assert.Contains(t, output, "api, web")
			},
		},
		{
			name: "more than 5 services truncated",
			status: relay.StatusMessage{
				Profile:  "all",
				Version:  "1.0.0",
				Services: []string{"a", "b", "c", "d", "e", "f", "g", "h"},
			},
			subscribed: []string{"s1", "s2", "s3", "s4", "s5", "s6", "s7"},
			expect: func(t *testing.T, output string) {
				assert.Contains(t, output, "s1, s2, s3, s4, s5")
				assert.Contains(t, output, "and 2 more")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			log := NewLog(false)

			var buf bytes.Buffer

			log.RenderBanner(&buf, BannerOptions{
				Width:      80,
				Status:     tt.status,
				Subscribed: tt.subscribed,
				Following:  true,
			})

			tt.expect(t, buf.String())
		})
	}
}

func Test_Log_RenderBanner_ReportsAProjectMismatch(t *testing.T) {
	tests := []struct {
		name    string
		local   string
		served  string
		warning bool
	}{
		{name: "same project", local: "aaaaaaaaaaaaaaaa", served: "aaaaaaaaaaaaaaaa", warning: false},
		{name: "different project", local: "aaaaaaaaaaaaaaaa", served: "bbbbbbbbbbbbbbbb", warning: true},
		{name: "instance does not report a project", local: "aaaaaaaaaaaaaaaa", served: "", warning: false},
		{name: "local project is unknown", local: "", served: "bbbbbbbbbbbbbbbb", warning: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			log := NewLog(false)

			var buf bytes.Buffer

			log.RenderBanner(&buf, BannerOptions{
				Width: 80,
				Status: relay.StatusMessage{
					Profile:  "default",
					Version:  "1.0.0",
					Project:  tt.served,
					Services: []string{"api"},
				},
				Project:   tt.local,
				Following: true,
			})

			output := buf.String()

			if !tt.warning {
				assert.NotContains(t, output, "serves a different project")

				return
			}

			assert.Contains(t, output, "serves a different project")
		})
	}
}

func Test_Log_RenderBanner_HidesTheExitHintWhenNotFollowing(t *testing.T) {
	tests := []struct {
		name      string
		following bool
		expect    func(t *testing.T, output string)
	}{
		{
			name:      "following shows how to exit",
			following: true,
			expect: func(t *testing.T, output string) {
				t.Helper()

				assert.Contains(t, output, "ctrl+c")
			},
		},
		{
			name:      "a bounded read has already exited",
			following: false,
			expect: func(t *testing.T, output string) {
				t.Helper()

				assert.NotContains(t, output, "ctrl+c")
				assert.Contains(t, output, "minimal")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			log := NewLog(false)

			var buf bytes.Buffer

			log.RenderBanner(&buf, BannerOptions{
				Width:      80,
				Status:     relay.StatusMessage{Profile: "minimal", Version: "1.0.0", Services: []string{"api"}},
				Subscribed: []string{"api"},
				Following:  tt.following,
			})

			tt.expect(t, buf.String())
		})
	}
}

func Test_Log_FormatMessage(t *testing.T) {
	tests := []struct {
		name    string
		format  string
		service string
		message string
		expect  func(t *testing.T, result string)
	}{
		{
			name:    "console format delegates to FormatServiceLine",
			format:  logger.ConsoleFormat,
			service: "api",
			message: "started",
			expect: func(t *testing.T, result string) {
				assert.Contains(t, result, "api")
				assert.Contains(t, result, "|")
				assert.Contains(t, result, "started")
				assert.True(t, strings.HasSuffix(result, "\n"))
			},
		},
		{
			name:    "JSON format returns JSON",
			format:  logger.JSONFormat,
			service: "web",
			message: "listening on :8080",
			expect: func(t *testing.T, result string) {
				var parsed map[string]string

				err := json.Unmarshal([]byte(result), &parsed)
				require.NoError(t, err)
				assert.Equal(t, "web", parsed["service"])
				assert.Equal(t, "listening on :8080", parsed["message"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			log := NewLog(false)

			result := log.FormatMessage(tt.format, tt.service, tt.message)

			tt.expect(t, result)
		})
	}
}

func Test_FormatJSON(t *testing.T) {
	result := FormatJSON("api", "hello world")

	var parsed map[string]string

	err := json.Unmarshal([]byte(result), &parsed)
	require.NoError(t, err)
	assert.Equal(t, "api", parsed["service"])
	assert.Equal(t, "hello world", parsed["message"])
	assert.True(t, strings.HasSuffix(result, "\n"))
}

func Test_hashString(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect int
	}{
		{
			name:   "empty string returns 0",
			input:  "",
			expect: 0,
		},
		{
			name:   "api returns 96794",
			input:  "api",
			expect: 96794,
		},
		{
			name:   "consistency check",
			input:  "api",
			expect: hashString("api"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hashString(tt.input)

			assert.Equal(t, tt.expect, result)
			assert.GreaterOrEqual(t, result, 0)
		})
	}
}

func Test_Log_getServiceStyle(t *testing.T) {
	log := NewLog(false)

	log.mu.Lock()
	style1 := log.getServiceStyle("api")
	style2 := log.getServiceStyle("api")
	log.mu.Unlock()

	assert.Equal(t, style1, style2)

	log.mu.Lock()
	style3 := log.getServiceStyle("web")
	log.mu.Unlock()

	assert.NotNil(t, style3)
}

func Test_hashString_NegativeOverflow(t *testing.T) {
	result := hashString("superlongservicenamethatwilloverflowtheinteger")

	assert.Positive(t, result)
}

func Test_Log_Theme(t *testing.T) {
	log := NewLog(false)

	theme := log.Theme()

	assert.NotNil(t, theme)
	assert.NotNil(t, theme.ServiceColorPalette)
}

func Test_Log_formatLine(t *testing.T) {
	log := NewLog(false)

	log.mu.Lock()
	result := log.formatLine("api", "hello world")
	log.mu.Unlock()

	assert.Contains(t, result, "api")
	assert.Contains(t, result, "|")
	assert.Contains(t, result, "hello world")
	assert.True(t, strings.HasSuffix(result, "\n"))

	log.mu.Lock()
	log.formatLine("long-service-name", "test")
	assert.GreaterOrEqual(t, log.maxServiceLen, len("long-service-name"))
	log.mu.Unlock()
}
