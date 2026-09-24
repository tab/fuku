package output

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"fuku/internal/platform/buildinfo"
	"fuku/internal/platform/logging"
)

func Test_NewWriter(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockFormatter := NewMockFormatter(ctrl)

	var buf bytes.Buffer

	w := NewWriter(Options{Format: logging.FormatConsole}, mockFormatter, &buf)

	require.NotNil(t, w)
	assert.Equal(t, logging.FormatConsole, w.format)
	assert.Equal(t, mockFormatter, w.formatter)
	assert.False(t, w.enabled)
}

func Test_Writer_Write(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockFormatter := NewMockFormatter(ctrl)

	tests := []struct {
		name     string
		format   string
		enabled  bool
		input    string
		before   func()
		expected string
	}{
		{
			name:     "disabled drops the entry",
			format:   logging.FormatConsole,
			input:    `{"message":"hello"}`,
			before:   func() {},
			expected: "",
		},
		{
			name:     "JSON format passes the entry through",
			format:   logging.FormatJSON,
			enabled:  true,
			input:    `{"level":"info","message":"hello"}` + "\n",
			before:   func() {},
			expected: `{"level":"info","message":"hello"}` + "\n",
		},
		{
			name:    "console format renders the service line",
			format:  logging.FormatConsole,
			enabled: true,
			input:   `{"service":"api","message":"server started"}`,
			before: func() {
				mockFormatter.EXPECT().FormatServiceLine("api", "server started").Return("api | server started\n")
			},
			expected: "api | server started\n",
		},
		{
			name:    "console format defaults the service to the app name",
			format:  logging.FormatConsole,
			enabled: true,
			input:   `{"message":"starting"}`,
			before: func() {
				mockFormatter.EXPECT().FormatServiceLine(buildinfo.AppName, "starting").Return("fuku | starting\n")
			},
			expected: "fuku | starting\n",
		},
		{
			name:    "console format prepends the component",
			format:  logging.FormatConsole,
			enabled: true,
			input:   `{"service":"api","component":"HTTP","message":"request received"}`,
			before: func() {
				mockFormatter.EXPECT().FormatServiceLine("api", "[HTTP] request received").Return("api | [HTTP] request received\n")
			},
			expected: "api | [HTTP] request received\n",
		},
		{
			name:     "console format passes a non-JSON entry through",
			format:   logging.FormatConsole,
			enabled:  true,
			input:    "not json at all",
			before:   func() {},
			expected: "not json at all",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.before()

			var buf bytes.Buffer

			w := NewWriter(Options{Format: tt.format}, mockFormatter, &buf)
			w.SetEnabled(tt.enabled)

			n, err := w.Write([]byte(tt.input))

			require.NoError(t, err)
			assert.Equal(t, len(tt.input), n)
			assert.Equal(t, tt.expected, buf.String())
		})
	}
}

func Test_Writer_SetEnabled(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockFormatter := NewMockFormatter(ctrl)

	var buf bytes.Buffer

	entry, err := json.Marshal(logEntry{Message: "toggled"})
	require.NoError(t, err)

	w := NewWriter(Options{Format: logging.FormatJSON}, mockFormatter, &buf)

	w.Write(entry)
	assert.Empty(t, buf.String())

	w.SetEnabled(true)
	w.Write(entry)
	assert.Equal(t, string(entry), buf.String())

	buf.Reset()
	w.SetEnabled(false)
	w.Write(entry)
	assert.Empty(t, buf.String())
}
