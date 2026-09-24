package logsocket

import (
	"bufio"
	"context"
	"encoding/json"
	"net"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"fuku/internal/adapters/instance"
	"fuku/internal/contracts"
	"fuku/internal/model"
)

func Test_NewClient(t *testing.T) {
	identity := model.Instance{Fingerprint: "3f2a9c1d8b4e6072"}

	c := NewClient(identity)

	require.NotNil(t, c)
	assert.Equal(t, identity.Fingerprint, c.fingerprint)
	assert.Nil(t, c.conn)
}

func Test_Client_Connect(t *testing.T) {
	running := startScriptedServer(t)

	tests := []struct {
		name        string
		fingerprint string
		expect      error
	}{
		{
			name:        "connects to the running instance",
			fingerprint: running,
		},
		{
			name:        "no instance for the project",
			fingerprint: instance.Fingerprint("/Users/dev/projects/not-running"),
			expect:      contracts.ErrNoInstanceRunning,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewClient(model.Instance{Fingerprint: tt.fingerprint})

			err := c.Connect()

			require.ErrorIs(t, err, tt.expect)
			require.NoError(t, c.Close())
		})
	}
}

func Test_Client_Connect_DeadSocket(t *testing.T) {
	fingerprint := instance.Fingerprint("/Users/dev/projects/dead-socket")
	socketPath := instance.SocketPath(instance.SocketDir, fingerprint)

	listener, err := net.Listen("unix", socketPath)
	require.NoError(t, err)

	listener.Close()

	createStaleSocket(t, socketPath)

	defer os.Remove(socketPath)

	c := NewClient(model.Instance{Fingerprint: fingerprint})

	err = c.Connect()

	require.ErrorContains(t, err, "failed to connect to socket")
}

func Test_Client_Subscribe(t *testing.T) {
	fingerprint := startScriptedServer(t)

	connected := func(t *testing.T) *Client {
		c := NewClient(model.Instance{Fingerprint: fingerprint})
		require.NoError(t, c.Connect())

		t.Cleanup(func() { c.Close() })

		return c
	}

	tests := []struct {
		name     string
		before   func(t *testing.T) *Client
		services []string
		expect   error
	}{
		{
			name:     "with services",
			before:   connected,
			services: []string{"api", "web"},
		},
		{
			name:     "empty services",
			before:   connected,
			services: []string{},
		},
		{
			name: "closed connection",
			before: func(t *testing.T) *Client {
				c := connected(t)
				require.NoError(t, c.Close())

				return c
			},
			services: []string{"api"},
			expect:   net.ErrClosed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := tt.before(t)

			err := c.Subscribe(tt.services, model.ReplayOptions{})

			require.ErrorIs(t, err, tt.expect)
		})
	}
}

func Test_Client_Stream_ReceivesStatusAndLogs(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockHandler := NewMockHandler(ctrl)

	status := marshalLine(t, StatusMessage{Type: MessageStatus, Version: "0.17.0", Profile: "default", Services: []string{"api", "web"}})
	line := marshalLine(t, LogMessage{Type: MessageLog, Service: "api", Message: "hello from api"})
	fingerprint := startScriptedServer(t, status, line)

	c := NewClient(model.Instance{Fingerprint: fingerprint})
	require.NoError(t, c.Connect())

	defer c.Close()

	require.NoError(t, c.Subscribe(nil, model.ReplayOptions{}))

	gomock.InOrder(
		mockHandler.EXPECT().HandleStatus(contracts.LogStatus{Version: "0.17.0", Profile: "default", Services: []string{"api", "web"}}).Return(nil),
		mockHandler.EXPECT().HandleLog(model.LogLine{Service: "api", Message: "hello from api"}),
	)

	err := c.Stream(t.Context(), mockHandler)

	require.NoError(t, err)
}

func Test_Client_Stream_ContextCancellation(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockHandler := NewMockHandler(ctrl)

	clientConn, serverConn := net.Pipe()
	defer serverConn.Close()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	c := &Client{conn: clientConn}

	err := c.Stream(ctx, mockHandler)

	require.NoError(t, err)
}

func Test_Client_Stream_Frames(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockHandler := NewMockHandler(ctrl)

	tests := []struct {
		name   string
		before func()
		lines  []string
	}{
		{
			name:   "end of stream returns nil",
			before: func() {},
		},
		{
			name: "skips invalid JSON",
			before: func() {
				mockHandler.EXPECT().HandleLog(model.LogLine{Service: "api", Message: "valid message"})
			},
			lines: []string{"not json", `{"type":"log","service":"api","message":"valid message"}`},
		},
		{
			name: "skips unknown message types",
			before: func() {
				mockHandler.EXPECT().HandleLog(model.LogLine{Service: "api", Message: "real message"})
			},
			lines: []string{`{"type":"unknown","data":"something"}`, `{"type":"log","service":"api","message":"real message"}`},
		},
		{
			name: "skips a malformed status",
			before: func() {
				mockHandler.EXPECT().HandleLog(model.LogLine{Service: "api", Message: "after bad status"})
			},
			lines: []string{`{"type":"status","services":"not-an-array"}`, `{"type":"log","service":"api","message":"after bad status"}`},
		},
		{
			name: "skips a malformed log",
			before: func() {
				mockHandler.EXPECT().HandleLog(model.LogLine{Service: "api", Message: "after bad log"})
			},
			lines: []string{`{"type":"log","service":123}`, `{"type":"log","service":"api","message":"after bad log"}`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.before()

			clientConn, serverConn := net.Pipe()

			go func() {
				defer serverConn.Close()

				for _, line := range tt.lines {
					serverConn.Write([]byte(line + "\n"))
				}
			}()

			c := &Client{conn: clientConn}

			err := c.Stream(t.Context(), mockHandler)

			require.NoError(t, err)
		})
	}
}

func Test_Client_Stream_ReadError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockHandler := NewMockHandler(ctrl)

	clientConn, serverConn := net.Pipe()
	defer serverConn.Close()

	err := clientConn.SetReadDeadline(time.Now().Add(10 * time.Millisecond))
	require.NoError(t, err)

	c := &Client{conn: clientConn}

	err = c.Stream(t.Context(), mockHandler)
	require.Error(t, err)
	assert.ErrorContains(t, err, "failed to read from socket")
}

func Test_Client_Close_WithoutConnection(t *testing.T) {
	c := NewClient(model.Instance{})

	err := c.Close()
	require.NoError(t, err)
}

func Test_Client_Stream_BoundedReadAcknowledgement(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockHandler := NewMockHandler(ctrl)

	requested := 2
	different := 5

	logLine := func(t *testing.T) string {
		t.Helper()

		return marshalLine(t, LogMessage{Type: MessageLog, Service: "api", Message: "hello from api"})
	}

	statusLine := func(t *testing.T, tail *int, noFollow bool) string {
		t.Helper()

		return marshalLine(t, StatusMessage{
			Type:          MessageStatus,
			Version:       "0.17.0",
			Profile:       "default",
			Services:      []string{"api"},
			ReplayOptions: model.ReplayOptions{Tail: tail, NoFollow: noFollow},
		})
	}

	accepted := func() {
		gomock.InOrder(
			mockHandler.EXPECT().HandleStatus(contracts.LogStatus{Version: "0.17.0", Profile: "default", Services: []string{"api"}}).Return(nil),
			mockHandler.EXPECT().HandleLog(model.LogLine{Service: "api", Message: "hello from api"}),
		)
	}

	tests := []struct {
		name          string
		before        func()
		services      []string
		requested     model.ReplayOptions
		lines         func(t *testing.T) []string
		expectedError error
	}{
		{
			name:      "exact echo is accepted",
			before:    accepted,
			requested: model.ReplayOptions{Tail: &requested, NoFollow: true},
			lines: func(t *testing.T) []string {
				return []string{statusLine(t, &requested, true), logLine(t)}
			},
		},
		{
			name:      "missing echo is rejected",
			before:    func() {},
			requested: model.ReplayOptions{Tail: &requested, NoFollow: true},
			lines: func(t *testing.T) []string {
				return []string{statusLine(t, nil, false), logLine(t)}
			},
			expectedError: contracts.ErrBoundedReadNotSupported,
		},
		{
			name:      "different tail is rejected",
			before:    func() {},
			requested: model.ReplayOptions{Tail: &requested},
			lines: func(t *testing.T) []string {
				return []string{statusLine(t, &different, false), logLine(t)}
			},
			expectedError: contracts.ErrBoundedReadNotSupported,
		},
		{
			name:      "missing no-follow echo is rejected",
			before:    func() {},
			requested: model.ReplayOptions{NoFollow: true},
			lines: func(t *testing.T) []string {
				return []string{statusLine(t, nil, false), logLine(t)}
			},
			expectedError: contracts.ErrBoundedReadNotSupported,
		},
		{
			name:      "log before status is rejected",
			before:    func() {},
			requested: model.ReplayOptions{Tail: &requested},
			lines: func(t *testing.T) []string {
				return []string{logLine(t), statusLine(t, &requested, false)}
			},
			expectedError: contracts.ErrBoundedReadNotSupported,
		},
		{
			name:      "invalid JSON before status is rejected",
			before:    func() {},
			requested: model.ReplayOptions{Tail: &requested},
			lines: func(t *testing.T) []string {
				return []string{"not json", statusLine(t, &requested, false)}
			},
			expectedError: contracts.ErrBoundedReadNotSupported,
		},
		{
			name:      "unknown message type before status is rejected",
			before:    func() {},
			requested: model.ReplayOptions{Tail: &requested},
			lines: func(t *testing.T) []string {
				return []string{`{"type":"heartbeat"}`, statusLine(t, &requested, false)}
			},
			expectedError: contracts.ErrBoundedReadNotSupported,
		},
		{
			name:      "malformed status before status is rejected",
			before:    func() {},
			requested: model.ReplayOptions{Tail: &requested},
			lines: func(t *testing.T) []string {
				return []string{`{"type":"status","tail":"two"}`, statusLine(t, &requested, false)}
			},
			expectedError: contracts.ErrBoundedReadNotSupported,
		},
		{
			name:      "end of stream before status is rejected",
			before:    func() {},
			requested: model.ReplayOptions{NoFollow: true},
			lines: func(t *testing.T) []string {
				return nil
			},
			expectedError: contracts.ErrBoundedReadNotSupported,
		},
		{
			name:     "unbounded request accepts a status without echo",
			before:   accepted,
			services: []string{"api"},
			lines: func(t *testing.T) []string {
				return []string{statusLine(t, nil, false), logLine(t)}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.before()

			fingerprint := startScriptedServer(t, tt.lines(t)...)

			c := NewClient(model.Instance{Fingerprint: fingerprint})
			require.NoError(t, c.Connect())

			defer c.Close()

			require.NoError(t, c.Subscribe(tt.services, tt.requested))

			err := c.Stream(t.Context(), mockHandler)

			require.ErrorIs(t, err, tt.expectedError)
		})
	}
}

func Test_Client_Stream_StatusError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockHandler := NewMockHandler(ctrl)

	requested := 2

	mismatched := func() {
		mockHandler.EXPECT().HandleStatus(contracts.LogStatus{Profile: "default"}).Return(contracts.ErrProfileMismatch)
	}

	tests := []struct {
		name      string
		before    func()
		services  []string
		requested model.ReplayOptions
		status    StatusMessage
	}{
		{
			name:      "acknowledgement path",
			before:    mismatched,
			requested: model.ReplayOptions{Tail: &requested, NoFollow: true},
			status: StatusMessage{
				Type:          MessageStatus,
				Profile:       "default",
				ReplayOptions: model.ReplayOptions{Tail: &requested, NoFollow: true},
			},
		},
		{
			name:     "plain path",
			before:   mismatched,
			services: []string{"api"},
			status:   StatusMessage{Type: MessageStatus, Profile: "default"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.before()

			logLine := marshalLine(t, LogMessage{Type: MessageLog, Service: "api", Message: "hello from api"})
			fingerprint := startScriptedServer(t, marshalLine(t, tt.status), logLine)

			c := NewClient(model.Instance{Fingerprint: fingerprint})
			require.NoError(t, c.Connect())

			defer c.Close()

			require.NoError(t, c.Subscribe(tt.services, tt.requested))

			err := c.Stream(t.Context(), mockHandler)

			require.ErrorIs(t, err, contracts.ErrProfileMismatch)
		})
	}
}

// startScriptedServer answers one subscription on its own project socket with the lines and returns the fingerprint
func startScriptedServer(t *testing.T, lines ...string) string {
	t.Helper()

	identity := testIdentity(t)
	socketPath := instance.SocketPath(instance.SocketDir, identity.Fingerprint)

	listener, err := net.Listen("unix", socketPath)
	require.NoError(t, err)

	t.Cleanup(func() {
		listener.Close()
	})

	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}

		defer conn.Close()

		if _, err := bufio.NewReader(conn).ReadBytes('\n'); err != nil {
			return
		}

		for _, line := range lines {
			conn.Write([]byte(line + "\n"))
		}
	}()

	return identity.Fingerprint
}

// marshalLine renders a protocol message as one wire line
func marshalLine(t *testing.T, msg any) string {
	t.Helper()

	data, err := json.Marshal(msg)
	require.NoError(t, err)

	return string(data)
}
