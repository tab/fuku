package logs

import (
	"bytes"
	"net"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"fuku/internal/app/errors"
	"fuku/internal/app/instance"
	"fuku/internal/app/relay"
	"fuku/internal/app/render"
	"fuku/internal/config"
	"fuku/internal/config/logger"
)

// serviceName is the subscription filter repeated across the screen tests
const serviceName = "api"

func Test_NewScreen(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := relay.NewMockClient(ctrl)
	mockLog := logger.NewMockLogger(ctrl)
	mockLog.EXPECT().WithComponent("LOGS").Return(mockLog)

	cfg := config.DefaultConfig()
	r := render.NewLog(false)
	identity := instance.Identity{
		ID:          "1f0c6e4a-2b8d-4c3e-9a7f-5d6b8c0e1a24",
		Project:     "/Users/dev/projects/shop",
		Fingerprint: instance.Fingerprint("/Users/dev/projects/shop"),
	}

	s := NewScreen(mockClient, r, cfg, identity, mockLog)

	require.NotNil(t, s)
	assert.Equal(t, identity.Project, s.(*screen).project)
	assert.Equal(t, identity.Fingerprint, s.(*screen).fingerprint)
}

func Test_screen_streamLogs(t *testing.T) {
	tail := 100

	tests := []struct {
		name    string
		options Options
		before  func(client *relay.MockClient)
		logged  int
		expect  int
	}{
		{
			name:    "success",
			options: Options{Services: []string{serviceName}},
			before: func(client *relay.MockClient) {
				client.EXPECT().Connect("/tmp/test.sock").Return(nil)
				client.EXPECT().Subscribe(relay.SubscribeOptions{Services: []string{serviceName}}).Return(nil)
				client.EXPECT().Stream(gomock.Any(), gomock.Any()).Return(nil)
				client.EXPECT().Close().Return(nil)
			},
			expect: 0,
		},
		{
			name: "bounded read passes tail and no-follow",
			options: Options{
				Services:      []string{serviceName},
				NoUI:          true,
				ReplayOptions: relay.ReplayOptions{Tail: &tail, NoFollow: true},
			},
			before: func(client *relay.MockClient) {
				client.EXPECT().Connect("/tmp/test.sock").Return(nil)
				client.EXPECT().Subscribe(relay.SubscribeOptions{
					Services:      []string{serviceName},
					ReplayOptions: relay.ReplayOptions{Tail: &tail, NoFollow: true},
				}).Return(nil)
				client.EXPECT().Stream(gomock.Any(), gomock.Any()).Return(nil)
				client.EXPECT().Close().Return(nil)
			},
			expect: 0,
		},
		{
			name:    "connect error",
			options: Options{},
			before: func(client *relay.MockClient) {
				client.EXPECT().Connect("/tmp/test.sock").Return(errors.New("connection refused"))
			},
			logged: 1,
			expect: 1,
		},
		{
			name:    "subscribe error",
			options: Options{Services: []string{serviceName}},
			before: func(client *relay.MockClient) {
				client.EXPECT().Connect("/tmp/test.sock").Return(nil)
				client.EXPECT().Subscribe(relay.SubscribeOptions{Services: []string{serviceName}}).Return(errors.New("subscribe failed"))
				client.EXPECT().Close().Return(nil)
			},
			logged: 1,
			expect: 1,
		},
		{
			name:    "stream error",
			options: Options{Services: []string{serviceName, "web"}},
			before: func(client *relay.MockClient) {
				client.EXPECT().Connect("/tmp/test.sock").Return(nil)
				client.EXPECT().Subscribe(relay.SubscribeOptions{Services: []string{serviceName, "web"}}).Return(nil)
				client.EXPECT().Stream(gomock.Any(), gomock.Any()).Return(errors.New("stream interrupted"))
				client.EXPECT().Close().Return(nil)
			},
			logged: 1,
			expect: 1,
		},
		{
			name:    "profile mismatch",
			options: Options{Profile: "core", Services: []string{serviceName}},
			before: func(client *relay.MockClient) {
				client.EXPECT().Connect("/tmp/test.sock").Return(nil)
				client.EXPECT().Subscribe(relay.SubscribeOptions{Services: []string{serviceName}}).Return(nil)
				client.EXPECT().Stream(gomock.Any(), gomock.Any()).Return(errors.ErrProfileMismatch)
				client.EXPECT().Close().Return(nil)
			},
			expect: 1,
		},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := relay.NewMockClient(ctrl)
	mockLog := logger.NewMockLogger(ctrl)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.before(mockClient)
			mockLog.EXPECT().Error().Return(nil).Times(tt.logged)

			var buf bytes.Buffer

			s := &screen{
				client: mockClient,
				render: render.NewLog(false),
				format: logger.ConsoleFormat,
				out:    &buf,
				width:  func() int { return 80 },
				log:    mockLog,
			}

			result := s.streamLogs(t.Context(), "/tmp/test.sock", tt.options)

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
	mockClient.EXPECT().Subscribe(relay.SubscribeOptions{Services: []string{serviceName}}).Return(nil)
	mockClient.EXPECT().Stream(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ any, handler relay.Handler) error {
			if err := handler.HandleStatus(relay.StatusMessage{
				Profile:  "default",
				Version:  "1.0.0",
				Services: []string{serviceName},
			}); err != nil {
				return err
			}

			handler.HandleLog(relay.LogMessage{
				Service: serviceName,
				Message: "hello from api",
			})

			return nil
		},
	)
	mockClient.EXPECT().Close().Return(nil)

	s := &screen{
		client: mockClient,
		render: r,
		format: logger.ConsoleFormat,
		out:    &buf,
		width:  func() int { return 80 },
		log:    mockLog,
	}

	result := s.streamLogs(t.Context(), "/tmp/test.sock", Options{Services: []string{serviceName}})

	assert.Equal(t, 0, result)

	output := buf.String()
	assert.Contains(t, output, serviceName)
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
		mockLog.EXPECT().Error().Return(nil)

		s := &screen{
			client:      mockClient,
			render:      render.NewLog(false),
			format:      logger.ConsoleFormat,
			project:     "/Users/dev/projects/not-running",
			fingerprint: instance.Fingerprint("/Users/dev/projects/not-running"),
			out:         &bytes.Buffer{},
			width:       func() int { return 80 },
			log:         mockLog,
		}

		result := s.Run(t.Context(), Options{})

		assert.Equal(t, 1, result)
	})

	t.Run("FindSocket success delegates to streamLogs", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		fingerprint := instance.Fingerprint("/Users/dev/projects/screen-run-test")
		socketPath := instance.SocketPath(config.SocketDir, fingerprint)

		ln, err := net.Listen("unix", socketPath)
		require.NoError(t, err)

		defer ln.Close()
		defer os.Remove(socketPath)

		mockClient := relay.NewMockClient(ctrl)
		mockLog := logger.NewMockLogger(ctrl)
		mockLog.EXPECT().Error().Return(nil).AnyTimes()

		mockClient.EXPECT().Connect(socketPath).Return(nil)
		mockClient.EXPECT().Subscribe(relay.SubscribeOptions{Services: []string{serviceName}}).Return(nil)
		mockClient.EXPECT().Stream(gomock.Any(), gomock.Any()).Return(nil)
		mockClient.EXPECT().Close().Return(nil)

		s := &screen{
			client:      mockClient,
			render:      render.NewLog(false),
			format:      logger.ConsoleFormat,
			fingerprint: fingerprint,
			out:         &bytes.Buffer{},
			width:       func() int { return 80 },
			log:         mockLog,
		}

		result := s.Run(t.Context(), Options{Services: []string{serviceName}})

		assert.Equal(t, 0, result)
	})
}

func Test_screenHandler_HandleStatus(t *testing.T) {
	tests := []struct {
		name    string
		profile string
		noUI    bool
		logged  int
		wantErr error
		expects []string
	}{
		{
			name:    "renders the banner by default",
			noUI:    false,
			expects: []string{"default", "2 running", serviceName, "ctrl+c"},
		},
		{
			name:    "hides the panel and footer",
			noUI:    true,
			expects: nil,
		},
		{
			name:    "matching profile renders the banner",
			profile: "default",
			expects: []string{"default", "2 running", serviceName, "ctrl+c"},
		},
		{
			name:    "mismatched profile renders nothing",
			profile: "core",
			logged:  1,
			wantErr: errors.ErrProfileMismatch,
		},
	}

	status := relay.StatusMessage{
		Profile:  "default",
		Version:  "1.0.0",
		Services: []string{serviceName, "web"},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLog := logger.NewMockLogger(ctrl)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockLog.EXPECT().Error().Return(nil).Times(tt.logged)

			var buf bytes.Buffer

			handler := &screenHandler{
				render:     render.NewLog(false),
				format:     logger.ConsoleFormat,
				profile:    tt.profile,
				subscribed: []string{serviceName},
				out:        &buf,
				width:      func() int { return 80 },
				noUI:       tt.noUI,
				log:        mockLog,
			}

			err := handler.HandleStatus(status)

			output := buf.String()

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				assert.Empty(t, output)

				return
			}

			require.NoError(t, err)

			if tt.noUI {
				assert.Empty(t, output)
			}

			for _, expected := range tt.expects {
				assert.Contains(t, output, expected)
			}
		})
	}
}

func Test_screenHandler_HandleLog_NoUIStillWritesLogs(t *testing.T) {
	var buf bytes.Buffer

	handler := &screenHandler{
		render:     render.NewLog(false),
		format:     logger.ConsoleFormat,
		subscribed: []string{serviceName},
		out:        &buf,
		width:      func() int { return 80 },
		noUI:       true,
	}

	handler.HandleLog(relay.LogMessage{Service: serviceName, Message: "request processed"})

	output := buf.String()
	assert.Contains(t, output, serviceName)
	assert.Contains(t, output, "request processed")
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
				Service: serviceName,
				Message: "request processed",
			},
			expects: []string{serviceName, "request processed"},
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
