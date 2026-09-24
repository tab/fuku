package terminal

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fuku/internal/platform/logging"
)

func Test_NewLog(t *testing.T) {
	theme := NewTheme(AppearanceLight)

	log := NewLog(Options{Format: logging.FormatConsole}, theme)

	require.NotNil(t, log)
	assert.Equal(t, logging.FormatConsole, log.format)
	assert.Equal(t, LogStreamMaxServiceNameLen, log.maxServiceLen)
	assert.NotNil(t, log.serviceStyles)
}

func Test_Log_FormatServiceLine(t *testing.T) {
	theme := NewTheme(AppearanceLight)

	log := NewLog(Options{Format: logging.FormatConsole}, theme)

	result := log.FormatServiceLine("api", "hello world")

	assert.Contains(t, result, "api")
	assert.Contains(t, result, "|")
	assert.Contains(t, result, "hello world")
	assert.True(t, strings.HasSuffix(result, "\n"))
}

func Test_Log_FormatServiceLine_AlignsToTheLongestName(t *testing.T) {
	theme := NewTheme(AppearanceLight)

	log := NewLog(Options{Format: logging.FormatConsole}, theme)

	log.FormatServiceLine("long-service-name", "test")

	assert.GreaterOrEqual(t, log.maxServiceLen, len("long-service-name"))
}

func Test_Log_FormatMessage(t *testing.T) {
	theme := NewTheme(AppearanceLight)

	tests := []struct {
		name     string
		format   string
		service  string
		message  string
		expected string
	}{
		{
			name:     "console format renders the padded line",
			format:   logging.FormatConsole,
			service:  "api",
			message:  "started",
			expected: "api          | started\n",
		},
		{
			name:     "JSON format returns JSON",
			format:   logging.FormatJSON,
			service:  "web",
			message:  "listening on :8080",
			expected: `{"service":"web","message":"listening on :8080"}` + "\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			log := NewLog(Options{Format: tt.format}, theme)

			result := log.FormatMessage(tt.service, tt.message)

			assert.Equal(t, tt.expected, ansi.Strip(result))
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hashString(tt.input)

			assert.Equal(t, tt.expect, result)
			assert.GreaterOrEqual(t, result, 0)
		})
	}
}

func Test_hashString_NegativeOverflow(t *testing.T) {
	result := hashString("superlongservicenamethatwilloverflowtheinteger")

	assert.Positive(t, result)
}

func Test_Log_getServiceStyle(t *testing.T) {
	theme := NewTheme(AppearanceLight)

	log := NewLog(Options{Format: logging.FormatConsole}, theme)

	style1 := log.getServiceStyle("api")
	style2 := log.getServiceStyle("api")
	style3 := log.getServiceStyle("web")

	assert.Equal(t, style1, style2)
	assert.NotNil(t, style3)
	assert.Len(t, log.serviceStyles, 2)
}
