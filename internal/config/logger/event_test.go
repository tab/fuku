package logger

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_NewEventLogger(t *testing.T) {
	el := NewEventLogger()
	assert.NotNil(t, el)
}

func Test_EventLogger_NewLogger(t *testing.T) {
	el := NewEventLogger()

	var buf bytes.Buffer

	l := el.NewLogger(&buf)

	l.Log().Str("key", "value").Msg("test")

	output := buf.String()
	assert.Contains(t, output, "test")
	assert.Contains(t, output, "key=value")
}
