package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fuku/internal/app/errors"
)

const testToken = "test-token"
const testListen = "127.0.0.1:9876"

func Test_Validate(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid configuration with default workers",
			config:      DefaultConfig(),
			expectError: false,
		},
		{
			name: "valid configuration with custom workers",
			config: func() *Config {
				cfg := DefaultConfig()
				cfg.Concurrency.Workers = 10

				return cfg
			}(),
			expectError: false,
		},
		{
			name: "invalid workers zero",
			config: func() *Config {
				cfg := DefaultConfig()
				cfg.Concurrency.Workers = 0

				return cfg
			}(),
			expectError: true,
			errorMsg:    "concurrency workers must be greater than 0",
		},
		{
			name: "invalid workers negative",
			config: func() *Config {
				cfg := DefaultConfig()
				cfg.Concurrency.Workers = -1

				return cfg
			}(),
			expectError: true,
			errorMsg:    "concurrency workers must be greater than 0",
		},
		{
			name: "invalid retry attempts zero",
			config: func() *Config {
				cfg := DefaultConfig()
				cfg.Retry.Attempts = 0

				return cfg
			}(),
			expectError: true,
			errorMsg:    "retry attempts must be greater than 0",
		},
		{
			name: "invalid retry attempts negative",
			config: func() *Config {
				cfg := DefaultConfig()
				cfg.Retry.Attempts = -1

				return cfg
			}(),
			expectError: true,
			errorMsg:    "retry attempts must be greater than 0",
		},
		{
			name: "invalid retry backoff negative",
			config: func() *Config {
				cfg := DefaultConfig()
				cfg.Retry.Backoff = -1

				return cfg
			}(),
			expectError: true,
			errorMsg:    "retry backoff must not be negative",
		},
		{
			name: "invalid logs buffer zero",
			config: func() *Config {
				cfg := DefaultConfig()
				cfg.Logs.Buffer = 0

				return cfg
			}(),
			expectError: true,
			errorMsg:    "logs buffer must be greater than 0",
		},
		{
			name: "invalid logs buffer negative",
			config: func() *Config {
				cfg := DefaultConfig()
				cfg.Logs.Buffer = -1

				return cfg
			}(),
			expectError: true,
			errorMsg:    "logs buffer must be greater than 0",
		},
		{
			name: "invalid logs history zero",
			config: func() *Config {
				cfg := DefaultConfig()
				cfg.Logs.History = 0

				return cfg
			}(),
			expectError: true,
			errorMsg:    "logs history must be greater than 0",
		},
		{
			name: "invalid logs history negative",
			config: func() *Config {
				cfg := DefaultConfig()
				cfg.Logs.History = -1

				return cfg
			}(),
			expectError: true,
			errorMsg:    "logs history must be greater than 0",
		},
		{
			name: "valid configuration with standard tiers",
			config: func() *Config {
				cfg := DefaultConfig()
				cfg.Services = map[string]*Service{
					"api": {Dir: "api", Tier: "foundation"},
					"web": {Dir: "web", Tier: "platform"},
				}

				return cfg
			}(),
			expectError: false,
		},
		{
			name: "valid configuration with custom tier",
			config: func() *Config {
				cfg := DefaultConfig()
				cfg.Services = map[string]*Service{
					"api": {Dir: "api", Tier: "custom-tier"},
				}

				return cfg
			}(),
			expectError: false,
		},
		{
			name: "valid configuration with mixed tiers",
			config: func() *Config {
				cfg := DefaultConfig()
				cfg.Services = map[string]*Service{
					"api":     {Dir: "api", Tier: "foundation"},
					"custom":  {Dir: "custom", Tier: "middleware"},
					"another": {Dir: "another", Tier: "services"},
				}

				return cfg
			}(),
			expectError: false,
		},
		{
			name: "service with invalid readiness type",
			config: func() *Config {
				cfg := DefaultConfig()
				cfg.Services = map[string]*Service{
					"api": {Dir: "api", Readiness: &Readiness{Type: "invalid"}},
				}

				return cfg
			}(),
			expectError: true,
			errorMsg:    "service api",
		},
		{
			name: "service with http readiness missing url",
			config: func() *Config {
				cfg := DefaultConfig()
				cfg.Services = map[string]*Service{
					"api": {Dir: "api", Readiness: &Readiness{Type: TypeHTTP}},
				}

				return cfg
			}(),
			expectError: true,
			errorMsg:    "service api",
		},
		{
			name: "service with log readiness missing pattern",
			config: func() *Config {
				cfg := DefaultConfig()
				cfg.Services = map[string]*Service{
					"api": {Dir: "api", Readiness: &Readiness{Type: TypeLog}},
				}

				return cfg
			}(),
			expectError: true,
			errorMsg:    "service api",
		},
		{
			name:        "empty services map",
			config:      DefaultConfig(),
			expectError: false,
		},
		{
			name: "service with invalid logs output value",
			config: func() *Config {
				cfg := DefaultConfig()
				cfg.Services = map[string]*Service{
					"api": {Dir: "api", Logs: &Logs{Output: []string{"invalid"}}},
				}

				return cfg
			}(),
			expectError: true,
			errorMsg:    "service api",
		},
		{
			name: "service with whitespace-only command",
			config: func() *Config {
				cfg := DefaultConfig()
				cfg.Services = map[string]*Service{
					"api": {Dir: "api", Command: "   "},
				}

				return cfg
			}(),
			expectError: true,
			errorMsg:    "service api",
		},
		{
			name: "valid server configuration",
			config: func() *Config {
				cfg := DefaultConfig()
				cfg.Server.Listen = testListen
				cfg.Server.Auth.Token = testToken

				return cfg
			}(),
			expectError: false,
		},
		{
			name: "valid server configuration with IPv6 loopback",
			config: func() *Config {
				cfg := DefaultConfig()
				cfg.Server.Listen = "[::1]:9876"
				cfg.Server.Auth.Token = testToken

				return cfg
			}(),
			expectError: false,
		},
		{
			name:        "server without listen address is disabled",
			config:      DefaultConfig(),
			expectError: false,
		},
		{
			name: "valid server configuration with localhost",
			config: func() *Config {
				cfg := DefaultConfig()
				cfg.Server.Listen = "localhost:9876"
				cfg.Server.Auth.Token = testToken

				return cfg
			}(),
			expectError: false,
		},
		{
			name: "server with listen but no token",
			config: func() *Config {
				cfg := DefaultConfig()
				cfg.Server.Listen = testListen

				return cfg
			}(),
			expectError: true,
			errorMsg:    "server.auth.token is required",
		},
		{
			name: "server with non-loopback address",
			config: func() *Config {
				cfg := DefaultConfig()
				cfg.Server.Listen = "0.0.0.0:9876"
				cfg.Server.Auth.Token = testToken

				return cfg
			}(),
			expectError: true,
			errorMsg:    "api listen must bind to a loopback address",
		},
		{
			name: "server with invalid listen address",
			config: func() *Config {
				cfg := DefaultConfig()
				cfg.Server.Listen = "not-valid"
				cfg.Server.Auth.Token = testToken

				return cfg
			}(),
			expectError: true,
			errorMsg:    "api listen must be a valid host:port address",
		},
		{
			name: "server with out-of-range port",
			config: func() *Config {
				cfg := DefaultConfig()
				cfg.Server.Listen = "127.0.0.1:99999"
				cfg.Server.Auth.Token = testToken

				return cfg
			}(),
			expectError: true,
			errorMsg:    "api listen must be a valid host:port address",
		},
		{
			name: "server with zero port",
			config: func() *Config {
				cfg := DefaultConfig()
				cfg.Server.Listen = "127.0.0.1:0"
				cfg.Server.Auth.Token = testToken

				return cfg
			}(),
			expectError: true,
			errorMsg:    "api listen must be a valid host:port address",
		},
		{
			name: "server with empty host",
			config: func() *Config {
				cfg := DefaultConfig()
				cfg.Server.Listen = ":9876"
				cfg.Server.Auth.Token = testToken

				return cfg
			}(),
			expectError: true,
			errorMsg:    "api listen must be a valid host:port address",
		},
		{
			name: "streaming defaults applied when section absent",
			config: func() *Config {
				cfg := DefaultConfig()
				cfg.Server.Listen = testListen
				cfg.Server.Auth.Token = testToken

				return cfg
			}(),
			expectError: false,
		},
		{
			name: "streaming with valid custom values",
			config: func() *Config {
				cfg := DefaultConfig()
				cfg.Server.Listen = testListen
				cfg.Server.Auth.Token = testToken
				cfg.Server.Streaming.Connections = intPtr(20)
				cfg.Server.Streaming.Buffer = intPtr(5000)

				return cfg
			}(),
			expectError: false,
		},
		{
			name: "streaming with explicit zero connections",
			config: func() *Config {
				cfg := DefaultConfig()
				cfg.Server.Listen = testListen
				cfg.Server.Auth.Token = testToken
				cfg.Server.Streaming.Connections = intPtr(0)

				return cfg
			}(),
			expectError: true,
			errorMsg:    "streaming connections must be greater than 0",
		},
		{
			name: "streaming with explicit zero buffer",
			config: func() *Config {
				cfg := DefaultConfig()
				cfg.Server.Listen = testListen
				cfg.Server.Auth.Token = testToken
				cfg.Server.Streaming.Buffer = intPtr(0)

				return cfg
			}(),
			expectError: true,
			errorMsg:    "streaming buffer must be greater than 0",
		},
		{
			name: "streaming with negative connections",
			config: func() *Config {
				cfg := DefaultConfig()
				cfg.Server.Listen = testListen
				cfg.Server.Auth.Token = testToken
				cfg.Server.Streaming.Connections = intPtr(-1)

				return cfg
			}(),
			expectError: true,
			errorMsg:    "streaming connections must be greater than 0",
		},
		{
			name: "streaming with negative buffer",
			config: func() *Config {
				cfg := DefaultConfig()
				cfg.Server.Listen = testListen
				cfg.Server.Auth.Token = testToken
				cfg.Server.Streaming.Buffer = intPtr(-1)

				return cfg
			}(),
			expectError: true,
			errorMsg:    "streaming buffer must be greater than 0",
		},
		{
			name: "streaming ignored when server disabled",
			config: func() *Config {
				cfg := DefaultConfig()
				cfg.Server.Streaming.Connections = intPtr(-1)

				return cfg
			}(),
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()

			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func Test_ValidateStreaming_DefaultsApplied(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Server.Listen = testListen
	cfg.Server.Auth.Token = testToken

	require.NoError(t, cfg.Validate())

	require.NotNil(t, cfg.Server.Streaming.Connections)
	require.NotNil(t, cfg.Server.Streaming.Buffer)
	assert.Equal(t, StreamingConnections, *cfg.Server.Streaming.Connections)
	assert.Equal(t, StreamingBuffer, *cfg.Server.Streaming.Buffer)
}

func Test_ValidateCommand(t *testing.T) {
	tests := []struct {
		name        string
		command     string
		expectError bool
	}{
		{
			name:        "empty command is valid (uses default)",
			command:     "",
			expectError: false,
		},
		{
			name:        "valid command",
			command:     "go run cmd/main.go",
			expectError: false,
		},
		{
			name:        "whitespace-only command is invalid",
			command:     "   ",
			expectError: true,
		},
		{
			name:        "tab-only command is invalid",
			command:     "\t",
			expectError: true,
		},
		{
			name:        "newline-only command is invalid",
			command:     "\n",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := &Service{Command: tt.command}
			err := service.validateCommand()

			if tt.expectError {
				require.Error(t, err)
				assert.ErrorIs(t, err, errors.ErrInvalidCommand)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func Test_ValidateReadiness(t *testing.T) {
	tests := []struct {
		name        string
		readiness   *Readiness
		expectError bool
		expectedErr error
	}{
		{
			name:        "nil readiness is valid",
			readiness:   nil,
			expectError: false,
		},
		{
			name: "empty type",
			readiness: &Readiness{
				Type: "",
			},
			expectError: true,
			expectedErr: errors.ErrReadinessTypeRequired,
		},
		{
			name: "invalid type",
			readiness: &Readiness{
				Type: "invalid",
			},
			expectError: true,
			expectedErr: errors.ErrInvalidReadinessType,
		},
		{
			name: "uppercase type is invalid",
			readiness: &Readiness{
				Type: "HTTP",
				URL:  "http://localhost:8080",
			},
			expectError: true,
			expectedErr: errors.ErrInvalidReadinessType,
		},
		{
			name: "http type with url is valid",
			readiness: &Readiness{
				Type: TypeHTTP,
				URL:  "http://localhost:8080",
			},
			expectError: false,
		},
		{
			name: "http type without url",
			readiness: &Readiness{
				Type: TypeHTTP,
			},
			expectError: true,
			expectedErr: errors.ErrReadinessURLRequired,
		},
		{
			name: "tcp type with address is valid",
			readiness: &Readiness{
				Type:    TypeTCP,
				Address: "localhost:9090",
			},
			expectError: false,
		},
		{
			name: "tcp type without address",
			readiness: &Readiness{
				Type: TypeTCP,
			},
			expectError: true,
			expectedErr: errors.ErrReadinessAddressRequired,
		},
		{
			name: "log type with pattern is valid",
			readiness: &Readiness{
				Type:    TypeLog,
				Pattern: "Server started",
			},
			expectError: false,
		},
		{
			name: "log type without pattern",
			readiness: &Readiness{
				Type: TypeLog,
			},
			expectError: true,
			expectedErr: errors.ErrReadinessPatternRequired,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := &Service{Readiness: tt.readiness}
			err := service.validateReadiness()

			if tt.expectError {
				require.Error(t, err)
				assert.ErrorIs(t, err, tt.expectedErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func Test_ValidateServiceLogs(t *testing.T) {
	tests := []struct {
		name        string
		logs        *Logs
		expectError bool
		expectedErr error
	}{
		{
			name:        "nil logs is valid",
			logs:        nil,
			expectError: false,
		},
		{
			name:        "empty output is valid",
			logs:        &Logs{Output: []string{}},
			expectError: false,
		},
		{
			name:        "stdout only is valid",
			logs:        &Logs{Output: []string{"stdout"}},
			expectError: false,
		},
		{
			name:        "stderr only is valid",
			logs:        &Logs{Output: []string{"stderr"}},
			expectError: false,
		},
		{
			name:        "both stdout and stderr is valid",
			logs:        &Logs{Output: []string{"stdout", "stderr"}},
			expectError: false,
		},
		{
			name:        "case insensitive STDOUT is valid",
			logs:        &Logs{Output: []string{"STDOUT"}},
			expectError: false,
		},
		{
			name:        "case insensitive STDERR is valid",
			logs:        &Logs{Output: []string{"Stderr"}},
			expectError: false,
		},
		{
			name:        "invalid output value",
			logs:        &Logs{Output: []string{"invalid"}},
			expectError: true,
			expectedErr: errors.ErrInvalidLogsOutput,
		},
		{
			name:        "mixed valid and invalid output values",
			logs:        &Logs{Output: []string{"stdout", "badvalue"}},
			expectError: true,
			expectedErr: errors.ErrInvalidLogsOutput,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := &Service{Logs: tt.logs}
			err := service.validateLogs()

			if tt.expectError {
				require.Error(t, err)
				assert.ErrorIs(t, err, tt.expectedErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func Test_ValidateWatch(t *testing.T) {
	tests := []struct {
		name        string
		watch       *Watch
		expectError bool
		expectedErr error
	}{
		{
			name:        "nil watch is valid",
			watch:       nil,
			expectError: false,
		},
		{
			name: "watch with include is valid",
			watch: &Watch{
				Include: []string{"**/*.go"},
			},
			expectError: false,
		},
		{
			name: "watch with include and ignore is valid",
			watch: &Watch{
				Include: []string{"**/*.go", "**/*.yaml"},
				Ignore:  []string{"*_test.go", "vendor/**"},
			},
			expectError: false,
		},
		{
			name: "watch with include ignore and shared is valid",
			watch: &Watch{
				Include: []string{"**/*.go"},
				Ignore:  []string{"*_test.go"},
				Shared:  []string{"pkg/common", "pkg/models"},
			},
			expectError: false,
		},
		{
			name: "watch with empty include",
			watch: &Watch{
				Include: []string{},
			},
			expectError: true,
			expectedErr: errors.ErrWatchIncludeRequired,
		},
		{
			name: "watch without include field",
			watch: &Watch{
				Ignore: []string{"*_test.go"},
			},
			expectError: true,
			expectedErr: errors.ErrWatchIncludeRequired,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := &Service{Watch: tt.watch}
			err := service.validateWatch()

			if tt.expectError {
				require.Error(t, err)
				assert.ErrorIs(t, err, tt.expectedErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
