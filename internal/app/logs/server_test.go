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
	assert.Equal(t, expected, s.SocketPath())
}

func Test_Server_StartStop(t *testing.T) {
	t.Run("Start and stop successfully", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// Use /tmp for shorter socket paths (macOS has 104 char limit)
		socketPath := filepath.Join("/tmp", "fuku-test-start.sock")
		mockLogger := logger.NewMockLogger(ctrl)
		mockLogger.EXPECT().Info().Return(nil).AnyTimes()
		mockLogger.EXPECT().Debug().Return(nil).AnyTimes()
		mockLogger.EXPECT().Warn().Return(nil).AnyTimes()
		mockLogger.EXPECT().Error().Return(nil).AnyTimes()

		s := &server{
			profile:    "test",
			socketPath: socketPath,
			hub:        NewHub(config.DefaultConfig()),
			log:        mockLogger,
		}

		defer os.Remove(socketPath)

		ctx := context.Background()
		err := s.Start(ctx)
		assert.NoError(t, err)
		assert.True(t, s.running.Load())
		assert.FileExists(t, s.socketPath)

		err = s.Stop()
		assert.NoError(t, err)
		assert.False(t, s.running.Load())
		assert.NoFileExists(t, s.socketPath)
	})

	t.Run("Stop when not running", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockLogger := logger.NewMockLogger(ctrl)

		s := &server{
			profile: "test",
			log:     mockLogger,
		}

		err := s.Stop()
		assert.NoError(t, err)
	})

	t.Run("Start fails on invalid path", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockLogger := logger.NewMockLogger(ctrl)

		s := &server{
			profile:    "test",
			socketPath: "/nonexistent/path/test.sock",
			hub:        NewHub(config.DefaultConfig()),
			log:        mockLogger,
		}

		ctx := context.Background()
		err := s.Start(ctx)
		assert.Error(t, err)
	})
}

func Test_Server_Broadcast(t *testing.T) {
	t.Run("Broadcasts when running", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockHub := NewMockHub(ctrl)
		mockHub.EXPECT().Broadcast("api", "test message")

		s := &server{
			hub: mockHub,
		}
		s.running.Store(true)

		s.Broadcast("api", "test message")
	})

	t.Run("Does not broadcast when not running", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockHub := NewMockHub(ctrl)

		s := &server{
			hub: mockHub,
		}
		s.running.Store(false)

		s.Broadcast("api", "test message")
	})
}

func Test_Server_cleanupStaleSocket(t *testing.T) {
	t.Run("No socket exists", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockLogger := logger.NewMockLogger(ctrl)

		s := &server{
			socketPath: filepath.Join("/tmp", "fuku-nonexistent.sock"),
			log:        mockLogger,
		}

		err := s.cleanupStaleSocket()
		assert.NoError(t, err)
	})

	t.Run("Stale socket is removed", func(t *testing.T) {
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
	})

	t.Run("Active socket returns error", func(t *testing.T) {
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
	})
}

func Test_Server_handleConnection(t *testing.T) {
	t.Run("Successful connection flow", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		socketPath := filepath.Join("/tmp", "fuku-test-conn1.sock")
		mockLogger := logger.NewMockLogger(ctrl)
		mockLogger.EXPECT().Info().Return(nil).AnyTimes()
		mockLogger.EXPECT().Debug().Return(nil).AnyTimes()
		mockLogger.EXPECT().Warn().Return(nil).AnyTimes()
		mockLogger.EXPECT().Error().Return(nil).AnyTimes()

		s := &server{
			profile:    "test",
			socketPath: socketPath,
			hub:        NewHub(config.DefaultConfig()),
			log:        mockLogger,
		}

		defer os.Remove(socketPath)

		ctx, cancel := context.WithCancel(context.Background())

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
	})

	t.Run("Invalid subscribe request", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		socketPath := filepath.Join("/tmp", "fuku-test-conn2.sock")
		mockLogger := logger.NewMockLogger(ctrl)
		mockLogger.EXPECT().Info().Return(nil).AnyTimes()
		mockLogger.EXPECT().Debug().Return(nil).AnyTimes()
		mockLogger.EXPECT().Warn().Return(nil).AnyTimes()
		mockLogger.EXPECT().Error().Return(nil).AnyTimes()

		s := &server{
			profile:    "test",
			socketPath: socketPath,
			hub:        NewHub(config.DefaultConfig()),
			log:        mockLogger,
		}

		defer os.Remove(socketPath)

		ctx, cancel := context.WithCancel(context.Background())

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
	})

	t.Run("Wrong message type", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		socketPath := filepath.Join("/tmp", "fuku-test-conn3.sock")
		mockLogger := logger.NewMockLogger(ctrl)
		mockLogger.EXPECT().Info().Return(nil).AnyTimes()
		mockLogger.EXPECT().Debug().Return(nil).AnyTimes()
		mockLogger.EXPECT().Warn().Return(nil).AnyTimes()
		mockLogger.EXPECT().Error().Return(nil).AnyTimes()

		s := &server{
			profile:    "test",
			socketPath: socketPath,
			hub:        NewHub(config.DefaultConfig()),
			log:        mockLogger,
		}

		defer os.Remove(socketPath)

		ctx, cancel := context.WithCancel(context.Background())

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
	})
}
