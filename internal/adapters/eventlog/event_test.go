package eventlog

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_NewEventLogger_RendersLogfmt(t *testing.T) {
	var buf bytes.Buffer

	l := newEventLogger(&buf)
	l.Log().Str("key", "value").Msg("test")

	assert.Equal(t, "test key=value\n", buf.String())
}
