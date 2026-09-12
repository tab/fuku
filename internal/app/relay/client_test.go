package relay

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fuku/internal/app/errors"
	"fuku/internal/config"
	"fuku/internal/config/logger"
)

type testHandler struct {
	mu       sync.Mutex
	statuses []StatusMessage
	logs     []LogMessage
}

func (h *testHandler) HandleStatus(msg StatusMessage) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.statuses = append(h.statuses, msg)
}

func (h *testHandler) HandleLog(msg LogMessage) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.logs = append(h.logs, msg)
}

func (h *testHandler) getStatuses() []StatusMessage {
	h.mu.Lock()
	defer h.mu.Unlock()

	result := make([]StatusMessage, len(h.statuses))
	copy(result, h.statuses)

	return result
}

func (h *testHandler) getLogs() []LogMessage {
	h.mu.Lock()
	defer h.mu.Unlock()

	result := make([]LogMessage, len(h.logs))
	copy(result, h.logs)

	return result
}

func Test_NewClient(t *testing.T) {
	c := NewClient()

	assert.NotNil(t, c)
}

func Test_Client_Connect_Success(t *testing.T) {
	srv := newTestServer(t)
	profile := uniqueProfile(t)

	cancel := startTestServer(t, srv, profile, []string{"api"})
	defer srv.Stop()
	defer cancel()

	c := NewClient()

	err := c.Connect(srv.SocketPath())
	require.NoError(t, err)

	err = c.Close()
	require.NoError(t, err)
}

func Test_Client_Connect_SocketNotFound(t *testing.T) {
	c := NewClient()

	err := c.Connect("/tmp/nonexistent-socket.sock")
	require.Error(t, err)
	assert.True(t, errors.Is(err, errors.ErrFailedToConnectSocket))
}

func Test_Client_Subscribe(t *testing.T) {
	tests := []struct {
		name     string
		services []string
	}{
		{
			name:     "with services",
			services: []string{"api", "web"},
		},
		{
			name:     "empty services",
			services: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := newTestServer(t)
			profile := uniqueProfile(t)

			cancel := startTestServer(t, srv, profile, []string{"api", "web"})
			defer srv.Stop()
			defer cancel()

			c := NewClient()
			err := c.Connect(srv.SocketPath())
			require.NoError(t, err)

			defer c.Close()

			err = c.Subscribe(SubscribeOptions{Services: tt.services})
			require.NoError(t, err)
		})
	}
}

func Test_Client_Subscribe_ClosedConnection(t *testing.T) {
	srv := newTestServer(t)
	profile := uniqueProfile(t)

	cancel := startTestServer(t, srv, profile, []string{"api"})
	defer srv.Stop()
	defer cancel()

	c := NewClient()
	err := c.Connect(srv.SocketPath())
	require.NoError(t, err)

	err = c.Close()
	require.NoError(t, err)

	err = c.Subscribe(SubscribeOptions{Services: []string{"api"}})
	require.Error(t, err)
	assert.True(t, errors.Is(err, errors.ErrFailedToWriteSocket))
}

func Test_Client_Stream_ReceivesLogMessages(t *testing.T) {
	srv := newTestServer(t)
	profile := uniqueProfile(t)

	cancel := startTestServer(t, srv, profile, []string{"api"})
	defer srv.Stop()
	defer cancel()

	c := NewClient()
	err := c.Connect(srv.SocketPath())
	require.NoError(t, err)

	defer c.Close()

	err = c.Subscribe(SubscribeOptions{})
	require.NoError(t, err)

	handler := &testHandler{}
	ctx, cancel := context.WithCancel(t.Context())

	defer cancel()

	streamDone := make(chan error, 1)

	go func() {
		streamDone <- c.Stream(ctx, handler)
	}()

	//nolint:forbidigo // allow stream goroutine to start and connect
	time.Sleep(50 * time.Millisecond)

	srv.Broadcast("api", "hello from api")

	//nolint:forbidigo // allow broadcast to propagate through socket
	time.Sleep(100 * time.Millisecond)

	cancel()

	select {
	case err := <-streamDone:
		require.NoError(t, err)
	case <-time.After(1 * time.Second):
		t.Fatal("Stream did not return after context cancellation")
	}

	logs := handler.getLogs()
	require.GreaterOrEqual(t, len(logs), 1)
	assert.Equal(t, "api", logs[0].Service)
	assert.Equal(t, "hello from api", logs[0].Message)
}

func Test_Client_Stream_ContextCancellation(t *testing.T) {
	srv := newTestServer(t)
	profile := uniqueProfile(t)

	cancel := startTestServer(t, srv, profile, []string{"api"})
	defer srv.Stop()
	defer cancel()

	c := NewClient()
	err := c.Connect(srv.SocketPath())
	require.NoError(t, err)

	defer c.Close()

	err = c.Subscribe(SubscribeOptions{})
	require.NoError(t, err)

	handler := &testHandler{}
	ctx, cancel := context.WithCancel(t.Context())

	streamDone := make(chan error, 1)

	go func() {
		streamDone <- c.Stream(ctx, handler)
	}()

	//nolint:forbidigo // allow stream goroutine to start and connect
	time.Sleep(50 * time.Millisecond)

	cancel()

	select {
	case err := <-streamDone:
		require.NoError(t, err)
	case <-time.After(1 * time.Second):
		t.Fatal("Stream did not return after context cancellation")
	}
}

func Test_Client_Stream_EOF_ReturnsNil(t *testing.T) {
	//nolint:usetesting // socket path length exceeds macOS limit with t.TempDir
	tmpDir, err := os.MkdirTemp("/tmp", "fuku-test-")
	require.NoError(t, err)

	defer os.RemoveAll(tmpDir)

	socketPath := tmpDir + "/test.sock"

	listener, err := net.Listen("unix", socketPath)
	require.NoError(t, err)

	defer listener.Close()

	serverDone := make(chan struct{})

	go func() {
		defer close(serverDone)

		conn, err := listener.Accept()
		if err != nil {
			return
		}

		//nolint:forbidigo // simulate delayed server close for EOF test
		time.Sleep(50 * time.Millisecond)
		conn.Close()
	}()

	c := NewClient()
	err = c.Connect(socketPath)
	require.NoError(t, err)

	defer c.Close()

	handler := &testHandler{}

	err = c.Stream(t.Context(), handler)
	require.NoError(t, err)

	<-serverDone
}

func Test_Client_Stream_SkipsInvalidJSON(t *testing.T) {
	//nolint:usetesting // socket path length exceeds macOS limit with t.TempDir
	tmpDir, err := os.MkdirTemp("/tmp", "fuku-test-")
	require.NoError(t, err)

	defer os.RemoveAll(tmpDir)

	socketPath := tmpDir + "/test.sock"

	listener, err := net.Listen("unix", socketPath)
	require.NoError(t, err)

	defer listener.Close()

	serverDone := make(chan struct{})

	go func() {
		defer close(serverDone)

		conn, err := listener.Accept()
		if err != nil {
			return
		}

		defer conn.Close()

		conn.Write([]byte("not json\n"))

		logMsg := LogMessage{
			Type:    MessageLog,
			Service: "api",
			Message: "valid message",
		}
		data, _ := json.Marshal(logMsg)
		data = append(data, '\n')
		conn.Write(data)

		//nolint:forbidigo // allow client to read before server closes
		time.Sleep(100 * time.Millisecond)
	}()

	c := NewClient()
	err = c.Connect(socketPath)
	require.NoError(t, err)

	defer c.Close()

	handler := &testHandler{}

	err = c.Stream(t.Context(), handler)
	require.NoError(t, err)

	<-serverDone

	logs := handler.getLogs()
	require.Len(t, logs, 1)
	assert.Equal(t, "valid message", logs[0].Message)
}

func Test_Client_Stream_ReceivesStatusMessage(t *testing.T) {
	srv := newTestServer(t)
	profile := uniqueProfile(t)

	cancel := startTestServer(t, srv, profile, []string{"api", "web"})
	defer srv.Stop()
	defer cancel()

	c := NewClient()
	err := c.Connect(srv.SocketPath())
	require.NoError(t, err)

	defer c.Close()

	err = c.Subscribe(SubscribeOptions{})
	require.NoError(t, err)

	handler := &testHandler{}
	ctx, cancel := context.WithCancel(t.Context())

	defer cancel()

	streamDone := make(chan error, 1)

	go func() {
		streamDone <- c.Stream(ctx, handler)
	}()

	//nolint:forbidigo // allow stream to receive status message
	time.Sleep(100 * time.Millisecond)

	cancel()

	select {
	case err := <-streamDone:
		require.NoError(t, err)
	case <-time.After(1 * time.Second):
		t.Fatal("Stream did not return")
	}

	statuses := handler.getStatuses()
	require.Len(t, statuses, 1)
	assert.Equal(t, MessageStatus, statuses[0].Type)
	assert.Equal(t, profile, statuses[0].Profile)
	assert.Equal(t, []string{"api", "web"}, statuses[0].Services)
}

func Test_Client_Stream_SkipsNonLogMessages(t *testing.T) {
	//nolint:usetesting // socket path length exceeds macOS limit with t.TempDir
	tmpDir, err := os.MkdirTemp("/tmp", "fuku-test-")
	require.NoError(t, err)

	defer os.RemoveAll(tmpDir)

	socketPath := tmpDir + "/test.sock"

	listener, err := net.Listen("unix", socketPath)
	require.NoError(t, err)

	defer listener.Close()

	serverDone := make(chan struct{})

	go func() {
		defer close(serverDone)

		conn, err := listener.Accept()
		if err != nil {
			return
		}

		defer conn.Close()

		unknown := `{"type":"unknown","data":"something"}` + "\n"
		conn.Write([]byte(unknown))

		logMsg := LogMessage{
			Type:    MessageLog,
			Service: "api",
			Message: "real message",
		}
		data, _ := json.Marshal(logMsg)
		data = append(data, '\n')
		conn.Write(data)

		//nolint:forbidigo // allow client to read before server closes
		time.Sleep(100 * time.Millisecond)
	}()

	c := NewClient()
	err = c.Connect(socketPath)
	require.NoError(t, err)

	defer c.Close()

	handler := &testHandler{}

	err = c.Stream(t.Context(), handler)
	require.NoError(t, err)

	<-serverDone

	logs := handler.getLogs()
	require.Len(t, logs, 1)
	assert.Equal(t, "real message", logs[0].Message)
}

func Test_Client_Close_WithConnection(t *testing.T) {
	cfg := config.DefaultConfig()
	log := logger.NewLoggerWithOutput(cfg, io.Discard)

	srv := &Server{
		bufferSize:  cfg.Logs.Buffer,
		historySize: cfg.Logs.History,
		hub:         NewHub(cfg.Logs.Buffer, cfg.Logs.History, log),
		log:         log,
	}

	profile := uniqueProfile(t)

	srv.profile = profile
	srv.services = []string{"api"}

	ctx, cancel := context.WithCancel(t.Context())

	err := srv.start(ctx)
	require.NoError(t, err)

	defer srv.Stop()
	defer cancel()

	c := NewClient()
	err = c.Connect(srv.SocketPath())
	require.NoError(t, err)

	err = c.Close()
	require.NoError(t, err)
}

func Test_Client_Close_WithoutConnection(t *testing.T) {
	c := NewClient()

	err := c.Close()
	require.NoError(t, err)
}

func Test_Client_Stream_SkipsInvalidStatusJSON(t *testing.T) {
	clientConn, serverConn := net.Pipe()

	go func() {
		defer serverConn.Close()

		badStatus := `{"type":"status","services":"not-an-array"}` + "\n"
		serverConn.Write([]byte(badStatus))

		logMsg := LogMessage{
			Type:    MessageLog,
			Service: "api",
			Message: "after bad status",
		}
		data, _ := json.Marshal(logMsg)
		data = append(data, '\n')
		serverConn.Write(data)

		//nolint:forbidigo // allow client to read before server closes
		time.Sleep(100 * time.Millisecond)
	}()

	c := &client{conn: clientConn}
	handler := &testHandler{}

	err := c.Stream(t.Context(), handler)
	require.NoError(t, err)

	logs := handler.getLogs()
	require.Len(t, logs, 1)
	assert.Equal(t, "after bad status", logs[0].Message)
}

func Test_Client_Stream_SkipsInvalidLogJSON(t *testing.T) {
	clientConn, serverConn := net.Pipe()

	go func() {
		defer serverConn.Close()

		badLog := `{"type":"log","service":123}` + "\n"
		serverConn.Write([]byte(badLog))

		logMsg := LogMessage{
			Type:    MessageLog,
			Service: "api",
			Message: "after bad log",
		}
		data, _ := json.Marshal(logMsg)
		data = append(data, '\n')
		serverConn.Write(data)

		//nolint:forbidigo // allow client to read before server closes
		time.Sleep(100 * time.Millisecond)
	}()

	c := &client{conn: clientConn}
	handler := &testHandler{}

	err := c.Stream(t.Context(), handler)
	require.NoError(t, err)

	logs := handler.getLogs()
	require.Len(t, logs, 1)
	assert.Equal(t, "after bad log", logs[0].Message)
}

func Test_Client_Stream_ReadError(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer serverConn.Close()

	err := clientConn.SetReadDeadline(time.Now().Add(10 * time.Millisecond))
	require.NoError(t, err)

	c := &client{conn: clientConn}
	handler := &testHandler{}

	err = c.Stream(t.Context(), handler)
	require.Error(t, err)
	assert.True(t, errors.Is(err, errors.ErrFailedToReadSocket))
}

func Test_Client_Stream_BoundedReadAcknowledgement(t *testing.T) {
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
			ReplayOptions: ReplayOptions{Tail: tail, NoFollow: noFollow},
		})
	}

	tests := []struct {
		name             string
		requested        SubscribeOptions
		lines            func(t *testing.T) []string
		expectedError    bool
		expectedStatuses int
		expectedLogs     int
	}{
		{
			name:      "exact echo is accepted",
			requested: SubscribeOptions{ReplayOptions: ReplayOptions{Tail: &requested, NoFollow: true}},
			lines: func(t *testing.T) []string {
				return []string{statusLine(t, &requested, true), logLine(t)}
			},
			expectedStatuses: 1,
			expectedLogs:     1,
		},
		{
			name:      "missing echo is rejected",
			requested: SubscribeOptions{ReplayOptions: ReplayOptions{Tail: &requested, NoFollow: true}},
			lines: func(t *testing.T) []string {
				return []string{statusLine(t, nil, false), logLine(t)}
			},
			expectedError: true,
		},
		{
			name:      "different tail is rejected",
			requested: SubscribeOptions{ReplayOptions: ReplayOptions{Tail: &requested}},
			lines: func(t *testing.T) []string {
				return []string{statusLine(t, &different, false), logLine(t)}
			},
			expectedError: true,
		},
		{
			name:      "missing no-follow echo is rejected",
			requested: SubscribeOptions{ReplayOptions: ReplayOptions{NoFollow: true}},
			lines: func(t *testing.T) []string {
				return []string{statusLine(t, nil, false), logLine(t)}
			},
			expectedError: true,
		},
		{
			name:      "log before status is rejected",
			requested: SubscribeOptions{ReplayOptions: ReplayOptions{Tail: &requested}},
			lines: func(t *testing.T) []string {
				return []string{logLine(t), statusLine(t, &requested, false)}
			},
			expectedError: true,
		},
		{
			name:      "invalid JSON before status is rejected",
			requested: SubscribeOptions{ReplayOptions: ReplayOptions{Tail: &requested}},
			lines: func(t *testing.T) []string {
				return []string{"not json", statusLine(t, &requested, false)}
			},
			expectedError: true,
		},
		{
			name:      "unknown message type before status is rejected",
			requested: SubscribeOptions{ReplayOptions: ReplayOptions{Tail: &requested}},
			lines: func(t *testing.T) []string {
				return []string{`{"type":"heartbeat"}`, statusLine(t, &requested, false)}
			},
			expectedError: true,
		},
		{
			name:      "malformed status before status is rejected",
			requested: SubscribeOptions{ReplayOptions: ReplayOptions{Tail: &requested}},
			lines: func(t *testing.T) []string {
				return []string{`{"type":"status","tail":"two"}`, statusLine(t, &requested, false)}
			},
			expectedError: true,
		},
		{
			name:      "end of stream before status is rejected",
			requested: SubscribeOptions{ReplayOptions: ReplayOptions{NoFollow: true}},
			lines: func(t *testing.T) []string {
				return nil
			},
			expectedError: true,
		},
		{
			name:      "unbounded request accepts a status without echo",
			requested: SubscribeOptions{Services: []string{"api"}},
			lines: func(t *testing.T) []string {
				return []string{statusLine(t, nil, false), logLine(t)}
			},
			expectedStatuses: 1,
			expectedLogs:     1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			socketPath := startScriptedServer(t, tt.lines(t)...)

			c := NewClient()
			require.NoError(t, c.Connect(socketPath))

			defer c.Close()

			require.NoError(t, c.Subscribe(tt.requested))

			handler := &testHandler{}
			err := c.Stream(t.Context(), handler)

			if tt.expectedError {
				require.ErrorIs(t, err, errors.ErrBoundedReadNotSupported)
			} else {
				require.NoError(t, err)
			}

			assert.Len(t, handler.getStatuses(), tt.expectedStatuses)
			assert.Len(t, handler.getLogs(), tt.expectedLogs)
		})
	}
}

// startScriptedServer accepts one connection, reads the subscribe line, writes the given lines and closes
func startScriptedServer(t *testing.T, lines ...string) string {
	t.Helper()

	//nolint:usetesting // socket path length exceeds macOS limit with t.TempDir
	tmpDir, err := os.MkdirTemp("/tmp", "fuku-test-")
	require.NoError(t, err)

	t.Cleanup(func() {
		os.RemoveAll(tmpDir)
	})

	socketPath := tmpDir + "/test.sock"

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

	return socketPath
}

// marshalLine renders a protocol message as one wire line
func marshalLine(t *testing.T, msg any) string {
	t.Helper()

	data, err := json.Marshal(msg)
	require.NoError(t, err)

	return string(data)
}
