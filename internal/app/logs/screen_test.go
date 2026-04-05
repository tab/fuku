package logs

import (
	"bytes"
	"errors"
	"net"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"fuku/internal/app/relay"
	"fuku/internal/app/render"
	"fuku/internal/config"
	"fuku/internal/config/logger"
)

func Test_NewScreen(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := relay.NewMockClient(ctrl)
	mockLog := logger.NewMockLogger(ctrl)
	mockLog.EXPECT().WithComponent("LOGS").Return(mockLog)

	cfg := config.DefaultConfig()
	r := render.NewLog(false)

	s := NewScreen(mockClient, mockLog, r, cfg)

	require.NotNil(t, s)
}

func Test_screen_streamLogs(t *testing.T) {
	tests := []struct {
		name     string
		services []string
		before   func(client *relay.MockClient)
		expect   int
	}{
		{
			name:     "success",
			services: []string{"api"},
			before: func(client *relay.MockClient) {
				client.EXPECT().Connect("/tmp/test.sock").Return(nil)
				client.EXPECT().Subscribe([]string{"api"}).Return(nil)
				client.EXPECT().Stream(gomock.Any(), gomock.Any()).Return(nil)
				client.EXPECT().Close().Return(nil)
			},
			expect: 0,
		},
		{
			name:     "connect error",
			services: nil,
			before: func(client *relay.MockClient) {
				client.EXPECT().Connect("/tmp/test.sock").Return(errors.New("connection refused"))
			},
			expect: 1,
		},
		{
			name:     "subscribe error",
			services: []string{"api"},
			before: func(client *relay.MockClient) {
				client.EXPECT().Connect("/tmp/test.sock").Return(nil)
				client.EXPECT().Subscribe([]string{"api"}).Return(errors.New("subscribe failed"))
				client.EXPECT().Close().Return(nil)
			},
			expect: 1,
		},
		{
			name:     "stream error",
			services: []string{"api", "web"},
			before: func(client *relay.MockClient) {
				client.EXPECT().Connect("/tmp/test.sock").Return(nil)
				client.EXPECT().Subscribe([]string{"api", "web"}).Return(nil)
				client.EXPECT().Stream(gomock.Any(), gomock.Any()).Return(errors.New("stream interrupted"))
				client.EXPECT().Close().Return(nil)
			},
			expect: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockClient := relay.NewMockClient(ctrl)
			mockLog := logger.NewMockLogger(ctrl)
			mockLog.EXPECT().Error().Return(nil).AnyTimes()
			mockLog.EXPECT().Info().Return(nil).AnyTimes()

			tt.before(mockClient)

			var buf bytes.Buffer

			s := &screen{
				client: mockClient,
				log:    mockLog,
				render: render.NewLog(false),
				format: logger.ConsoleFormat,
				out:    &buf,
				width:  func() int { return 80 },
			}

			result := s.streamLogs(t.Context(), "/tmp/test.sock", tt.services)

			assert.Equal(t, tt.expect, result)
		})
	}
}

func Test_screen_streamLogs_WritesToOutput(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := relay.NewMockClient(ctrl)
	mockLog := logger.NewMockLogger(ctrl)
	mockLog.EXPECT().Error().Return(nil).AnyTimes()

	var buf bytes.Buffer

	r := render.NewLog(false)

	mockClient.EXPECT().Connect("/tmp/test.sock").Return(nil)
	mockClient.EXPECT().Subscribe([]string{"api"}).Return(nil)
	mockClient.EXPECT().Stream(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ any, handler relay.Handler) error {
			handler.HandleStatus(relay.StatusMessage{
				Profile:  "default",
				Version:  "1.0.0",
				Services: []string{"api"},
			})
			handler.HandleLog(relay.LogMessage{
				Service: "api",
				Message: "hello from api",
			})

			return nil
		},
	)
	mockClient.EXPECT().Close().Return(nil)

	s := &screen{
		client: mockClient,
		log:    mockLog,
		render: r,
		format: logger.ConsoleFormat,
		out:    &buf,
		width:  func() int { return 80 },
	}

	result := s.streamLogs(t.Context(), "/tmp/test.sock", []string{"api"})

	assert.Equal(t, 0, result)

	output := buf.String()
	assert.Contains(t, output, "api")
	assert.Contains(t, output, "hello from api")
}

func Test_terminalWidth(t *testing.T) {
	w := terminalWidth()

	assert.GreaterOrEqual(t, w, 40)
}

func Test_screen_Run(t *testing.T) {
	t.Run("FindSocket error returns 1", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockClient := relay.NewMockClient(ctrl)
		mockLog := logger.NewMockLogger(ctrl)
		mockLog.EXPECT().Error().Return(nil).AnyTimes()

		s := &screen{
			client: mockClient,
			log:    mockLog,
			render: render.NewLog(false),
			format: logger.ConsoleFormat,
			out:    &bytes.Buffer{},
			width:  func() int { return 80 },
		}

		result := s.Run(t.Context(), "nonexistent-profile-that-does-not-exist", nil)

		assert.Equal(t, 1, result)
	})

	t.Run("FindSocket success delegates to streamLogs", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		profile := "screen-run-test"
		socketPath := relay.SocketPathForProfile(config.SocketDir, profile)

		ln, err := net.Listen("unix", socketPath)
		require.NoError(t, err)

		defer ln.Close()
		defer os.Remove(socketPath)

		mockClient := relay.NewMockClient(ctrl)
		mockLog := logger.NewMockLogger(ctrl)
		mockLog.EXPECT().Error().Return(nil).AnyTimes()

		mockClient.EXPECT().Connect(socketPath).Return(nil)
		mockClient.EXPECT().Subscribe([]string{"api"}).Return(nil)
		mockClient.EXPECT().Stream(gomock.Any(), gomock.Any()).Return(nil)
		mockClient.EXPECT().Close().Return(nil)

		s := &screen{
			client: mockClient,
			log:    mockLog,
			render: render.NewLog(false),
			format: logger.ConsoleFormat,
			out:    &bytes.Buffer{},
			width:  func() int { return 80 },
		}

		result := s.Run(t.Context(), profile, []string{"api"})

		assert.Equal(t, 0, result)
	})
}

func Test_screenHandler_HandleStatus(t *testing.T) {
	var buf bytes.Buffer

	r := render.NewLog(false)

	handler := &screenHandler{
		render:     r,
		format:     logger.ConsoleFormat,
		subscribed: []string{"api"},
		out:        &buf,
		width:      func() int { return 80 },
	}

	status := relay.StatusMessage{
		Profile:  "default",
		Version:  "1.0.0",
		Services: []string{"api", "web"},
	}

	handler.HandleStatus(status)

	output := buf.String()
	assert.Contains(t, output, "default")
	assert.Contains(t, output, "2 running")
	assert.Contains(t, output, "api")
}

func Test_screenHandler_HandleLog(t *testing.T) {
	tests := []struct {
		name    string
		format  string
		msg     relay.LogMessage
		expects []string
	}{
		{
			name:   "console format",
			format: logger.ConsoleFormat,
			msg: relay.LogMessage{
				Service: "api",
				Message: "request processed",
			},
			expects: []string{"api", "request processed"},
		},
		{
			name:   "JSON format",
			format: logger.JSONFormat,
			msg: relay.LogMessage{
				Service: "web",
				Message: "listening on :3000",
			},
			expects: []string{"web", "listening on :3000"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer

			r := render.NewLog(false)

			handler := &screenHandler{
				render:     r,
				format:     tt.format,
				subscribed: nil,
				out:        &buf,
				width:      func() int { return 80 },
			}

			handler.HandleLog(tt.msg)

			output := buf.String()
			for _, expected := range tt.expects {
				assert.Contains(t, output, expected)
			}
		})
	}
}
