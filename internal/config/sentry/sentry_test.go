package sentry

import (
	"testing"

	gosentry "github.com/getsentry/sentry-go"
	"github.com/stretchr/testify/assert"

	"fuku/internal/config"
)

func Test_NewSentry(t *testing.T) {
	tests := []struct {
		name string
		cfg  *config.Config
	}{
		{
			name: "Empty DSN",
			cfg: &config.Config{
				AppEnv:    config.EnvTest,
				SentryDSN: "",
			},
		},
		{
			name: "Invalid DSN",
			cfg: &config.Config{
				AppEnv:    config.EnvDevelopment,
				SentryDSN: "invalid-dsn",
				Telemetry: true,
			},
		},
		{
			name: "Telemetry disabled",
			cfg: &config.Config{
				AppEnv:    config.EnvDevelopment,
				SentryDSN: "https://key@sentry.io/123",
				Telemetry: false,
			},
		},
		{
			name: "Valid DSN with telemetry enabled",
			cfg: &config.Config{
				AppEnv:    config.EnvDevelopment,
				SentryDSN: "https://key@o123.ingest.sentry.io/456",
				Telemetry: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("XDG_CONFIG_HOME", t.TempDir())

			client := NewSentry(tt.cfg)
			assert.NotNil(t, client)
		})
	}
}

func Test_Flush(t *testing.T) {
	client := &sentryClient{}
	client.Flush()
}

func Test_stripPII(t *testing.T) {
	event := &gosentry.Event{
		ServerName: "my-hostname",
		User: gosentry.User{
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
