package logs

import (
	"bufio"
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"fuku/internal/app/errors"
	"fuku/internal/config"
	"fuku/internal/config/logger"
)

func Test_NewServer(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := config.DefaultConfig()
	mockLogger := logger.NewMockLogger(ctrl)
	componentLogger := logger.NewMockLogger(ctrl)
	hubLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().WithComponent("HUB").Return(hubLogger)
	mockLogger.EXPECT().WithComponent("SERVER").Return(componentLogger)

	s := NewServer(cfg, mockLogger)
	assert.NotNil(t, s)

	impl := s.(*server)
	assert.Equal(t, cfg.Logs.Buffer, impl.bufferSize)
	assert.Equal(t, cfg.Logs.History, impl.historySize)
	assert.NotNil(t, impl.hub)
	assert.Equal(t, componentLogger, impl.log)
}

func Test_Server_SocketPath(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := config.DefaultConfig()
	mockLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().Info().Return(nil).AnyTimes()

	s := &server{
		bufferSize:  cfg.Logs.Buffer,
		historySize: cfg.Logs.History,
		hub:         NewHub(cfg.Logs.Buffer, cfg.Logs.History, mockLogger),
		log:         mockLogger,
	}

	ctx := context.Background()
	err := s.Start(ctx, "my-profile", []string{"api", "db"})
	require.NoError(t, err)

	defer s.Stop()

	expected := SocketPathForProfile(config.SocketDir, "my-profile")
	assert.Equal(t, expected, s.SocketPath())
}

func Test_Server_StartAndStop(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	cfg := config.DefaultConfig()

	mockLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().Info().Return(nil).AnyTimes()
	mockLogger.EXPECT().Debug().Return(nil).AnyTimes()
	mockLogger.EXPECT().Warn().Return(nil).AnyTimes()
	mockLogger.EXPECT().Error().Return(nil).AnyTimes()

	s := &server{
		bufferSize:  cfg.Logs.Buffer,
		historySize: cfg.Logs.History,
		hub:         NewHub(cfg.Logs.Buffer, cfg.Logs.History, mockLogger),
		log:         mockLogger,
	}

	err := s.Start(ctx, "test-start", nil)
	require.NoError(t, err)
	assert.True(t, s.running.Load())
	assert.FileExists(t, s.socketPath)

	err = s.Stop()
	require.NoError(t, err)
	assert.False(t, s.running.Load())
	assert.NoFileExists(t, s.socketPath)
}

func Test_Server_StopWhenNotRunning(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logger.NewMockLogger(ctrl)

	s := &server{
		log: mockLogger,
	}

	err := s.Stop()
	require.NoError(t, err)
}

func Test_Server_Broadcast(t *testing.T) {
	tests := []struct {
		name       string
		running    bool
		expectCall bool
	}{
		{
			name:       "Broadcasts when running",
			running:    true,
			expectCall: true,
		},
		{
			name:       "Does not broadcast when not running",
			running:    false,
			expectCall: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockHub := NewMockHub(ctrl)
			if tt.expectCall {
				mockHub.EXPECT().Broadcast("api", "test message")
			}

			s := &server{
				hub: mockHub,
			}
			s.running.Store(tt.running)

			s.Broadcast("api", "test message")
		})
	}
}

func Test_Server_Start_ActiveSocketReturnsError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	socketPath := filepath.Join("/tmp", "fuku-active-start.sock")

	mockLogger := logger.NewMockLogger(ctrl)

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Skip("Cannot create unix socket listener:", err)
	}

	defer listener.Close()
	defer os.Remove(socketPath)

	cfg := config.DefaultConfig()
	s := &server{
		bufferSize:  cfg.Logs.Buffer,
		historySize: cfg.Logs.History,
		hub:         NewHub(cfg.Logs.Buffer, cfg.Logs.History, mockLogger),
		log:         mockLogger,
	}

	err = s.Start(context.Background(), "active-start", nil)
	require.ErrorIs(t, err, errors.ErrSocketAlreadyInUse)
}

func Test_Server_Start_RecoverFromStaleSocket(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	socketPath := filepath.Join("/tmp", "fuku-stale-start.sock")

	f, err := os.Create(socketPath)
	require.NoError(t, err)
	f.Close()

	defer os.Remove(socketPath)

	mockLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().Info().Return(nil).AnyTimes()

	cfg := config.DefaultConfig()
	s := &server{
		bufferSize:  cfg.Logs.Buffer,
		historySize: cfg.Logs.History,
		hub:         NewHub(cfg.Logs.Buffer, cfg.Logs.History, mockLogger),
		log:         mockLogger,
	}

	err = s.Start(context.Background(), "stale-start", nil)
	require.NoError(t, err)

	defer s.Stop()

	assert.True(t, s.running.Load())
	assert.FileExists(t, s.socketPath)
}

func Test_Cleanup_NoSockets(t *testing.T) {
	dir := t.TempDir()

	err := Cleanup(dir)
	require.NoError(t, err)
}

func Test_Cleanup_RemovesStaleSocket(t *testing.T) {
	//nolint:usetesting // t.TempDir() paths exceed Unix socket max length on macOS
	dir, err := os.MkdirTemp("/tmp", "fuku-test-")
	require.NoError(t, err)

	defer os.RemoveAll(dir)

	socketPath := filepath.Join(dir, config.SocketPrefix+"stale"+config.SocketSuffix)

	fd, err := syscall.Socket(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
	require.NoError(t, err)

	require.NoError(t, syscall.Bind(fd, &syscall.SockaddrUnix{Name: socketPath}))
	syscall.Close(fd)

	err = Cleanup(dir)
	require.NoError(t, err)
	assert.NoFileExists(t, socketPath)
}

func Test_Cleanup_SkipsNonSocketFile(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, config.SocketPrefix+"fake"+config.SocketSuffix)

	f, err := os.Create(filePath)
	require.NoError(t, err)
	f.Close()

	err = Cleanup(dir)
	require.NoError(t, err)
	assert.FileExists(t, filePath)
}

func Test_Cleanup_PreservesActiveSocket(t *testing.T) {
	//nolint:usetesting // t.TempDir() paths exceed Unix socket max length on macOS
	dir, err := os.MkdirTemp("/tmp", "fuku-test-")
	require.NoError(t, err)

	defer os.RemoveAll(dir)

	socketPath := filepath.Join(dir, config.SocketPrefix+"active"+config.SocketSuffix)

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Skip("Cannot create unix socket listener:", err)
	}
	defer listener.Close()

	err = Cleanup(dir)
	require.NoError(t, err)
	assert.FileExists(t, socketPath)
}

func Test_Cleanup_MixedSockets(t *testing.T) {
	//nolint:usetesting // t.TempDir() paths exceed Unix socket max length on macOS
	dir, err := os.MkdirTemp("/tmp", "fuku-test-")
	require.NoError(t, err)

	defer os.RemoveAll(dir)

	stalePath := filepath.Join(dir, config.SocketPrefix+"stale"+config.SocketSuffix)
	activePath := filepath.Join(dir, config.SocketPrefix+"active"+config.SocketSuffix)

	listener, err := net.Listen("unix", activePath)
	if err != nil {
		t.Skip("Cannot create unix socket listener:", err)
	}
	defer listener.Close()

	fd, err := syscall.Socket(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
	require.NoError(t, err)

	require.NoError(t, syscall.Bind(fd, &syscall.SockaddrUnix{Name: stalePath}))
	syscall.Close(fd)

	err = Cleanup(dir)
	require.NoError(t, err)
	assert.NoFileExists(t, stalePath)
	assert.FileExists(t, activePath)
}

func Test_Server_handleConnection_SuccessfulFlow(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx, cancel := context.WithCancel(context.Background())

	cfg := config.DefaultConfig()

	mockLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().Info().Return(nil).AnyTimes()
	mockLogger.EXPECT().Debug().Return(nil).AnyTimes()
	mockLogger.EXPECT().Warn().Return(nil).AnyTimes()
	mockLogger.EXPECT().Error().Return(nil).AnyTimes()

	s := &server{
		bufferSize:  cfg.Logs.Buffer,
		historySize: cfg.Logs.History,
		hub:         NewHub(cfg.Logs.Buffer, cfg.Logs.History, mockLogger),
		log:         mockLogger,
	}

	err := s.Start(ctx, "test-conn1", []string{"api", "db"})
	require.NoError(t, err)

	defer os.Remove(s.SocketPath())

	conn, err := net.Dial("unix", s.SocketPath())
	require.NoError(t, err)

	subscribeReq := SubscribeRequest{Type: MessageSubscribe, Services: []string{"api"}}
	data, err := json.Marshal(subscribeReq)
	require.NoError(t, err)

	data = append(data, '\n')

	_, err = conn.Write(data)
	require.NoError(t, err)

	reader := bufio.NewReader(conn)

	statusLine, err := reader.ReadBytes('\n')
	require.NoError(t, err)

	var status StatusMessage

	err = json.Unmarshal(statusLine, &status)
	require.NoError(t, err)
	assert.Equal(t, MessageStatus, status.Type)
	assert.NotEmpty(t, status.Version)
	assert.Equal(t, "test-conn1", status.Profile)
	assert.Equal(t, []string{"api", "db"}, status.Services)

	msgReceived := make(chan LogMessage, 1)

	go func() {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			return
		}

		var msg LogMessage
		if json.Unmarshal(line, &msg) == nil {
			msgReceived <- msg
		}
	}()

	assert.Eventually(t, func() bool {
		s.Broadcast("api", "hello")

		select {
		case msg := <-msgReceived:
			assert.Equal(t, MessageLog, msg.Type)
			assert.Equal(t, "api", msg.Service)
			assert.Equal(t, "hello", msg.Message)

			return true
		default:
			return false
		}
	}, time.Second, 10*time.Millisecond)

	conn.Close()
	cancel()
	s.Stop()
}

func Test_Server_handleConnection_InvalidSubscribeRequest(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx, cancel := context.WithCancel(context.Background())

	cfg := config.DefaultConfig()

	mockLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().Info().Return(nil).AnyTimes()
	mockLogger.EXPECT().Debug().Return(nil).AnyTimes()
	mockLogger.EXPECT().Warn().Return(nil).AnyTimes()
	mockLogger.EXPECT().Error().Return(nil).AnyTimes()

	s := &server{
		bufferSize:  cfg.Logs.Buffer,
		historySize: cfg.Logs.History,
		hub:         NewHub(cfg.Logs.Buffer, cfg.Logs.History, mockLogger),
		log:         mockLogger,
	}

	err := s.Start(ctx, "test-conn2", nil)
	require.NoError(t, err)

	defer os.Remove(s.SocketPath())

	conn, err := net.Dial("unix", s.SocketPath())
	require.NoError(t, err)

	_, err = conn.Write([]byte("invalid json\n"))
	require.NoError(t, err)

	conn.Close()
	cancel()
	s.Stop()
}

func Test_Server_handleConnection_WrongMessageType(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx, cancel := context.WithCancel(context.Background())

	cfg := config.DefaultConfig()

	mockLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().Info().Return(nil).AnyTimes()
	mockLogger.EXPECT().Debug().Return(nil).AnyTimes()
	mockLogger.EXPECT().Warn().Return(nil).AnyTimes()
	mockLogger.EXPECT().Error().Return(nil).AnyTimes()

	s := &server{
		bufferSize:  cfg.Logs.Buffer,
		historySize: cfg.Logs.History,
		hub:         NewHub(cfg.Logs.Buffer, cfg.Logs.History, mockLogger),
		log:         mockLogger,
	}

	err := s.Start(ctx, "test-conn3", nil)
	require.NoError(t, err)

	defer os.Remove(s.SocketPath())

	conn, err := net.Dial("unix", s.SocketPath())
	require.NoError(t, err)

	wrongReq := SubscribeRequest{Type: MessageLog, Services: []string{"api"}}
	data, _ := json.Marshal(wrongReq)
	data = append(data, '\n')

	_, err = conn.Write(data)
	require.NoError(t, err)

	conn.Close()
	cancel()
	s.Stop()
}

func Test_Server_handleConnection_ReplayHistory(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx, cancel := context.WithCancel(context.Background())

	cfg := config.DefaultConfig()

	mockLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().Info().Return(nil).AnyTimes()
	mockLogger.EXPECT().Debug().Return(nil).AnyTimes()
	mockLogger.EXPECT().Warn().Return(nil).AnyTimes()
	mockLogger.EXPECT().Error().Return(nil).AnyTimes()

	s := &server{
		bufferSize:  cfg.Logs.Buffer,
		historySize: cfg.Logs.History,
		hub:         NewHub(cfg.Logs.Buffer, cfg.Logs.History, mockLogger),
		log:         mockLogger,
	}

	err := s.Start(ctx, "test-replay", []string{"api"})
	require.NoError(t, err)

	defer os.Remove(s.SocketPath())

	sentinel, err := net.Dial("unix", s.SocketPath())
	require.NoError(t, err)

	defer sentinel.Close()

	sentinelReq := SubscribeRequest{Type: MessageSubscribe, Services: []string{"api"}}
	sentinelData, err := json.Marshal(sentinelReq)
	require.NoError(t, err)

	sentinelData = append(sentinelData, '\n')

	_, err = sentinel.Write(sentinelData)
	require.NoError(t, err)

	sentinelReader := bufio.NewReader(sentinel)

	_, err = sentinelReader.ReadBytes('\n')
	require.NoError(t, err)

	s.Broadcast("api", "before-connect")

	_, err = sentinelReader.ReadBytes('\n')
	require.NoError(t, err)

	conn, err := net.Dial("unix", s.SocketPath())
	require.NoError(t, err)

	subscribeReq := SubscribeRequest{Type: MessageSubscribe, Services: []string{"api"}}
	data, err := json.Marshal(subscribeReq)
	require.NoError(t, err)

	data = append(data, '\n')

	_, err = conn.Write(data)
	require.NoError(t, err)

	reader := bufio.NewReader(conn)

	statusLine, err := reader.ReadBytes('\n')
	require.NoError(t, err)

	var status StatusMessage

	err = json.Unmarshal(statusLine, &status)
	require.NoError(t, err)
	assert.Equal(t, MessageStatus, status.Type)

	replayLine, err := reader.ReadBytes('\n')
	require.NoError(t, err)

	var replayMsg LogMessage

	err = json.Unmarshal(replayLine, &replayMsg)
	require.NoError(t, err)
	assert.Equal(t, MessageLog, replayMsg.Type)
	assert.Equal(t, "before-connect", replayMsg.Message)

	conn.Close()
	cancel()
	s.Stop()
}

func Test_Server_ClientQueueSizing(t *testing.T) {
	cfg := config.DefaultConfig()

	tests := []struct {
		name        string
		bufferSize  int
		historySize int
		expected    int
	}{
		{
			name:        "History larger than buffer",
			bufferSize:  100,
			historySize: 500,
			expected:    600,
		},
		{
			name:        "Buffer larger than history",
			bufferSize:  cfg.Logs.Buffer,
			historySize: 100,
			expected:    cfg.Logs.Buffer + 100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClientConn("test", tt.bufferSize+tt.historySize)
			assert.Equal(t, tt.expected, cap(client.SendChan))
		})
	}
}
