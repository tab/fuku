package telemetry

import (
	"testing"

	"github.com/getsentry/sentry-go"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func Test_Client_Start(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTransport := NewMockTransport(ctrl)

	hub := sentry.CurrentHub()
	bound := hub.Client()

	t.Cleanup(func() { hub.BindClient(bound) })

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", home)

	dsn := "https://key@localhost/1"

	tests := []struct {
		name   string
		before func() *Client
	}{
		{
			name: "disabled",
			before: func() *Client {
				return NewClient(Options{Environment: "test", transport: mockTransport})
			},
		},
		{
			name: "disabled with a DSN",
			before: func() *Client {
				return NewClient(Options{DSN: dsn, Environment: "development", transport: mockTransport})
			},
		},
		{
			name: "invalid DSN",
			before: func() *Client {
				return NewClient(Options{Enabled: true, DSN: "invalid-dsn", Environment: "development", transport: mockTransport})
			},
		},
		{
			name: "enabled with a DSN initializes the SDK in Start, not in the constructor",
			before: func() *Client {
				client := NewClient(Options{Enabled: true, DSN: dsn, Environment: "development", transport: mockTransport})
				mockTransport.EXPECT().Configure(gomock.Any())

				return client
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := tt.before()

			client.Start()
		})
	}
}

func Test_Client_Stop(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTransport := NewMockTransport(ctrl)

	hub := sentry.CurrentHub()
	bound := hub.Client()

	t.Cleanup(func() { hub.BindClient(bound) })

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", home)

	options := Options{Enabled: true, Environment: "test", transport: mockTransport}

	tests := []struct {
		name   string
		before func() *Client
	}{
		{
			name: "a started client flushes, closes the transport and unbinds the SDK",
			before: func() *Client {
				mockTransport.EXPECT().Configure(gomock.Any())
				gomock.InOrder(
					mockTransport.EXPECT().FlushWithContext(gomock.Any()).Return(true),
					mockTransport.EXPECT().Close(),
				)

				client := NewClient(options)
				client.Start()

				return client
			},
		},
		{
			name: "a flush that times out leaves the transport open and still unbinds the SDK",
			before: func() *Client {
				mockTransport.EXPECT().Configure(gomock.Any())
				mockTransport.EXPECT().FlushWithContext(gomock.Any()).Return(false)

				client := NewClient(options)
				client.Start()

				return client
			},
		},
		{
			name: "a client that never started has nothing to release",
			before: func() *Client {
				return NewClient(options)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := tt.before()

			client.Stop()

			assert.Nil(t, hub.Client())
		})
	}
}

func Test_Client_Recover(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTransport := NewMockTransport(ctrl)

	hub := sentry.CurrentHub()
	bound := hub.Client()

	t.Cleanup(func() { hub.BindClient(bound) })

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", home)

	options := Options{Enabled: true, Environment: "test", transport: mockTransport}
	fatalBoom := func(event *sentry.Event) bool {
		return event.Level == sentry.LevelFatal && event.Message == "boom"
	}

	tests := []struct {
		name   string
		before func() *Client
	}{
		{
			name: "a started client reports the panic and flushes it",
			before: func() *Client {
				mockTransport.EXPECT().Configure(gomock.Any())
				gomock.InOrder(
					mockTransport.EXPECT().SendEvent(gomock.Cond(fatalBoom)),
					mockTransport.EXPECT().FlushWithContext(gomock.Any()).Return(true),
				)

				client := NewClient(options)
				client.Start()

				return client
			},
		},
		{
			name: "a stopped client reports nothing",
			before: func() *Client {
				mockTransport.EXPECT().Configure(gomock.Any())
				mockTransport.EXPECT().FlushWithContext(gomock.Any()).Return(true)
				mockTransport.EXPECT().Close()

				client := NewClient(options)
				client.Start()
				client.Stop()

				return client
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := tt.before()

			client.Recover("boom")
		})
	}
}

func Test_stripPII(t *testing.T) {
	event := &sentry.Event{
		ServerName: "my-hostname",
		User: sentry.User{
			ID:       "anon-id-123",
			Email:    "user@example.com",
			Username: "jdoe",
		},
	}

	result := stripPII(event, nil)

	assert.Empty(t, result.ServerName)
	assert.Equal(t, "anon-id-123", result.User.ID)
	assert.Empty(t, result.User.Email)
	assert.Empty(t, result.User.Username)
}
