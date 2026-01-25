package logs

import (
	"bufio"
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"fuku/internal/config"
	"fuku/internal/config/logger"
)

func Test_NewServer(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := config.DefaultConfig()
	mockLogger := logger.NewMockLogger(ctrl)
	componentLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().WithComponent("SERVER").Return(componentLogger)

	s := NewServer(cfg, "test-profile", mockLogger)
	assert.NotNil(t, s)

	impl := s.(*server)
	assert.Equal(t, "test-profile", impl.profile)
	assert.Contains(t, impl.socketPath, config.SocketPrefix+"test-profile"+config.SocketSuffix)
	assert.Equal(t, cfg.Logs.Buffer, impl.bufferSize)
	assert.NotNil(t, impl.hub)
	assert.Equal(t, componentLogger, impl.log)
}

func Test_Server_SocketPath(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := config.DefaultConfig()
	mockLogger := logger.NewMockLogger(ctrl)
	componentLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().WithComponent("SERVER").Return(componentLogger)

	s := NewServer(cfg, "my-profile", mockLogger)
	expected := filepath.Join(config.SocketDir, config.SocketPrefix+"my-profile"+config.SocketSuffix)

	result := s.SocketPath()
	assert.Equal(t, expected, result)
}

func Test_Server_StartAndStop(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	socketPath := filepath.Join("/tmp", "fuku-test-start.sock")

	defer os.Remove(socketPath)

	cfg := config.DefaultConfig()

	mockLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().Info().Return(nil).AnyTimes()
	mockLogger.EXPECT().Debug().Return(nil).AnyTimes()
	mockLogger.EXPECT().Warn().Return(nil).AnyTimes()
	mockLogger.EXPECT().Error().Return(nil).AnyTimes()

	s := &server{
		profile:    "test",
		socketPath: socketPath,
		bufferSize: cfg.Logs.Buffer,
		hub:        NewHub(cfg.Logs.Buffer),
		log:        mockLogger,
	}

	err := s.Start(ctx)
	assert.NoError(t, err)
	assert.True(t, s.running.Load())
	assert.FileExists(t, s.socketPath)

	err = s.Stop()
	assert.NoError(t, err)
	assert.False(t, s.running.Load())
	assert.NoFileExists(t, s.socketPath)
}

func Test_Server_StopWhenNotRunning(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logger.NewMockLogger(ctrl)

	s := &server{
		profile: "test",
		log:     mockLogger,
	}

	err := s.Stop()
	assert.NoError(t, err)
}

func Test_Server_StartFailsOnInvalidPath(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	cfg := config.DefaultConfig()

	mockLogger := logger.NewMockLogger(ctrl)

	s := &server{
		profile:    "test",
		socketPath: "/nonexistent/path/test.sock",
		bufferSize: cfg.Logs.Buffer,
		hub:        NewHub(cfg.Logs.Buffer),
		log:        mockLogger,
	}

	err := s.Start(ctx)
	assert.Error(t, err)
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

func Test_Server_cleanupStaleSocket_NoSocketExists(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logger.NewMockLogger(ctrl)

	s := &server{
		socketPath: filepath.Join("/tmp", "fuku-nonexistent.sock"),
		log:        mockLogger,
	}

	err := s.cleanupStaleSocket()
	assert.NoError(t, err)
}

func Test_Server_cleanupStaleSocket_StaleSocketIsRemoved(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	socketPath := filepath.Join("/tmp", "fuku-stale.sock")

	mockLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().Info().Return(nil)

	f, err := os.Create(socketPath)
	assert.NoError(t, err)
	f.Close()

	s := &server{
		socketPath: socketPath,
		log:        mockLogger,
	}

	err = s.cleanupStaleSocket()
	assert.NoError(t, err)
	assert.NoFileExists(t, socketPath)
}

func Test_Server_cleanupStaleSocket_ActiveSocketReturnsError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	socketPath := filepath.Join("/tmp", "fuku-active.sock")

	mockLogger := logger.NewMockLogger(ctrl)

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Skip("Cannot create unix socket listener:", err)
	}

	defer listener.Close()
	defer os.Remove(socketPath)

	s := &server{
		socketPath: socketPath,
		log:        mockLogger,
	}

	err = s.cleanupStaleSocket()
	assert.Error(t, err)
}

func Test_Server_handleConnection_SuccessfulFlow(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx, cancel := context.WithCancel(context.Background())
	socketPath := filepath.Join("/tmp", "fuku-test-conn1.sock")

	defer os.Remove(socketPath)

	cfg := config.DefaultConfig()

	mockLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().Info().Return(nil).AnyTimes()
	mockLogger.EXPECT().Debug().Return(nil).AnyTimes()
	mockLogger.EXPECT().Warn().Return(nil).AnyTimes()
	mockLogger.EXPECT().Error().Return(nil).AnyTimes()

	s := &server{
		profile:    "test",
		socketPath: socketPath,
		bufferSize: cfg.Logs.Buffer,
		hub:        NewHub(cfg.Logs.Buffer),
		log:        mockLogger,
	}

	err := s.Start(ctx)
	assert.NoError(t, err)

	conn, err := net.Dial("unix", socketPath)
	assert.NoError(t, err)

	subscribeReq := SubscribeRequest{Type: MessageSubscribe, Services: []string{"api"}}
	data, _ := json.Marshal(subscribeReq)
	data = append(data, '\n')

	_, err = conn.Write(data)
	assert.NoError(t, err)

	time.Sleep(50 * time.Millisecond)
	s.Broadcast("api", "hello")

	reader := bufio.NewReader(conn)

	line, err := reader.ReadBytes('\n')
	assert.NoError(t, err)

	var msg LogMessage

	err = json.Unmarshal(line, &msg)
	assert.NoError(t, err)
	assert.Equal(t, MessageLog, msg.Type)
	assert.Equal(t, "api", msg.Service)
	assert.Equal(t, "hello", msg.Message)

	conn.Close()
	cancel()
	s.Stop()
}

func Test_Server_handleConnection_InvalidSubscribeRequest(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx, cancel := context.WithCancel(context.Background())
	socketPath := filepath.Join("/tmp", "fuku-test-conn2.sock")

	defer os.Remove(socketPath)

	cfg := config.DefaultConfig()

	mockLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().Info().Return(nil).AnyTimes()
	mockLogger.EXPECT().Debug().Return(nil).AnyTimes()
	mockLogger.EXPECT().Warn().Return(nil).AnyTimes()
	mockLogger.EXPECT().Error().Return(nil).AnyTimes()

	s := &server{
		profile:    "test",
		socketPath: socketPath,
		bufferSize: cfg.Logs.Buffer,
		hub:        NewHub(cfg.Logs.Buffer),
		log:        mockLogger,
	}

	err := s.Start(ctx)
	assert.NoError(t, err)

	conn, err := net.Dial("unix", socketPath)
	assert.NoError(t, err)

	_, err = conn.Write([]byte("invalid json\n"))
	assert.NoError(t, err)

	time.Sleep(50 * time.Millisecond)

	conn.Close()
	cancel()
	s.Stop()
}

func Test_Server_handleConnection_WrongMessageType(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx, cancel := context.WithCancel(context.Background())
	socketPath := filepath.Join("/tmp", "fuku-test-conn3.sock")

	defer os.Remove(socketPath)

	cfg := config.DefaultConfig()

	mockLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().Info().Return(nil).AnyTimes()
	mockLogger.EXPECT().Debug().Return(nil).AnyTimes()
	mockLogger.EXPECT().Warn().Return(nil).AnyTimes()
	mockLogger.EXPECT().Error().Return(nil).AnyTimes()

	s := &server{
		profile:    "test",
		socketPath: socketPath,
		bufferSize: cfg.Logs.Buffer,
		hub:        NewHub(cfg.Logs.Buffer),
		log:        mockLogger,
	}

	err := s.Start(ctx)
	assert.NoError(t, err)

	conn, err := net.Dial("unix", socketPath)
	assert.NoError(t, err)

	wrongReq := SubscribeRequest{Type: MessageLog, Services: []string{"api"}}
	data, _ := json.Marshal(wrongReq)
	data = append(data, '\n')

	_, err = conn.Write(data)
	assert.NoError(t, err)

	time.Sleep(50 * time.Millisecond)

	conn.Close()
	cancel()
	s.Stop()
}
