package process

import (
	"bytes"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"fuku/internal/model"
)

func Test_logsStream(t *testing.T) {
	tests := []struct {
		name     string
		service  model.Service
		stream   string
		expected bool
	}{
		{
			name:     "stdout configured logs stdout",
			service:  model.Service{Name: "api", LogOutput: []string{"stdout"}},
			stream:   streamStdout,
			expected: true,
		},
		{
			name:     "stdout configured skips stderr",
			service:  model.Service{Name: "api", LogOutput: []string{"stdout"}},
			stream:   streamStderr,
			expected: false,
		},
		{
			name:     "stderr configured logs stderr",
			service:  model.Service{Name: "api", LogOutput: []string{"stderr"}},
			stream:   streamStderr,
			expected: true,
		},
		{
			name:     "both configured logs stdout",
			service:  model.Service{Name: "api", LogOutput: []string{"stdout", "stderr"}},
			stream:   streamStdout,
			expected: true,
		},
		{
			name:     "the match ignores case",
			service:  model.Service{Name: "api", LogOutput: []string{"STDOUT"}},
			stream:   streamStdout,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := logsStream(tt.service, tt.stream)

			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_streamWriter(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSink := NewMockLogSink(ctrl)

	log := slog.New(slog.DiscardHandler)

	factory := &Factory{sink: mockSink, log: log}

	longLine := strings.Repeat("x", maxLineSize+16)

	stdoutOnly := model.Service{Name: "api", LogOutput: []string{"stdout"}}

	tests := []struct {
		name    string
		before  func()
		service model.Service
		stream  string
		writes  []string
	}{
		{
			name: "a logged stream forwards every line to the sink",
			before: func() {
				mockSink.EXPECT().Broadcast("api", "line one")
				mockSink.EXPECT().Broadcast("api", "line two")
			},
			service: stdoutOnly,
			stream:  streamStdout,
			writes:  []string{"line one\nline two\n"},
		},
		{
			name:    "a stream the service does not log reaches no sink",
			before:  func() {},
			service: stdoutOnly,
			stream:  streamStderr,
			writes:  []string{"error line\n"},
		},
		{
			name: "a line split across writes is forwarded whole",
			before: func() {
				mockSink.EXPECT().Broadcast("api", "line one")
			},
			service: stdoutOnly,
			stream:  streamStdout,
			writes:  []string{"line ", "one\n"},
		},
		{
			name: "a final line without a newline is forwarded on close",
			before: func() {
				mockSink.EXPECT().Broadcast("api", "no newline")
			},
			service: stdoutOnly,
			stream:  streamStdout,
			writes:  []string{"no newline"},
		},
		{
			name: "a carriage return before the newline is dropped from the line",
			before: func() {
				mockSink.EXPECT().Broadcast("api", "line one")
			},
			service: stdoutOnly,
			stream:  streamStdout,
			writes:  []string{"line one\r\n"},
		},
		{
			name: "a line longer than the cap is truncated in the broadcast",
			before: func() {
				mockSink.EXPECT().Broadcast("api", longLine[:maxLineSize])
			},
			service: stdoutOnly,
			stream:  streamStdout,
			writes:  []string{longLine + "\n"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.before()

			reader, pipe := io.Pipe()

			var copied bytes.Buffer

			done := make(chan struct{})

			collect := func() {
				io.Copy(&copied, reader)
				close(done)
			}

			go collect()

			writer := factory.newStreamWriter(pipe, tt.service, tt.stream)

			for _, chunk := range tt.writes {
				writer.Write([]byte(chunk))
			}

			writer.Close()

			select {
			case <-done:
			case <-time.After(2 * time.Second):
				t.Fatal("the copied stream did not finish")
			}

			assert.Equal(t, strings.Join(tt.writes, ""), copied.String())
		})
	}
}
