package config

import (
	"testing"

	"github.com/stretchr/testify/require"

	"fuku/internal/model"
	"fuku/internal/platform/logging"
)

func Test_validate(t *testing.T) {
	token := "test-token"

	tests := []struct {
		name        string
		config      *Config
		expectedErr error
	}{
		{
			name:   "valid configuration with default workers",
			config: defaultConfig(),
		},
		{
			name: "valid logging level debug",
			config: func() *Config {
				cfg := defaultConfig()
				cfg.Logging.Level = logging.LevelDebug

				return cfg
			}(),
		},
		{
			name: "logging level trace is invalid",
			config: func() *Config {
				cfg := defaultConfig()
				cfg.Logging.Level = logging.LevelTrace

				return cfg
			}(),
			expectedErr: ErrInvalidLogLevel,
		},
		{
			name: "logging level fatal is invalid",
			config: func() *Config {
				cfg := defaultConfig()
				cfg.Logging.Level = logging.LevelFatal

				return cfg
			}(),
			expectedErr: ErrInvalidLogLevel,
		},
		{
			name: "logging level panic is invalid",
			config: func() *Config {
				cfg := defaultConfig()
				cfg.Logging.Level = logging.LevelPanic

				return cfg
			}(),
			expectedErr: ErrInvalidLogLevel,
		},
		{
			name: "logging level in capitals is invalid",
			config: func() *Config {
				cfg := defaultConfig()
				cfg.Logging.Level = "INFO"

				return cfg
			}(),
			expectedErr: ErrInvalidLogLevel,
		},
		{
			name: "empty logging level is invalid",
			config: func() *Config {
				cfg := defaultConfig()
				cfg.Logging.Level = ""

				return cfg
			}(),
			expectedErr: ErrInvalidLogLevel,
		},
		{
			name: "valid configuration with custom workers",
			config: func() *Config {
				cfg := defaultConfig()
				cfg.Concurrency.Workers = 10

				return cfg
			}(),
		},
		{
			name: "invalid workers zero",
			config: func() *Config {
				cfg := defaultConfig()
				cfg.Concurrency.Workers = 0

				return cfg
			}(),
			expectedErr: ErrInvalidConcurrencyWorkers,
		},
		{
			name: "invalid workers negative",
			config: func() *Config {
				cfg := defaultConfig()
				cfg.Concurrency.Workers = -1

				return cfg
			}(),
			expectedErr: ErrInvalidConcurrencyWorkers,
		},
		{
			name: "invalid retry attempts zero",
			config: func() *Config {
				cfg := defaultConfig()
				cfg.Retry.Attempts = 0

				return cfg
			}(),
			expectedErr: ErrInvalidRetryAttempts,
		},
		{
			name: "invalid retry attempts negative",
			config: func() *Config {
				cfg := defaultConfig()
				cfg.Retry.Attempts = -1

				return cfg
			}(),
			expectedErr: ErrInvalidRetryAttempts,
		},
		{
			name: "invalid retry backoff negative",
			config: func() *Config {
				cfg := defaultConfig()
				cfg.Retry.Backoff = -1

				return cfg
			}(),
			expectedErr: ErrInvalidRetryBackoff,
		},
		{
			name: "invalid logs buffer zero",
			config: func() *Config {
				cfg := defaultConfig()
				cfg.Logs.Buffer = 0

				return cfg
			}(),
			expectedErr: ErrInvalidLogsBuffer,
		},
		{
			name: "invalid logs buffer negative",
			config: func() *Config {
				cfg := defaultConfig()
				cfg.Logs.Buffer = -1

				return cfg
			}(),
			expectedErr: ErrInvalidLogsBuffer,
		},
		{
			name: "invalid logs history zero",
			config: func() *Config {
				cfg := defaultConfig()
				cfg.Logs.History = 0

				return cfg
			}(),
			expectedErr: ErrInvalidLogsHistory,
		},
		{
			name: "invalid logs history negative",
			config: func() *Config {
				cfg := defaultConfig()
				cfg.Logs.History = -1

				return cfg
			}(),
			expectedErr: ErrInvalidLogsHistory,
		},
		{
			name: "valid configuration with standard tiers",
			config: func() *Config {
				cfg := defaultConfig()
				cfg.Services = map[string]*Service{
					"api": {Dir: "api", Tier: "foundation"},
					"web": {Dir: "web", Tier: "platform"},
				}

				return cfg
			}(),
		},
		{
			name: "valid configuration with custom tier",
			config: func() *Config {
				cfg := defaultConfig()
				cfg.Services = map[string]*Service{
					"api": {Dir: "api", Tier: "custom-tier"},
				}

				return cfg
			}(),
		},
		{
			name: "valid configuration with mixed tiers",
			config: func() *Config {
				cfg := defaultConfig()
				cfg.Services = map[string]*Service{
					"api":     {Dir: "api", Tier: "foundation"},
					"custom":  {Dir: "custom", Tier: "middleware"},
					"another": {Dir: "another", Tier: "services"},
				}

				return cfg
			}(),
		},
		{
			name: "service with invalid readiness type",
			config: func() *Config {
				cfg := defaultConfig()
				cfg.Services = map[string]*Service{
					"api": {Dir: "api", Readiness: &Readiness{Type: "invalid"}},
				}

				return cfg
			}(),
			expectedErr: ErrInvalidReadinessType,
		},
		{
			name: "service with http readiness missing url",
			config: func() *Config {
				cfg := defaultConfig()
				cfg.Services = map[string]*Service{
					"api": {Dir: "api", Readiness: &Readiness{Type: model.ReadinessHTTP}},
				}

				return cfg
			}(),
			expectedErr: ErrReadinessURLRequired,
		},
		{
			name: "service with log readiness missing pattern",
			config: func() *Config {
				cfg := defaultConfig()
				cfg.Services = map[string]*Service{
					"api": {Dir: "api", Readiness: &Readiness{Type: model.ReadinessLog}},
				}

				return cfg
			}(),
			expectedErr: ErrReadinessPatternRequired,
		},
		{
			name:   "empty services map",
			config: defaultConfig(),
		},
		{
			name: "service with invalid logs output value",
			config: func() *Config {
				cfg := defaultConfig()
				cfg.Services = map[string]*Service{
					"api": {Dir: "api", Logs: &Logs{Output: []string{"invalid"}}},
				}

				return cfg
			}(),
			expectedErr: ErrInvalidLogsOutput,
		},
		{
			name: "service with whitespace-only command",
			config: func() *Config {
				cfg := defaultConfig()
				cfg.Services = map[string]*Service{
					"api": {Dir: "api", Command: "   "},
				}

				return cfg
			}(),
			expectedErr: ErrInvalidCommand,
		},
		{
			name: "valid server configuration",
			config: func() *Config {
				cfg := defaultConfig()
				cfg.Server.Listen = "127.0.0.1:9876"
				cfg.Server.Auth.Token = token

				return cfg
			}(),
		},
		{
			name: "valid server configuration with IPv6 loopback",
			config: func() *Config {
				cfg := defaultConfig()
				cfg.Server.Listen = "[::1]:9876"
				cfg.Server.Auth.Token = token

				return cfg
			}(),
		},
		{
			name:   "server without listen address is disabled",
			config: defaultConfig(),
		},
		{
			name: "valid server configuration with localhost",
			config: func() *Config {
				cfg := defaultConfig()
				cfg.Server.Listen = "localhost:9876"
				cfg.Server.Auth.Token = token

				return cfg
			}(),
		},
		{
			name: "server with listen but no token",
			config: func() *Config {
				cfg := defaultConfig()
				cfg.Server.Listen = "127.0.0.1:9876"

				return cfg
			}(),
			expectedErr: ErrAPITokenRequired,
		},
		{
			name: "server with non-loopback address",
			config: func() *Config {
				cfg := defaultConfig()
				cfg.Server.Listen = "0.0.0.0:9876"
				cfg.Server.Auth.Token = token

				return cfg
			}(),
			expectedErr: ErrAPINotLoopback,
		},
		{
			name: "server with invalid listen address",
			config: func() *Config {
				cfg := defaultConfig()
				cfg.Server.Listen = "not-valid"
				cfg.Server.Auth.Token = token

				return cfg
			}(),
			expectedErr: ErrAPIInvalidListen,
		},
		{
			name: "server with out-of-range port",
			config: func() *Config {
				cfg := defaultConfig()
				cfg.Server.Listen = "127.0.0.1:99999"
				cfg.Server.Auth.Token = token

				return cfg
			}(),
			expectedErr: ErrAPIInvalidListen,
		},
		{
			name: "server with zero port",
			config: func() *Config {
				cfg := defaultConfig()
				cfg.Server.Listen = "127.0.0.1:0"
				cfg.Server.Auth.Token = token

				return cfg
			}(),
			expectedErr: ErrAPIInvalidListen,
		},
		{
			name: "server with empty host",
			config: func() *Config {
				cfg := defaultConfig()
				cfg.Server.Listen = ":9876"
				cfg.Server.Auth.Token = token

				return cfg
			}(),
			expectedErr: ErrAPIInvalidListen,
		},
		{
			name: "profile string referencing undefined service",
			config: func() *Config {
				cfg := defaultConfig()
				cfg.Services = map[string]*Service{
					"api": {Dir: "api"},
				}
				cfg.Profiles["backend"] = "missing"

				return cfg
			}(),
			expectedErr: ErrProfileReferenceUndefined,
		},
		{
			name: "profile list referencing undefined service",
			config: func() *Config {
				cfg := defaultConfig()
				cfg.Services = map[string]*Service{
					"api": {Dir: "api"},
					"web": {Dir: "web"},
				}
				cfg.Profiles["backend"] = []any{"api", "missing"}

				return cfg
			}(),
			expectedErr: ErrProfileReferenceUndefined,
		},
		{
			name: "profile referencing defined but excluded service is valid",
			config: func() *Config {
				cfg := defaultConfig()
				cfg.Services = map[string]*Service{
					"api": {Dir: "api"},
					"web": {Dir: "web"},
				}
				cfg.Exclude = []string{"web"}
				cfg.Profiles["backend"] = []any{"api", "web"}

				return cfg
			}(),
		},
		{
			name: "wildcard profile is always valid",
			config: func() *Config {
				cfg := defaultConfig()
				cfg.Services = map[string]*Service{
					"api": {Dir: "api"},
				}
				cfg.Profiles["all"] = "*"

				return cfg
			}(),
		},
		{
			name: "profile list with non-string entry errors with unsupported format",
			config: func() *Config {
				cfg := defaultConfig()
				cfg.Services = map[string]*Service{
					"api": {Dir: "api"},
				}
				cfg.Profiles["backend"] = []any{"api", 42}

				return cfg
			}(),
			expectedErr: ErrUnsupportedProfileFormat,
		},
		{
			name: "profile with unsupported value type",
			config: func() *Config {
				cfg := defaultConfig()
				cfg.Services = map[string]*Service{
					"api": {Dir: "api"},
				}
				cfg.Profiles["backend"] = 42

				return cfg
			}(),
			expectedErr: ErrUnsupportedProfileFormat,
		},
		{
			name: "valid profile list referencing existing services",
			config: func() *Config {
				cfg := defaultConfig()
				cfg.Services = map[string]*Service{
					"api": {Dir: "api"},
					"web": {Dir: "web"},
				}
				cfg.Profiles["backend"] = []any{"api", "web"}

				return cfg
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.validate()

			require.ErrorIs(t, err, tt.expectedErr)
		})
	}
}

func Test_ValidateCommand(t *testing.T) {
	tests := []struct {
		name        string
		command     string
		expectedErr error
	}{
		{
			name:    "empty command is valid (uses default)",
			command: "",
		},
		{
			name:    "valid command",
			command: "go run cmd/main.go",
		},
		{
			name:        "whitespace-only command is invalid",
			command:     "   ",
			expectedErr: ErrInvalidCommand,
		},
		{
			name:        "tab-only command is invalid",
			command:     "\t",
			expectedErr: ErrInvalidCommand,
		},
		{
			name:        "newline-only command is invalid",
			command:     "\n",
			expectedErr: ErrInvalidCommand,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := &Service{Command: tt.command}
			err := service.validateCommand()

			require.ErrorIs(t, err, tt.expectedErr)
		})
	}
}

func Test_ValidateReadiness(t *testing.T) {
	tests := []struct {
		name        string
		readiness   *Readiness
		expectedErr error
	}{
		{
			name:      "nil readiness is valid",
			readiness: nil,
		},
		{
			name: "empty type",
			readiness: &Readiness{
				Type: "",
			},
			expectedErr: ErrReadinessTypeRequired,
		},
		{
			name: "invalid type",
			readiness: &Readiness{
				Type: "invalid",
			},
			expectedErr: ErrInvalidReadinessType,
		},
		{
			name: "uppercase type is invalid",
			readiness: &Readiness{
				Type: "HTTP",
				URL:  "http://localhost:8080",
			},
			expectedErr: ErrInvalidReadinessType,
		},
		{
			name: "http type with url is valid",
			readiness: &Readiness{
				Type: model.ReadinessHTTP,
				URL:  "http://localhost:8080",
			},
		},
		{
			name: "http type without url",
			readiness: &Readiness{
				Type: model.ReadinessHTTP,
			},
			expectedErr: ErrReadinessURLRequired,
		},
		{
			name: "tcp type with address is valid",
			readiness: &Readiness{
				Type:    model.ReadinessTCP,
				Address: "localhost:9090",
			},
		},
		{
			name: "tcp type without address",
			readiness: &Readiness{
				Type: model.ReadinessTCP,
			},
			expectedErr: ErrReadinessAddressRequired,
		},
		{
			name: "log type with pattern is valid",
			readiness: &Readiness{
				Type:    model.ReadinessLog,
				Pattern: "Server started",
			},
		},
		{
			name: "log type without pattern",
			readiness: &Readiness{
				Type: model.ReadinessLog,
			},
			expectedErr: ErrReadinessPatternRequired,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := &Service{Readiness: tt.readiness}
			err := service.validateReadiness()

			require.ErrorIs(t, err, tt.expectedErr)
		})
	}
}

func Test_ValidateServiceLogs(t *testing.T) {
	tests := []struct {
		name        string
		logs        *Logs
		expectedErr error
	}{
		{
			name: "nil logs is valid",
			logs: nil,
		},
		{
			name: "empty output is valid",
			logs: &Logs{Output: []string{}},
		},
		{
			name: "stdout only is valid",
			logs: &Logs{Output: []string{"stdout"}},
		},
		{
			name: "stderr only is valid",
			logs: &Logs{Output: []string{"stderr"}},
		},
		{
			name: "both stdout and stderr is valid",
			logs: &Logs{Output: []string{"stdout", "stderr"}},
		},
		{
			name: "case insensitive STDOUT is valid",
			logs: &Logs{Output: []string{"STDOUT"}},
		},
		{
			name: "case insensitive STDERR is valid",
			logs: &Logs{Output: []string{"Stderr"}},
		},
		{
			name:        "invalid output value",
			logs:        &Logs{Output: []string{"invalid"}},
			expectedErr: ErrInvalidLogsOutput,
		},
		{
			name:        "mixed valid and invalid output values",
			logs:        &Logs{Output: []string{"stdout", "badvalue"}},
			expectedErr: ErrInvalidLogsOutput,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := &Service{Logs: tt.logs}
			err := service.validateLogs()

			require.ErrorIs(t, err, tt.expectedErr)
		})
	}
}

func Test_ValidateWatch(t *testing.T) {
	tests := []struct {
		name        string
		watch       *Watch
		expectedErr error
	}{
		{
			name:  "nil watch is valid",
			watch: nil,
		},
		{
			name: "watch with include is valid",
			watch: &Watch{
				Include: []string{"**/*.go"},
			},
		},
		{
			name: "watch with include and ignore is valid",
			watch: &Watch{
				Include: []string{"**/*.go", "**/*.yaml"},
				Ignore:  []string{"*_test.go", "vendor/**"},
			},
		},
		{
			name: "watch with include ignore and shared is valid",
			watch: &Watch{
				Include: []string{"**/*.go"},
				Ignore:  []string{"*_test.go"},
				Shared:  []string{"pkg/common", "pkg/models"},
			},
		},
		{
			name: "watch with empty include",
			watch: &Watch{
				Include: []string{},
			},
			expectedErr: ErrWatchIncludeRequired,
		},
		{
			name: "watch without include field",
			watch: &Watch{
				Ignore: []string{"*_test.go"},
			},
			expectedErr: ErrWatchIncludeRequired,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := &Service{Watch: tt.watch}
			err := service.validateWatch()

			require.ErrorIs(t, err, tt.expectedErr)
		})
	}
}
