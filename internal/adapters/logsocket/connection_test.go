package logsocket

import (
	"context"
	"io"
	"log/slog"
	"net"
	"os"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"fuku/internal/app/logs"
	"fuku/internal/model"
	"fuku/internal/platform/buildinfo"
)

func Test_Server_handleConnection(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockHub := NewMockHub(ctrl)

	log := slog.New(slog.DiscardHandler)

	tail := 2
	identity := model.Instance{ID: "1f0c6e4a-2b8d-4c3e-9a7f-5d6b8c0e1a24", Fingerprint: "3f2a9c1d8b4e6072"}
	replay := model.ReplayOptions{Tail: &tail, NoFollow: true}
	sub := &logs.Subscription{}

	srv := NewServer(mockHub, nil, identity, log)
	srv.profile = testProfile
	srv.services = []string{"api", "web"}

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	status := marshalLine(t, StatusMessage{
		Type:          MessageStatus,
		Version:       buildinfo.Version,
		Instance:      identity.ID,
		Fingerprint:   identity.Fingerprint,
		Profile:       testProfile,
		Services:      []string{"api", "web"},
		ReplayOptions: replay,
	}) + "\n"

	dial := func(request string) (net.Conn, <-chan string) {
		client, server := net.Pipe()
		received := make(chan string, 1)

		go func() {
			defer client.Close()

			client.Write([]byte(request))

			data, _ := io.ReadAll(client)
			received <- string(data)
		}()

		return server, received
	}

	tests := []struct {
		name     string
		before   func() (net.Conn, <-chan string)
		expected string
	}{
		{
			name: "announces the status, subscribes with the request and unsubscribes at the end",
			before: func() (net.Conn, <-chan string) {
				mockHub.EXPECT().Subscribe([]string{"api"}, replay).Return(sub)
				mockHub.EXPECT().Unsubscribe(sub)

				return dial(`{"type":"subscribe","services":["api"],"tail":2,"noFollow":true}` + "\n")
			},
			expected: status,
		},
		{
			name: "rejects invalid JSON",
			before: func() (net.Conn, <-chan string) {
				return dial("not valid json\n")
			},
		},
		{
			name: "rejects a message that is not a subscribe",
			before: func() (net.Conn, <-chan string) {
				return dial(`{"type":"log","services":["api"]}` + "\n")
			},
		},
		{
			name: "rejects an explicit zero tail",
			before: func() (net.Conn, <-chan string) {
				return dial(`{"type":"subscribe","tail":0}` + "\n")
			},
		},
		{
			name: "rejects a negative tail",
			before: func() (net.Conn, <-chan string) {
				return dial(`{"type":"subscribe","tail":-1}` + "\n")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conn, received := tt.before()

			srv.handleConnection(ctx, conn)

			assert.Equal(t, tt.expected, <-received)
		})
	}
}

func Test_Server_handleConnection_DisconnectsBeforeSubscribing(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockHub := NewMockHub(ctrl)
	mockLog := NewMockLogger(ctrl)

	client, conn := net.Pipe()
	client.Close()

	srv := NewServer(mockHub, nil, model.Instance{}, mockLog)

	mockLog.EXPECT().Debug("Client connected: client-1")
	mockLog.EXPECT().Debug("Client client-1 disconnected before subscribing", "error", io.EOF)

	srv.handleConnection(t.Context(), conn)
}

func Test_Server_writePump(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLog := NewMockLogger(ctrl)

	srv := NewServer(nil, nil, model.Instance{}, mockLog)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	queued := make(chan model.LogLine, 2)
	queued <- model.LogLine{Service: "api", Message: "first"}

	queued <- model.LogLine{Service: "web", Message: "second"}

	close(queued)

	pending := make(chan model.LogLine, 1)
	pending <- model.LogLine{Service: "api", Message: "first"}

	idle := make(chan model.LogLine)

	frames := marshalLine(t, LogMessage{Type: MessageLog, Service: "api", Message: "first"}) + "\n" +
		marshalLine(t, LogMessage{Type: MessageLog, Service: "web", Message: "second"}) + "\n"

	piped := func() (net.Conn, <-chan string) {
		client, server := net.Pipe()
		received := make(chan string, 1)

		go func() {
			data, _ := io.ReadAll(client)
			received <- string(data)
		}()

		return server, received
	}

	tests := []struct {
		name     string
		before   func() (net.Conn, <-chan string)
		lines    <-chan model.LogLine
		expected string
	}{
		{
			name:     "writes a frame per line until the queue closes",
			before:   piped,
			lines:    queued,
			expected: frames,
		},
		{
			name: "stops when the connection is closed",
			before: func() (net.Conn, <-chan string) {
				mockLog.EXPECT().Debug("Client client-1 disconnected", "error", io.ErrClosedPipe)

				conn, received := piped()
				conn.Close()

				return conn, received
			},
			lines: pending,
		},
		{
			name: "stops when the context is cancelled",
			before: func() (net.Conn, <-chan string) {
				cancel()

				return piped()
			},
			lines: idle,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conn, received := tt.before()

			srv.writePump(ctx, conn, "client-1", tt.lines)
			conn.Close()

			assert.Equal(t, tt.expected, <-received)
		})
	}
}

func Test_Server_writePump_WriteError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLog := NewMockLogger(ctrl)

	fds, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
	require.NoError(t, err)
	require.NoError(t, syscall.Close(fds[1]))

	file := os.NewFile(uintptr(fds[0]), "server")
	defer file.Close()

	conn, err := net.FileConn(file)
	require.NoError(t, err)

	defer conn.Close()

	lines := make(chan model.LogLine, 1)
	lines <- model.LogLine{Service: "api", Message: "hello"}

	srv := NewServer(nil, nil, model.Instance{}, mockLog)

	mockLog.EXPECT().Debug("Client client-1 disconnected", "error", gomock.Any())

	srv.writePump(t.Context(), conn, "client-1", lines)
}

func Test_Server_hello(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLog := NewMockLogger(ctrl)

	closed, _ := net.Pipe()
	closed.Close()

	fds, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
	require.NoError(t, err)
	require.NoError(t, syscall.Close(fds[1]))

	file := os.NewFile(uintptr(fds[0]), "server")
	defer file.Close()

	hungUp, err := net.FileConn(file)
	require.NoError(t, err)

	defer hungUp.Close()

	srv := NewServer(nil, nil, model.Instance{}, mockLog)

	tests := []struct {
		name   string
		before func()
		conn   net.Conn
	}{
		{
			name: "a closed connection fails the write deadline",
			before: func() {
				mockLog.EXPECT().Debug("Failed to set write deadline for client-1", "error", io.ErrClosedPipe)
			},
			conn: closed,
		},
		{
			name: "a peer that hung up fails the write",
			before: func() {
				mockLog.EXPECT().Debug("Failed to send status to client-1", "error", gomock.Any())
			},
			conn: hungUp,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.before()

			srv.hello(tt.conn, "client-1", model.ReplayOptions{})
		})
	}
}
