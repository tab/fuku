package process

import (
	"bytes"
	"io"
	"strings"

	"fuku/internal/model"
)

const maxLineSize = 4 * 1024 * 1024

// Stream names as logs.output lists them
const (
	streamStdout = "STDOUT"
	streamStderr = "STDERR"
)

// LogSink receives every line a service writes on a stream it logs
type LogSink interface {
	Broadcast(service, message string)
}

// streamWriter passes a child stream to the handle's reader and logs and forwards each line of a logged stream
type streamWriter struct {
	dst     *io.PipeWriter
	service string
	stream  string
	logged  bool
	line    []byte
	sink    LogSink
	log     Logger
}

// newStreamWriter creates the writer exec copies one stream of the child into
func (f *Factory) newStreamWriter(dst *io.PipeWriter, service model.Service, stream string) *streamWriter {
	return &streamWriter{
		dst:     dst,
		service: service.Name,
		stream:  stream,
		logged:  logsStream(service, stream),
		sink:    f.sink,
		log:     f.log,
	}
}

// Write passes the chunk to the handle's reader and emits every line it completes
func (w *streamWriter) Write(chunk []byte) (int, error) {
	//nolint:errcheck // pipe write errors are handled by the reader
	w.dst.Write(chunk)

	if !w.logged {
		return len(chunk), nil
	}

	rest := chunk

	for {
		line, after, found := bytes.Cut(rest, []byte{'\n'})
		w.append(line)

		if !found {
			return len(chunk), nil
		}

		w.emit()

		rest = after
	}
}

// Close emits a final line that has no newline and closes the handle's reader
func (w *streamWriter) Close() error {
	if len(w.line) > 0 {
		w.emit()
	}

	return w.dst.Close()
}

// append adds part of a line to the pending line, truncated at maxLineSize
func (w *streamWriter) append(part []byte) {
	room := maxLineSize - len(w.line)
	w.line = append(w.line, part[:min(len(part), room)]...)
}

// emit logs and forwards the pending line
func (w *streamWriter) emit() {
	text := string(bytes.TrimSuffix(w.line, []byte{'\r'}))
	w.log.Info(text, "service", w.service, "stream", w.stream)
	w.sink.Broadcast(w.service, text)

	w.line = w.line[:0]
}

// logsStream reports whether the service logs a stream (no log output or an empty list logs both)
func logsStream(service model.Service, stream string) bool {
	if len(service.LogOutput) == 0 {
		return true
	}

	for _, output := range service.LogOutput {
		if strings.EqualFold(output, stream) {
			return true
		}
	}

	return false
}
