package sentry

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"fuku/internal/config"
)

func Test_NewSentry(t *testing.T) {
	t.Run("Returns client with empty DSN", func(t *testing.T) {
		cfg := &config.Config{
			AppEnv:    config.EnvTest,
			SentryDSN: "",
		}

		client := NewSentry(cfg)
		assert.NotNil(t, client)
	})

	t.Run("Returns client with invalid DSN", func(t *testing.T) {
		cfg := &config.Config{
			AppEnv:    config.EnvDevelopment,
			SentryDSN: "invalid-dsn",
		}

		client := NewSentry(cfg)
		assert.NotNil(t, client)
	})
}

func Test_Flush(t *testing.T) {
	client := &sentryClient{}
	client.Flush()
}
