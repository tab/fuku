package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_DefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	assert.NotNil(t, cfg.Services)
	assert.NotNil(t, cfg.Profiles)
	assert.Equal(t, LogLevel, cfg.Logging.Level)
	assert.Equal(t, LogFormat, cfg.Logging.Format)
	assert.Equal(t, MaxWorkers, cfg.Concurrency.Workers)
	assert.Equal(t, RetryAttempts, cfg.Retry.Attempts)
	assert.Equal(t, RetryBackoff, cfg.Retry.Backoff)
	assert.Equal(t, SocketLogsBufferSize, cfg.Logs.Buffer)
	assert.Equal(t, SocketLogsHistorySize, cfg.Logs.History)
	assert.Equal(t, 1, cfg.Version)
}

func Test_ApplyDefaults(t *testing.T) {
	tests := []struct {
		name     string
		config   *Config
		expected *Config
	}{
		{
			name: "no defaults",
			config: &Config{
				Services: map[string]*Service{
					"test": {Dir: "test"},
				},
			},
			expected: &Config{
				Services: map[string]*Service{
					"test": {Dir: "test"},
				},
			},
		},
		{
			name: "apply defaults to service",
			config: &Config{
				Services: map[string]*Service{
					"api":  {Dir: "api"},
					"test": {},
				},
				Defaults: &ServiceDefaults{
					Profiles: []string{"default"},
				},
			},
			expected: &Config{
				Services: map[string]*Service{
					"api":  {Dir: "api", Profiles: []string{"default"}},
					"test": {Dir: "test", Profiles: []string{"default"}},
				},
				Defaults: &ServiceDefaults{
					Profiles: []string{"default"},
				},
			},
		},
		{
			name: "service without watch config not affected",
			config: &Config{
				Services: map[string]*Service{
					"api": {Dir: "api"},
				},
			},
			expected: &Config{
				Services: map[string]*Service{
					"api": {Dir: "api"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.config.ApplyDefaults()
			assert.Equal(t, tt.expected, tt.config)
		})
	}
}

func Test_TelemetryEnabled(t *testing.T) {
	tests := []struct {
		name     string
		cfg      *Config
		expected bool
	}{
		{
			name:     "enabled when telemetry true and DSN set",
			cfg:      &Config{Telemetry: true, SentryDSN: "https://key@sentry.io/123"},
			expected: true,
		},
		{
			name:     "disabled when telemetry false",
			cfg:      &Config{Telemetry: false, SentryDSN: "https://key@sentry.io/123"},
			expected: false,
		},
		{
			name:     "disabled when DSN empty",
			cfg:      &Config{Telemetry: true, SentryDSN: ""},
			expected: false,
		},
		{
			name:     "disabled when both false and empty",
			cfg:      &Config{Telemetry: false, SentryDSN: ""},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.cfg.TelemetryEnabled())
		})
	}
}

func Test_TelemetryDisabled(t *testing.T) {
	tests := []struct {
		name     string
		cfg      *Config
		expected bool
	}{
		{
			name:     "disabled when telemetry false",
			cfg:      &Config{Telemetry: false, SentryDSN: "https://key@sentry.io/123"},
			expected: true,
		},
		{
			name:     "disabled when DSN empty",
			cfg:      &Config{Telemetry: true, SentryDSN: ""},
			expected: true,
		},
		{
			name:     "disabled when both false and empty",
			cfg:      &Config{Telemetry: false, SentryDSN: ""},
			expected: true,
		},
		{
			name:     "not disabled when telemetry true and DSN set",
			cfg:      &Config{Telemetry: true, SentryDSN: "https://key@sentry.io/123"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.cfg.TelemetryDisabled())
		})
	}
}

func Test_ServerListen(t *testing.T) {
	tests := []struct {
		name   string
		listen string
		want   string
	}{
		{
			name:   "configured address",
			listen: "127.0.0.1:9876",
			want:   "127.0.0.1:9876",
		},
		{
			name:   "empty returns empty",
			listen: "",
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			cfg.Server.Listen = tt.listen

			assert.Equal(t, tt.want, cfg.ServerListen())
		})
	}
}

func Test_ServerToken(t *testing.T) {
	tests := []struct {
		name  string
		token string
		want  string
	}{
		{
			name:  "configured token",
			token: "my-secret",
			want:  "my-secret",
		},
		{
			name:  "empty returns empty",
			token: "",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			cfg.Server.Auth.Token = tt.token

			assert.Equal(t, tt.want, cfg.ServerToken())
		})
	}
}

func intPtr(v int) *int { return &v }

func Test_ServerStreamingConnections(t *testing.T) {
	tests := []struct {
		name string
		max  *int
		want int
	}{
		{
			name: "configured value",
			max:  intPtr(20),
			want: 20,
		},
		{
			name: "nil returns default",
			max:  nil,
			want: StreamingConnections,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			cfg.Server.Streaming.Connections = tt.max

			assert.Equal(t, tt.want, cfg.ServerStreamingConnections())
		})
	}
}

func Test_ServerStreamingBuffer(t *testing.T) {
	tests := []struct {
		name   string
		buffer *int
		want   int
	}{
		{
			name:   "configured value",
			buffer: intPtr(2000),
			want:   2000,
		},
		{
			name:   "nil returns default",
			buffer: nil,
			want:   StreamingBuffer,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			cfg.Server.Streaming.Buffer = tt.buffer

			assert.Equal(t, tt.want, cfg.ServerStreamingBuffer())
		})
	}
}

func Test_NormalizeTier(t *testing.T) {
	tests := []struct {
		name     string
		tier     string
		expected string
	}{
		{
			name:     "lowercase tier unchanged",
			tier:     "foundation",
			expected: "foundation",
		},
		{
			name:     "uppercase tier lowercased",
			tier:     "FOUNDATION",
			expected: "foundation",
		},
		{
			name:     "mixed case tier lowercased",
			tier:     "Foundation",
			expected: "foundation",
		},
		{
			name:     "whitespace trimmed",
			tier:     "  foundation  ",
			expected: "foundation",
		},
		{
			name:     "mixed case with whitespace",
			tier:     " Platform ",
			expected: "platform",
		},
		{
			name:     "empty tier",
			tier:     "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeTier(tt.tier)
			assert.Equal(t, tt.expected, result)
		})
	}
}
