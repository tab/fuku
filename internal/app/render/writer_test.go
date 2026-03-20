package render

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fuku/internal/config"
	"fuku/internal/config/logger"
)

func Test_NewWriter(t *testing.T) {
	cfg := config.DefaultConfig()
	log := NewLog(false)

	var buf bytes.Buffer

	w := NewWriter(cfg, log, &buf)

	require.NotNil(t, w)
}

func Test_Writer_Write_Disabled(t *testing.T) {
	cfg := config.DefaultConfig()
	log := NewLog(false)

	var buf bytes.Buffer

	w := NewWriter(cfg, log, &buf)

	n, err := w.Write([]byte("some data"))

	require.NoError(t, err)
	assert.Equal(t, 9, n)
	assert.Empty(t, buf.String())
}

func Test_Writer_Write_EnabledJSONFormat(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Logging.Format = logger.JSONFormat
	log := NewLog(false)

	var buf bytes.Buffer

	w := NewWriter(cfg, log, &buf)
	w.SetEnabled(true)

	input := `{"level":"info","message":"hello"}` + "\n"
	n, err := w.Write([]byte(input))

	require.NoError(t, err)
	assert.Equal(t, len(input), n)
	assert.Equal(t, input, buf.String())
}

func Test_Writer_Write_EnabledConsoleFormatValidJSON(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Logging.Format = logger.ConsoleFormat
	log := NewLog(false)

	var buf bytes.Buffer

	w := NewWriter(cfg, log, &buf)
	w.SetEnabled(true)

	entry := logEntry{
		Service: "api",
		Message: "server started",
	}
	data, err := json.Marshal(entry)
	require.NoError(t, err)

	n, writeErr := w.Write(data)

	require.NoError(t, writeErr)
	assert.Equal(t, len(data), n)

	output := buf.String()
	assert.Contains(t, output, "api")
	assert.Contains(t, output, "server started")
}

func Test_Writer_Write_EnabledConsoleFormatInvalidJSON(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Logging.Format = logger.ConsoleFormat
	log := NewLog(false)

	var buf bytes.Buffer

	w := NewWriter(cfg, log, &buf)
	w.SetEnabled(true)

	input := []byte("not json at all")
	n, err := w.Write(input)

	require.NoError(t, err)
	assert.Equal(t, len(input), n)
	assert.Equal(t, "not json at all", buf.String())
}

func Test_Writer_Write_ConsoleServiceNameFromEntry(t *testing.T) {
	tests := []struct {
		name        string
		entry       logEntry
		expectInOut string
	}{
		{
			name: "uses entry.Service when set",
			entry: logEntry{
				Service: "worker",
				Message: "processing",
			},
			expectInOut: "worker",
		},
		{
			name: "defaults to AppName when Service is empty",
			entry: logEntry{
				Message: "starting",
			},
			expectInOut: config.AppName,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.DefaultConfig()
			cfg.Logging.Format = logger.ConsoleFormat
			log := NewLog(false)

			var buf bytes.Buffer

			w := NewWriter(cfg, log, &buf)
			w.SetEnabled(true)

			data, err := json.Marshal(tt.entry)
			require.NoError(t, err)

			_, writeErr := w.Write(data)
			require.NoError(t, writeErr)

			assert.Contains(t, buf.String(), tt.expectInOut)
		})
	}
}

func Test_Writer_Write_ConsoleComponentPrepended(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Logging.Format = logger.ConsoleFormat
	log := NewLog(false)

	var buf bytes.Buffer

	w := NewWriter(cfg, log, &buf)
	w.SetEnabled(true)

	entry := logEntry{
		Service:   "api",
		Component: "HTTP",
		Message:   "request received",
	}
	data, err := json.Marshal(entry)
	require.NoError(t, err)

	_, writeErr := w.Write(data)
	require.NoError(t, writeErr)

	output := buf.String()
	assert.Contains(t, output, "[HTTP]")
	assert.Contains(t, output, "request received")
}

func Test_Writer_SetEnabled(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Logging.Format = logger.JSONFormat
	log := NewLog(false)

	var buf bytes.Buffer

	w := NewWriter(cfg, log, &buf)

	w.Write([]byte("should be dropped\n"))
	assert.Empty(t, buf.String())

	w.SetEnabled(true)
	w.Write([]byte("should appear\n"))
	assert.Equal(t, "should appear\n", buf.String())

	buf.Reset()
	w.SetEnabled(false)
	w.Write([]byte("dropped again\n"))
	assert.Empty(t, buf.String())
}
