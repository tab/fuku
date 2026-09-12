package relay

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"fuku/internal/app/bus"
	"fuku/internal/app/errors"
	"fuku/internal/app/instance"
	"fuku/internal/config"
	"fuku/internal/config/logger"
)

func uniqueProfile(t *testing.T) string {
	t.Helper()

	return fmt.Sprintf("test-%d", time.Now().UnixNano())
}

func newTestServer(t *testing.T) *Server {
	t.Helper()

	cfg := config.DefaultConfig()
	log := logger.NewLoggerWithOutput(cfg, io.Discard)

	identity := instance.Identity{
		ID:          "1f0c6e4a-2b8d-4c3e-9a7f-5d6b8c0e1a24",
		Project:     "/Users/dev/projects/shop",
		Fingerprint: instance.Fingerprint("/Users/dev/projects/shop"),
	}

	return &Server{
		instanceID:  identity.ID,
		fingerprint: identity.Fingerprint,
		bufferSize:  cfg.Logs.Buffer,
		historySize: cfg.Logs.History,
		hub:         NewHub(cfg.Logs.Buffer, cfg.Logs.History, log),
		log:         log,
	}
}

func startTestServer(t *testing.T, srv *Server, profile string, services []string) context.CancelFunc {
	t.Helper()

	srv.profile = profile
	srv.services = services

	ctx, cancel := context.WithCancel(t.Context())

	err := srv.start(ctx)
	require.NoError(t, err)

	return cancel
}

func Test_NewServer(t *testing.T) {
	cfg := config.DefaultConfig()
	log := logger.NewLoggerWithOutput(cfg, io.Discard)

	identity := instance.Identity{
		ID:          "1f0c6e4a-2b8d-4c3e-9a7f-5d6b8c0e1a24",
		Project:     "/Users/dev/projects/shop",
		Fingerprint: instance.Fingerprint("/Users/dev/projects/shop"),
	}

	s := NewServer(cfg, bus.NoOp(), identity, log)

	assert.NotNil(t, s)
	assert.Equal(t, identity.ID, s.instanceID)
	assert.Equal(t, identity.Fingerprint, s.fingerprint)
}

func Test_Server_Subscribe(t *testing.T) {
	cfg := config.DefaultConfig()
	log := logger.NewLoggerWithOutput(cfg, io.Discard)

	identity := instance.Identity{
		ID:          "1f0c6e4a-2b8d-4c3e-9a7f-5d6b8c0e1a24",
		Project:     "/Users/dev/projects/shop",
		Fingerprint: instance.Fingerprint("/Users/dev/projects/shop"),
	}

	s := NewServer(cfg, bus.NoOp(), identity, log)

	s.Subscribe(t.Context())

	assert.NotNil(t, s.ch)
	assert.NotNil(t, s.cancelSub)
}

func Test_Server_Run_ActivatesOnProfileResolved(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Logs.Buffer = 10

	b := bus.NewBus(cfg, nil, nil)
	defer b.Close()

	log := logger.NewLoggerWithOutput(cfg, io.Discard)

	identity := instance.Identity{
		ID:          "1f0c6e4a-2b8d-4c3e-9a7f-5d6b8c0e1a24",
		Project:     "/Users/dev/projects/shop",
		Fingerprint: instance.Fingerprint("/Users/dev/projects/shop"),
	}

	s := NewServer(cfg, b, identity, log)

	ctx, cancel := context.WithCancel(t.Context())
	s.Subscribe(ctx)

	tierServices := []bus.Service{{ID: "test-id", Name: "api"}}
	b.Publish(bus.Message{
		Type: bus.EventProfileResolved,
		Data: bus.ProfileResolved{
			Profile: "test",
			Tiers:   []bus.Tier{{Name: "default", Services: tierServices}},
		},
	})

	done := make(chan struct{})

	go func() {
		s.Run(ctx)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run did not return after activation")
	}

	assert.Equal(t, "test", s.profile)

	cancel()
	s.Stop()
}

func Test_Server_Run_ContextCancelled(t *testing.T) {
	cfg := config.DefaultConfig()
	log := logger.NewLoggerWithOutput(cfg, io.Discard)

	identity := instance.Identity{
		ID:          "1f0c6e4a-2b8d-4c3e-9a7f-5d6b8c0e1a24",
		Project:     "/Users/dev/projects/shop",
		Fingerprint: instance.Fingerprint("/Users/dev/projects/shop"),
	}

	s := NewServer(cfg, bus.NoOp(), identity, log)

	ctx, cancel := context.WithCancel(t.Context())
	s.Subscribe(ctx)

	done := make(chan struct{})

	go func() {
		s.Run(ctx)
		close(done)
	}()

	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run did not return after context cancel")
	}
}

func Test_Server_SocketPath(t *testing.T) {
	cfg := config.DefaultConfig()
	log := logger.NewLoggerWithOutput(cfg, io.Discard)

	identity := instance.Identity{
		ID:          "1f0c6e4a-2b8d-4c3e-9a7f-5d6b8c0e1a24",
		Project:     "/Users/dev/projects/shop",
		Fingerprint: instance.Fingerprint("/Users/dev/projects/shop"),
	}

	s := NewServer(cfg, bus.NoOp(), identity, log)

	assert.Empty(t, s.SocketPath())
}

func Test_Server_StartStop(t *testing.T) {
	srv := newTestServer(t)
	profile := uniqueProfile(t)

	cancel := startTestServer(t, srv, profile, []string{"api", "web"})

	assert.NotEmpty(t, srv.SocketPath())
	assert.FileExists(t, srv.SocketPath())

	cancel()
	srv.Stop()
}

func Test_Server_Broadcast_Running(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockHub := NewMockHub(ctrl)
	mockHub.EXPECT().Broadcast("api", "hello").Times(1)

	srv := &Server{
		hub: mockHub,
		log: testLogger(),
	}
	srv.running.Store(true)

	srv.Broadcast("api", "hello")
}

func Test_Server_Broadcast_NotRunning(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockHub := NewMockHub(ctrl)

	srv := &Server{
		hub: mockHub,
		log: testLogger(),
	}
	srv.running.Store(false)

	srv.Broadcast("api", "hello")
}

func Test_Server_Start_ActiveSocket_ReturnsError(t *testing.T) {
	profile := uniqueProfile(t)
	socketPath := SocketPathForProfile(config.SocketDir, profile)

	listener, err := net.Listen("unix", socketPath)
	require.NoError(t, err)

	defer listener.Close()
	defer os.Remove(socketPath)

	srv := newTestServer(t)
	srv.profile = profile
	srv.services = []string{"api"}

	err = srv.start(t.Context())
	require.Error(t, err)
	assert.True(t, errors.Is(err, errors.ErrSocketAlreadyInUse))
}

func Test_Server_Start_RecoverFromStaleSocket(t *testing.T) {
	profile := uniqueProfile(t)
	socketPath := SocketPathForProfile(config.SocketDir, profile)

	err := os.WriteFile(socketPath, []byte("stale"), 0600)
	require.NoError(t, err)

	defer os.Remove(socketPath)

	srv := newTestServer(t)

	cancel := startTestServer(t, srv, profile, []string{"api"})

	cancel()
	srv.Stop()
}

func Test_Server_Stop_NotRunning(t *testing.T) {
	srv := &Server{
		log: testLogger(),
	}

	srv.Stop()
}

func Test_Server_HandleConnection_SuccessfulFlow(t *testing.T) {
	srv := newTestServer(t)
	profile := uniqueProfile(t)

	cancel := startTestServer(t, srv, profile, []string{"api", "web"})
	defer srv.Stop()
	defer cancel()

	conn, err := net.Dial("unix", srv.SocketPath())
	require.NoError(t, err)

	defer conn.Close()

	subReq := SubscribeRequest{
		Type:     MessageSubscribe,
		Services: []string{"api"},
	}

	data, err := json.Marshal(subReq)
	require.NoError(t, err)

	data = append(data, '\n')
	_, err = conn.Write(data)
	require.NoError(t, err)

	reader := bufio.NewReader(conn)

	line, err := reader.ReadBytes('\n')
	require.NoError(t, err)

	var status StatusMessage

	err = json.Unmarshal(line, &status)
	require.NoError(t, err)
	assert.Equal(t, MessageStatus, status.Type)
	assert.Equal(t, srv.instanceID, status.Instance)
	assert.Equal(t, srv.fingerprint, status.Fingerprint)
	assert.Equal(t, profile, status.Profile)
	assert.Equal(t, []string{"api", "web"}, status.Services)

	srv.Broadcast("api", "test log message")

	err = conn.SetReadDeadline(time.Now().Add(1 * time.Second))
	require.NoError(t, err)

	line, err = reader.ReadBytes('\n')
	require.NoError(t, err)

	var logMsg LogMessage

	err = json.Unmarshal(line, &logMsg)
	require.NoError(t, err)
	assert.Equal(t, MessageLog, logMsg.Type)
	assert.Equal(t, "api", logMsg.Service)
	assert.Equal(t, "test log message", logMsg.Message)
}

func Test_Server_HandleConnection_InvalidSubscribe(t *testing.T) {
	srv := newTestServer(t)
	profile := uniqueProfile(t)

	cancel := startTestServer(t, srv, profile, []string{"api"})
	defer srv.Stop()
	defer cancel()

	conn, err := net.Dial("unix", srv.SocketPath())
	require.NoError(t, err)

	defer conn.Close()

	_, err = conn.Write([]byte("not valid json\n"))
	require.NoError(t, err)

	err = conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	require.NoError(t, err)

	buf := make([]byte, 1)
	_, err = conn.Read(buf)
	assert.Error(t, err)
}

func Test_Server_HandleConnection_WrongMessageType(t *testing.T) {
	srv := newTestServer(t)
	profile := uniqueProfile(t)

	cancel := startTestServer(t, srv, profile, []string{"api"})
	defer srv.Stop()
	defer cancel()

	conn, err := net.Dial("unix", srv.SocketPath())
	require.NoError(t, err)

	defer conn.Close()

	wrongReq := SubscribeRequest{
		Type:     MessageLog,
		Services: []string{"api"},
	}
	data, err := json.Marshal(wrongReq)
	require.NoError(t, err)

	data = append(data, '\n')
	_, err = conn.Write(data)
	require.NoError(t, err)

	err = conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	require.NoError(t, err)

	buf := make([]byte, 1)
	_, err = conn.Read(buf)
	assert.Error(t, err)
}

func Test_Server_HandleConnection_ReplaysHistory(t *testing.T) {
	srv := newTestServer(t)
	profile := uniqueProfile(t)

	cancel := startTestServer(t, srv, profile, []string{"api"})
	defer srv.Stop()
	defer cancel()

	srv.Broadcast("api", "history-msg-1")
	srv.Broadcast("api", "history-msg-2")

	//nolint:forbidigo // allow broadcast to propagate through hub
	time.Sleep(50 * time.Millisecond)

	conn, err := net.Dial("unix", srv.SocketPath())
	require.NoError(t, err)

	defer conn.Close()

	subReq := SubscribeRequest{
		Type:     MessageSubscribe,
		Services: []string{},
	}
	data, err := json.Marshal(subReq)
	require.NoError(t, err)

	data = append(data, '\n')
	_, err = conn.Write(data)
	require.NoError(t, err)

	reader := bufio.NewReader(conn)

	err = conn.SetReadDeadline(time.Now().Add(1 * time.Second))
	require.NoError(t, err)

	line, err := reader.ReadBytes('\n')
	require.NoError(t, err)

	var status StatusMessage

	err = json.Unmarshal(line, &status)
	require.NoError(t, err)
	assert.Equal(t, MessageStatus, status.Type)

	replayed := make([]LogMessage, 0, 2)

	for range 2 {
		line, err = reader.ReadBytes('\n')
		require.NoError(t, err)

		var msg LogMessage

		err = json.Unmarshal(line, &msg)
		require.NoError(t, err)

		replayed = append(replayed, msg)
	}

	require.Len(t, replayed, 2)
	assert.Equal(t, "history-msg-1", replayed[0].Message)
	assert.Equal(t, "history-msg-2", replayed[1].Message)
}

func Test_Server_ClientQueueSizing(t *testing.T) {
	cfg := config.DefaultConfig()

	srv := &Server{
		bufferSize:  cfg.Logs.Buffer,
		historySize: cfg.Logs.History,
		log:         testLogger(),
	}

	conn := NewClientConn("test", srv.bufferSize+srv.historySize)
	assert.Equal(t, cfg.Logs.Buffer+cfg.Logs.History, cap(conn.SendChan))
}

func Test_Server_WritePump_ClientChannelClosed(t *testing.T) {
	srv := newTestServer(t)

	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	client := NewClientConn("test-1", 10)
	close(client.SendChan)

	srv.writePump(t.Context(), serverConn, client)
}

func Test_Server_WritePump_ContextCancelled(t *testing.T) {
	srv := newTestServer(t)

	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	client := NewClientConn("test-1", 10)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	srv.writePump(ctx, serverConn, client)
}

func Test_Server_WritePump_WriteError(t *testing.T) {
	//nolint:usetesting // socket path length exceeds macOS limit with t.TempDir
	tmpDir, err := os.MkdirTemp("/tmp", "fuku-test-")
	require.NoError(t, err)

	defer os.RemoveAll(tmpDir)

	socketPath := tmpDir + "/test.sock"

	listener, err := net.Listen("unix", socketPath)
	require.NoError(t, err)

	defer listener.Close()

	accepted := make(chan net.Conn, 1)

	go func() {
		conn, err := listener.Accept()
		if err == nil {
			accepted <- conn
		}
	}()

	dialed, err := net.Dial("unix", socketPath)
	require.NoError(t, err)

	serverSide := <-accepted

	dialed.Close()

	srv := newTestServer(t)

	client := NewClientConn("test-1", 10)
	client.SendChan <- LogMessage{Type: MessageLog, Service: "api", Message: "hello"}

	srv.writePump(t.Context(), serverSide, client)
	serverSide.Close()
}

func Test_Server_Hello_WriteDeadlineError(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	clientConn.Close()
	serverConn.Close()

	srv := newTestServer(t)
	srv.profile = "test"
	srv.services = []string{"api"}

	srv.hello(serverConn, NewClientConn("client-1", 1))
}

func Test_Server_Hello_WriteError(t *testing.T) {
	//nolint:usetesting // socket path length exceeds macOS limit with t.TempDir
	tmpDir, err := os.MkdirTemp("/tmp", "fuku-test-")
	require.NoError(t, err)

	defer os.RemoveAll(tmpDir)

	socketPath := tmpDir + "/test.sock"

	listener, err := net.Listen("unix", socketPath)
	require.NoError(t, err)

	defer listener.Close()

	accepted := make(chan net.Conn, 1)

	go func() {
		conn, err := listener.Accept()
		if err == nil {
			accepted <- conn
		}
	}()

	dialed, err := net.Dial("unix", socketPath)
	require.NoError(t, err)

	serverSide := <-accepted

	dialed.Close()

	srv := newTestServer(t)
	srv.profile = "test"
	srv.services = []string{"api"}

	srv.hello(serverSide, NewClientConn("client-1", 1))
	serverSide.Close()
}

func Test_Server_HandleConnection_ClientDisconnectsBeforeSubscribe(t *testing.T) {
	srv := newTestServer(t)
	profile := uniqueProfile(t)

	cancel := startTestServer(t, srv, profile, []string{"api"})
	defer srv.Stop()
	defer cancel()

	conn, err := net.Dial("unix", srv.SocketPath())
	require.NoError(t, err)

	conn.Close()

	//nolint:forbidigo // allow server goroutine to process the closed connection
	time.Sleep(50 * time.Millisecond)
}

func Test_Server_WritePump_SetWriteDeadlineError(t *testing.T) {
	srv := newTestServer(t)

	clientConn, serverConn := net.Pipe()
	clientConn.Close()
	serverConn.Close()

	client := NewClientConn("test-1", 10)
	client.SendChan <- LogMessage{Type: MessageLog, Service: "api", Message: "hello"}

	srv.writePump(t.Context(), serverConn, client)
}

func Test_Server_AcceptConnections_ErrorWhileRunning(t *testing.T) {
	srv := newTestServer(t)
	profile := uniqueProfile(t)

	ctx, cancel := context.WithCancel(t.Context())

	srv.profile = profile
	srv.services = []string{"api"}

	err := srv.start(ctx)
	require.NoError(t, err)

	srv.listener.Close()

	//nolint:forbidigo // allow accept loop to encounter and log the error
	time.Sleep(50 * time.Millisecond)

	srv.running.Store(false)
	cancel()
	srv.wg.Wait()

	if err := os.Remove(srv.SocketPath()); err != nil && !os.IsNotExist(err) {
		t.Logf("cleanup: %v", err)
	}
}

func Test_Server_Start_ListenError(t *testing.T) {
	srv := newTestServer(t)
	srv.profile = "nonexistent/profile"
	srv.services = []string{"api"}

	err := srv.start(t.Context())
	require.Error(t, err)
	assert.True(t, errors.Is(err, errors.ErrFailedToListenSocket))
}

func Test_Server_Stop_RemoveSocketError(t *testing.T) {
	//nolint:usetesting // socket path length exceeds macOS limit with t.TempDir
	tmpDir, err := os.MkdirTemp("/tmp", "fuku-test-")
	require.NoError(t, err)

	defer os.RemoveAll(tmpDir)

	err = os.WriteFile(tmpDir+"/child", []byte("keep"), 0600)
	require.NoError(t, err)

	srv := newTestServer(t)
	srv.running.Store(true)
	srv.socketPath = tmpDir

	srv.Stop()
}

func Test_Server_HandleConnection_RejectsInvalidTail(t *testing.T) {
	zero := 0
	negative := -1

	tests := []struct {
		name string
		tail *int
	}{
		{
			name: "explicit zero",
			tail: &zero,
		},
		{
			name: "negative value",
			tail: &negative,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := newTestServer(t)
			profile := uniqueProfile(t)

			cancel := startTestServer(t, srv, profile, []string{"api"})
			defer srv.Stop()
			defer cancel()

			conn, err := net.Dial("unix", srv.SocketPath())
			require.NoError(t, err)

			defer conn.Close()

			data, err := json.Marshal(SubscribeRequest{
				Type:          MessageSubscribe,
				Services:      []string{},
				ReplayOptions: ReplayOptions{Tail: tt.tail},
			})
			require.NoError(t, err)

			data = append(data, '\n')
			_, err = conn.Write(data)
			require.NoError(t, err)

			err = conn.SetReadDeadline(time.Now().Add(1 * time.Second))
			require.NoError(t, err)

			buf := make([]byte, 1)
			_, err = conn.Read(buf)
			assert.ErrorIs(t, err, io.EOF)
		})
	}
}

func Test_Server_HandleConnection_EchoesAcceptedOptions(t *testing.T) {
	tail := 2

	tests := []struct {
		name     string
		request  SubscribeRequest
		expected StatusMessage
	}{
		{
			name: "tail and no-follow",
			request: SubscribeRequest{
				Type:          MessageSubscribe,
				Services:      []string{"api"},
				ReplayOptions: ReplayOptions{Tail: &tail, NoFollow: true},
			},
			expected: StatusMessage{
				ReplayOptions: ReplayOptions{Tail: &tail, NoFollow: true},
			},
		},
		{
			name: "no bounded options",
			request: SubscribeRequest{
				Type:     MessageSubscribe,
				Services: []string{"api"},
			},
			expected: StatusMessage{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := newTestServer(t)
			profile := uniqueProfile(t)

			cancel := startTestServer(t, srv, profile, []string{"api"})
			defer srv.Stop()
			defer cancel()

			conn, err := net.Dial("unix", srv.SocketPath())
			require.NoError(t, err)

			defer conn.Close()

			data, err := json.Marshal(tt.request)
			require.NoError(t, err)

			data = append(data, '\n')
			_, err = conn.Write(data)
			require.NoError(t, err)

			err = conn.SetReadDeadline(time.Now().Add(1 * time.Second))
			require.NoError(t, err)

			line, err := bufio.NewReader(conn).ReadBytes('\n')
			require.NoError(t, err)

			var status StatusMessage

			err = json.Unmarshal(line, &status)
			require.NoError(t, err)
			assert.Equal(t, MessageStatus, status.Type)
			assert.Equal(t, tt.expected.Tail, status.Tail)
			assert.Equal(t, tt.expected.NoFollow, status.NoFollow)
		})
	}
}

func Test_Server_HandleConnection_NoFollowDrainsReplayBeforeClosing(t *testing.T) {
	tail := 2

	srv := newTestServer(t)
	profile := uniqueProfile(t)

	seeded, ok := srv.hub.(*hub)
	require.True(t, ok)

	pushHistory(seeded,
		LogMessage{Type: MessageLog, Service: "api", Message: "history-msg-1"},
		LogMessage{Type: MessageLog, Service: "web", Message: "history-msg-2"},
		LogMessage{Type: MessageLog, Service: "api", Message: "history-msg-3"},
		LogMessage{Type: MessageLog, Service: "api", Message: "history-msg-4"},
	)

	cancel := startTestServer(t, srv, profile, []string{"api"})
	defer srv.Stop()
	defer cancel()

	conn, err := net.Dial("unix", srv.SocketPath())
	require.NoError(t, err)

	defer conn.Close()

	data, err := json.Marshal(SubscribeRequest{
		Type:          MessageSubscribe,
		Services:      []string{"api"},
		ReplayOptions: ReplayOptions{Tail: &tail, NoFollow: true},
	})
	require.NoError(t, err)

	data = append(data, '\n')
	_, err = conn.Write(data)
	require.NoError(t, err)

	err = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	require.NoError(t, err)

	reader := bufio.NewReader(conn)

	line, err := reader.ReadBytes('\n')
	require.NoError(t, err)

	var status StatusMessage

	err = json.Unmarshal(line, &status)
	require.NoError(t, err)
	assert.Equal(t, MessageStatus, status.Type)

	replayed := make([]string, 0, 2)

	for {
		line, err = reader.ReadBytes('\n')
		if err != nil {
			break
		}

		var msg LogMessage

		require.NoError(t, json.Unmarshal(line, &msg))

		replayed = append(replayed, msg.Message)
	}

	require.ErrorIs(t, err, io.EOF)
	assert.Equal(t, []string{"history-msg-3", "history-msg-4"}, replayed)
}

func Test_Server_HandleConnection_NoFollowWithEmptyHistoryClosesAfterStatus(t *testing.T) {
	srv := newTestServer(t)
	profile := uniqueProfile(t)

	cancel := startTestServer(t, srv, profile, []string{"api"})
	defer srv.Stop()
	defer cancel()

	conn, err := net.Dial("unix", srv.SocketPath())
	require.NoError(t, err)

	defer conn.Close()

	data, err := json.Marshal(SubscribeRequest{
		Type:          MessageSubscribe,
		Services:      []string{"api"},
		ReplayOptions: ReplayOptions{NoFollow: true},
	})
	require.NoError(t, err)

	data = append(data, '\n')
	_, err = conn.Write(data)
	require.NoError(t, err)

	err = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	require.NoError(t, err)

	reader := bufio.NewReader(conn)

	line, err := reader.ReadBytes('\n')
	require.NoError(t, err)

	var status StatusMessage

	err = json.Unmarshal(line, &status)
	require.NoError(t, err)
	assert.Equal(t, MessageStatus, status.Type)

	_, err = reader.ReadBytes('\n')
	assert.ErrorIs(t, err, io.EOF)
}
