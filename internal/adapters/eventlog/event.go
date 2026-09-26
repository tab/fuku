package eventlog

import (
	"bytes"

	"github.com/rs/zerolog"
)

// newEventLogger creates a zerolog logger that writes logfmt into buf
func newEventLogger(buf *bytes.Buffer) zerolog.Logger {
	w := zerolog.ConsoleWriter{
		Out:        buf,
		NoColor:    true,
		PartsOrder: []string{zerolog.MessageFieldName},
	}

	return zerolog.New(w)
}
