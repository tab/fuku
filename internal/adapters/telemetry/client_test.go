package telemetry

import (
	"testing"

	"github.com/getsentry/sentry-go"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func Test_NewClient(t *testing.T) {
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
		name    string
		before  func()
		options Options
	}{
		{
			name:    "disabled",
			before:  func() {},
			options: Options{Environment: "test", transport: mockTransport},
		},
		{
			name:    "disabled with a DSN",
			before:  func() {},
			options: Options{DSN: dsn, Environment: "development", transport: mockTransport},
		},
		{
			name:    "invalid DSN",
			before:  func() {},
			options: Options{Enabled: true, DSN: "invalid-dsn", Environment: "development", transport: mockTransport},
		},
		{
			name: "enabled with a DSN",
			before: func() {
				mockTransport.EXPECT().Configure(gomock.Any())
			},
			options: Options{Enabled: true, DSN: dsn, Environment: "development", transport: mockTransport},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.before()

			NewClient(tt.options)
		})
	}
}

func Test_Client_Flush(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTransport := NewMockTransport(ctrl)

	hub := sentry.CurrentHub()
	bound := hub.Client()

	t.Cleanup(func() { hub.BindClient(bound) })

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", home)

	mockTransport.EXPECT().Configure(gomock.Any())
	mockTransport.EXPECT().FlushWithContext(gomock.Any()).Return(true)

	client := NewClient(Options{Enabled: true, Environment: "test", transport: mockTransport})

	client.Flush()
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

	fatalBoom := func(event *sentry.Event) bool {
		return event.Level == sentry.LevelFatal && event.Message == "boom"
	}

	mockTransport.EXPECT().Configure(gomock.Any())
	gomock.InOrder(
		mockTransport.EXPECT().SendEvent(gomock.Cond(fatalBoom)),
		mockTransport.EXPECT().FlushWithContext(gomock.Any()).Return(true),
	)

	client := NewClient(Options{Enabled: true, Environment: "test", transport: mockTransport})

	client.Recover("boom")
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
