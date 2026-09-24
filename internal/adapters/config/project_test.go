package config

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fuku/internal/model"
)

func Test_Project(t *testing.T) {
	services := map[string]*Service{
		"web": {
			Dir:     "services/web",
			Command: "make dev",
			Tier:    "edge",
			Readiness: &Readiness{
				Type:     model.ReadinessHTTP,
				URL:      "http://localhost:8080/health",
				Timeout:  3 * time.Second,
				Interval: 200 * time.Millisecond,
			},
			Watch: &Watch{
				Include:  []string{"**/*.go"},
				Ignore:   []string{"vendor/**"},
				Shared:   []string{"../shared/**"},
				Debounce: time.Second,
			},
			Logs: &Logs{Output: []string{"stdout"}},
			Env:  &Env{Files: []string{".env", ".env.local"}},
		},
		"db":      {Dir: "services/db", Tier: "foundation"},
		"cache":   {Dir: "services/cache", Tier: "foundation", Env: &Env{}},
		"unknown": {Dir: "services/unknown", Tier: "missing", Env: &Env{Files: []string{}}},
	}
	profiles := map[string]any{
		"all":     "*",
		"backend": []any{"db", "unknown"},
		"single":  "web",
		"invalid": 42,
	}
	exclude := []string{"unknown"}
	tierOrder := []string{"foundation", "edge"}
	cfg := &Config{
		Services:    services,
		Profiles:    profiles,
		Exclude:     exclude,
		Logging:     Logging{Level: "debug", Format: "json"},
		Concurrency: Concurrency{Workers: 4},
		Retry:       Retry{Attempts: 2, Backoff: 250 * time.Millisecond},
		Logs:        LogStream{Buffer: 100, History: 200},
		Server:      Server{Listen: "127.0.0.1:9000"},
		AppEnv:      "development",
		SentryDSN:   "https://dsn",
		Telemetry:   true,
		Updater:     true,
	}
	cfg.Server.Auth.Token = "secret"
	topology := &model.Topology{Order: tierOrder}
	expected := model.Project{
		Services: []model.Service{
			{Name: "cache", Command: DefaultServiceCommand, Directory: "services/cache", Tier: "foundation", Environment: &model.EnvFiles{}},
			{Name: "db", Command: DefaultServiceCommand, Directory: "services/db", Tier: "foundation"},
			{
				Name:      "web",
				Command:   "make dev",
				Directory: "services/web",
				Tier:      "edge",
				Readiness: &model.Readiness{
					Type:     model.ReadinessHTTP,
					URL:      "http://localhost:8080/health",
					Timeout:  3 * time.Second,
					Interval: 200 * time.Millisecond,
				},
				Watch: &model.Watch{
					Include:  []string{"**/*.go"},
					Ignore:   []string{"vendor/**"},
					Shared:   []string{"../shared/**"},
					Debounce: time.Second,
				},
				LogOutput:   []string{"stdout"},
				Environment: &model.EnvFiles{Files: []string{".env", ".env.local"}},
			},
			{
				Name:        "unknown",
				Command:     DefaultServiceCommand,
				Directory:   "services/unknown",
				Tier:        model.TierDefault,
				Environment: &model.EnvFiles{Files: []string{}},
			},
		},
		Profiles: map[string]model.Profile{
			"all":     {All: true},
			"backend": {Services: []string{"db", "unknown"}},
			"single":  {Services: []string{"web"}},
			"invalid": {},
		},
		Exclude:     []string{"unknown"},
		Logging:     model.Logging{Level: "debug", Format: "json"},
		Concurrency: model.Concurrency{Workers: 4},
		Retry:       model.Retry{Attempts: 2, Backoff: 250 * time.Millisecond},
		Logs:        model.Logs{Buffer: 100, History: 200},
		Server:      model.Server{Listen: "127.0.0.1:9000", Token: "secret"},
		Telemetry:   model.Telemetry{Enabled: true, DSN: "https://dsn", Environment: "development"},
		Updater:     model.Updater{Enabled: true},
	}

	actual := Project(cfg, topology)

	ids := make(map[string]bool)

	for i, service := range actual.Services {
		id, err := uuid.Parse(service.ID)
		require.NoError(t, err)
		assert.NotEqual(t, uuid.Nil, id)
		assert.False(t, ids[service.ID], "service IDs must be unique")
		ids[service.ID] = true
		expected.Services[i].ID = service.ID
	}

	assert.Equal(t, expected, actual)
}
