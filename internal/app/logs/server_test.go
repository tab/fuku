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

	s := NewServer(cfg, mockLogger)
	assert.NotNil(t, s)

	impl := s.(*server)
	assert.Equal(t, cfg.Logs.Buffer, impl.bufferSize)
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
		bufferSize: cfg.Logs.Buffer,
		hub:        NewHub(cfg.Logs.Buffer),
		log:        mockLogger,
	}

	ctx := context.Background()
	err := s.Start(ctx, "my-profile")
	assert.NoError(t, err)

	defer s.Stop()

	expected := filepath.Join(config.SocketDir, config.SocketPrefix+"my-profile"+config.SocketSuffix)
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
		bufferSize: cfg.Logs.Buffer,
		hub:        NewHub(cfg.Logs.Buffer),
		log:        mockLogger,
	}

	err := s.Start(ctx, "test-start")
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
		log: mockLogger,
	}

	err := s.Stop()
	assert.NoError(t, err)
}

func Test_Server_StartFailsOnInvalidPath(t *testing.T) {
	// Skip: With the refactored design, Start() computes socketPath from profile + config constants.
	// The SocketDir (/tmp) always exists on Unix, so we can't test invalid path through Start().
	// The old test manually set socketPath before Start(), but Start() now overwrites it.
	// Socket creation errors are covered by cleanupStaleSocket tests (active socket case).
	t.Skip("Socket path is now computed from profile; invalid path scenario is not possible")
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

	cfg := config.DefaultConfig()

	mockLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().Info().Return(nil).AnyTimes()
	mockLogger.EXPECT().Debug().Return(nil).AnyTimes()
	mockLogger.EXPECT().Warn().Return(nil).AnyTimes()
	mockLogger.EXPECT().Error().Return(nil).AnyTimes()

	s := &server{
		bufferSize: cfg.Logs.Buffer,
		hub:        NewHub(cfg.Logs.Buffer),
		log:        mockLogger,
	}

	err := s.Start(ctx, "test-conn1")
	assert.NoError(t, err)

	defer os.Remove(s.SocketPath())

	conn, err := net.Dial("unix", s.SocketPath())
	assert.NoError(t, err)

	subscribeReq := SubscribeRequest{Type: MessageSubscribe, Services: []string{"api"}}
	data, _ := json.Marshal(subscribeReq)
	data = append(data, '\n')

	_, err = conn.Write(data)
	assert.NoError(t, err)

	msgReceived := make(chan LogMessage, 1)

	go func() {
		reader := bufio.NewReader(conn)

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
		bufferSize: cfg.Logs.Buffer,
		hub:        NewHub(cfg.Logs.Buffer),
		log:        mockLogger,
	}

	err := s.Start(ctx, "test-conn2")
	assert.NoError(t, err)

	defer os.Remove(s.SocketPath())

	conn, err := net.Dial("unix", s.SocketPath())
	assert.NoError(t, err)

	_, err = conn.Write([]byte("invalid json\n"))
	assert.NoError(t, err)

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
		bufferSize: cfg.Logs.Buffer,
		hub:        NewHub(cfg.Logs.Buffer),
		log:        mockLogger,
	}

	err := s.Start(ctx, "test-conn3")
	assert.NoError(t, err)

	defer os.Remove(s.SocketPath())

	conn, err := net.Dial("unix", s.SocketPath())
	assert.NoError(t, err)

	wrongReq := SubscribeRequest{Type: MessageLog, Services: []string{"api"}}
	data, _ := json.Marshal(wrongReq)
	data = append(data, '\n')

	_, err = conn.Write(data)
	assert.NoError(t, err)

	conn.Close()
	cancel()
	s.Stop()
}
