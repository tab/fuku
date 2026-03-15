package config

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fuku/internal/app/errors"
)

func Test_Load(t *testing.T) {
	tests := []struct {
		name        string
		setupFunc   func() func()
		expectError bool
		error       error
	}{
		{
			name: "no config file found - uses default",
			setupFunc: func() func() {
				return func() {}
			},
			error: nil,
		},
		{
			name: "valid config file",
			setupFunc: func() func() {
				content := `version: 1
services:
  test-service:
    dir: ./test
    profiles: [test]
profiles:
  test:
    include:
      - test-service
logging:
  level: debug
  format: json
`

				err := os.WriteFile("fuku.yaml", []byte(content), 0644)
				if err != nil {
					t.Fatal(err)
				}

				return func() { os.Remove("fuku.yaml") }
			},
			error: nil,
		},
		{
			name: "valid config file with concurrency",
			setupFunc: func() func() {
				content := `version: 1
services:
  test-service:
    dir: ./test
concurrency:
  workers: 10
`

				err := os.WriteFile("fuku.yaml", []byte(content), 0644)
				if err != nil {
					t.Fatal(err)
				}

				return func() { os.Remove("fuku.yaml") }
			},
			error: nil,
		},
		{
			name: "invalid concurrency workers zero",
			setupFunc: func() func() {
				content := `version: 1
services:
  test-service:
    dir: ./test
concurrency:
  workers: 0
`

				err := os.WriteFile("fuku.yaml", []byte(content), 0644)
				if err != nil {
					t.Fatal(err)
				}

				return func() { os.Remove("fuku.yaml") }
			},
			error: errors.ErrInvalidConfig,
		},
		{
			name: "invalid yaml structure for unmarshal",
			setupFunc: func() func() {
				content := `version: "invalid_version_type"
services: "this should be a map not a string"
`

				err := os.WriteFile("fuku.yaml", []byte(content), 0644)
				if err != nil {
					t.Fatal(err)
				}

				return func() { os.Remove("fuku.yaml") }
			},
			error: errors.ErrFailedToParseConfig,
		},
		{
			name: "permission denied error",
			setupFunc: func() func() {
				err := os.WriteFile("fuku.yaml", []byte("test"), 0644)
				if err != nil {
					t.Fatal(err)
				}

				err = os.Chmod("fuku.yaml", 0000)
				if err != nil {
					t.Fatal(err)
				}

				return func() {
					_ = os.Chmod("fuku.yaml", 0644)

					os.Remove("fuku.yaml")
				}
			},
			error: errors.ErrFailedToReadConfig,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Chdir(t.TempDir())

			cleanup := tt.setupFunc()
			defer cleanup()

			cfg, topology, err := Load()

			if tt.error != nil {
				require.Error(t, err)
				assert.True(t, errors.Is(err, tt.error), "expected error %v, got %v", tt.error, err)
				assert.Nil(t, cfg)
				assert.Nil(t, topology)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, cfg)
				assert.NotNil(t, topology)
			}
		})
	}
}

func Test_Load_YmlFallback(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	content := `version: 1
services:
  api:
    dir: ./api
`

	err := os.WriteFile(ConfigFileAlt, []byte(content), 0644)
	require.NoError(t, err)

	cfg, topology, err := Load()
	require.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.NotNil(t, topology)
	assert.Contains(t, cfg.Services, "api")
}

func Test_Load_SentryDSN(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		expected string
	}{
		{
			name:     "reads SENTRY_DSN from environment",
			envValue: "https://key@sentry.io/123",
			expected: "https://key@sentry.io/123",
		},
		{
			name:     "empty when SENTRY_DSN not set",
			envValue: "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			t.Setenv("SENTRY_DSN", tt.envValue)

			cfg, _, err := Load()
			require.NoError(t, err)
			assert.Equal(t, tt.expected, cfg.SentryDSN)
		})
	}
}

func Test_Load_Telemetry(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		expected bool
	}{
		{
			name:     "disabled when FUKU_TELEMETRY_DISABLED=1",
			envValue: "1",
			expected: false,
		},
		{
			name:     "enabled when FUKU_TELEMETRY_DISABLED not set",
			envValue: "",
			expected: true,
		},
		{
			name:     "enabled when FUKU_TELEMETRY_DISABLED is not 1",
			envValue: "false",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			t.Setenv("FUKU_TELEMETRY_DISABLED", tt.envValue)

			cfg, _, err := Load()
			require.NoError(t, err)
			assert.Equal(t, tt.expected, cfg.Telemetry)
		})
	}
}

func Test_Load_ConcurrencyConfig(t *testing.T) {
	tests := []struct {
		name            string
		yaml            string
		expectedWorkers int
	}{
		{
			name:            "default workers when not specified",
			yaml:            `version: 1`,
			expectedWorkers: MaxWorkers,
		},
		{
			name: "custom workers value",
			yaml: `version: 1
concurrency:
  workers: 10`,
			expectedWorkers: 10,
		},
		{
			name: "workers value of 1",
			yaml: `version: 1
concurrency:
  workers: 1`,
			expectedWorkers: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Chdir(t.TempDir())

			err := os.WriteFile("fuku.yaml", []byte(tt.yaml), 0644)
			if err != nil {
				t.Fatal(err)
			}

			cfg, _, err := Load()
			require.NoError(t, err)
			assert.Equal(t, tt.expectedWorkers, cfg.Concurrency.Workers)
		})
	}
}

func Test_Load_RetryConfig(t *testing.T) {
	tests := []struct {
		name             string
		yaml             string
		expectedAttempts int
		expectedBackoff  time.Duration
	}{
		{
			name:             "default retry when not specified",
			yaml:             `version: 1`,
			expectedAttempts: RetryAttempts,
			expectedBackoff:  RetryBackoff,
		},
		{
			name: "custom retry values",
			yaml: `version: 1
retry:
  attempts: 5
  backoff: 1s`,
			expectedAttempts: 5,
			expectedBackoff:  time.Second,
		},
		{
			name: "retry with zero backoff",
			yaml: `version: 1
retry:
  attempts: 1
  backoff: 0s`,
			expectedAttempts: 1,
			expectedBackoff:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Chdir(t.TempDir())

			err := os.WriteFile("fuku.yaml", []byte(tt.yaml), 0644)
			if err != nil {
				t.Fatal(err)
			}

			cfg, _, err := Load()
			require.NoError(t, err)
			assert.Equal(t, tt.expectedAttempts, cfg.Retry.Attempts)
			assert.Equal(t, tt.expectedBackoff, cfg.Retry.Backoff)
		})
	}
}

func Test_Load_LogsConfig(t *testing.T) {
	tests := []struct {
		name           string
		yaml           string
		expectedBuffer int
	}{
		{
			name:           "default buffer when not specified",
			yaml:           `version: 1`,
			expectedBuffer: SocketLogsBufferSize,
		},
		{
			name: "custom buffer value",
			yaml: `version: 1
logs:
  buffer: 500`,
			expectedBuffer: 500,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Chdir(t.TempDir())

			err := os.WriteFile("fuku.yaml", []byte(tt.yaml), 0644)
			if err != nil {
				t.Fatal(err)
			}

			cfg, _, err := Load()
			require.NoError(t, err)
			assert.Equal(t, tt.expectedBuffer, cfg.Logs.Buffer)
		})
	}
}

func Test_Load_ServiceLogsConfig(t *testing.T) {
	tests := []struct {
		name         string
		yaml         string
		expectedLogs map[string]*Logs
		expectError  bool
	}{
		{
			name: "service with logs config both outputs",
			yaml: `version: 1
services:
  api:
    dir: ./api
    logs:
      output: [stdout, stderr]`,
			expectedLogs: map[string]*Logs{
				"api": {Output: []string{"stdout", "stderr"}},
			},
			expectError: false,
		},
		{
			name: "service with logs config stdout only",
			yaml: `version: 1
services:
  api:
    dir: ./api
    logs:
      output: [stdout]`,
			expectedLogs: map[string]*Logs{
				"api": {Output: []string{"stdout"}},
			},
			expectError: false,
		},
		{
			name: "service with logs config empty output",
			yaml: `version: 1
services:
  api:
    dir: ./api
    logs:
      output: []`,
			expectedLogs: map[string]*Logs{
				"api": {Output: []string{}},
			},
			expectError: false,
		},
		{
			name: "service without logs config",
			yaml: `version: 1
services:
  api:
    dir: ./api`,
			expectedLogs: map[string]*Logs{
				"api": nil,
			},
			expectError: false,
		},
		{
			name: "service with invalid logs output",
			yaml: `version: 1
services:
  api:
    dir: ./api
    logs:
      output: [invalid]`,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Chdir(t.TempDir())

			err := os.WriteFile("fuku.yaml", []byte(tt.yaml), 0644)
			if err != nil {
				t.Fatal(err)
			}

			cfg, _, err := Load()

			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)

				for name, expectedLogs := range tt.expectedLogs {
					service, ok := cfg.Services[name]
					assert.True(t, ok)
					assert.Equal(t, expectedLogs, service.Logs)
				}
			}
		})
	}
}

func Test_Load_WatchConfig(t *testing.T) {
	tests := []struct {
		name          string
		yaml          string
		expectedWatch map[string]*Watch
		expectError   bool
	}{
		{
			name: "service with watch config",
			yaml: `version: 1
services:
  api:
    dir: ./api
    watch:
      include: ["**/*.go"]
      ignore: ["*_test.go"]`,
			expectedWatch: map[string]*Watch{
				"api": {
					Include: []string{"**/*.go"},
					Ignore:  []string{"*_test.go"},
				},
			},
			expectError: false,
		},
		{
			name: "service with watch config and shared dirs",
			yaml: `version: 1
services:
  api:
    dir: ./api
    watch:
      include: ["**/*.go"]
      ignore: ["*_test.go"]
      shared: ["pkg/common", "pkg/models"]`,
			expectedWatch: map[string]*Watch{
				"api": {
					Include: []string{"**/*.go"},
					Ignore:  []string{"*_test.go"},
					Shared:  []string{"pkg/common", "pkg/models"},
				},
			},
			expectError: false,
		},
		{
			name: "service without watch config",
			yaml: `version: 1
services:
  api:
    dir: ./api`,
			expectedWatch: map[string]*Watch{
				"api": nil,
			},
			expectError: false,
		},
		{
			name: "service with watch but empty include",
			yaml: `version: 1
services:
  api:
    dir: ./api
    watch:
      include: []`,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Chdir(t.TempDir())

			err := os.WriteFile("fuku.yaml", []byte(tt.yaml), 0644)
			if err != nil {
				t.Fatal(err)
			}

			cfg, _, err := Load()

			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)

				for name, expectedWatch := range tt.expectedWatch {
					service, ok := cfg.Services[name]
					assert.True(t, ok)
					assert.Equal(t, expectedWatch, service.Watch)
				}
			}
		})
	}
}

func Test_Load_Override(t *testing.T) {
	tests := []struct {
		name             string
		base             string
		override         string
		overrideFile     string
		expectedServices []string
		expectedLevel    string
	}{
		{
			name:             "override merges with base",
			base:             "version: 1\nservices:\n  api:\n    dir: ./api\nlogging:\n  level: info\n",
			override:         "services:\n  api:\n    command: make dev\nlogging:\n  level: debug\n",
			overrideFile:     OverrideConfigFile,
			expectedServices: []string{"api"},
			expectedLevel:    "debug",
		},
		{
			name:             "override adds new service",
			base:             "version: 1\nservices:\n  api:\n    dir: ./api\n",
			override:         "services:\n  debug-tool:\n    dir: ./tools/debug\n",
			overrideFile:     OverrideConfigFile,
			expectedServices: []string{"api", "debug-tool"},
			expectedLevel:    LogLevel,
		},
		{
			name:             "fuku.override.yml fallback",
			base:             "version: 1\nservices:\n  api:\n    dir: ./api\n",
			override:         "services:\n  web:\n    dir: ./web\n",
			overrideFile:     OverrideConfigFileAlt,
			expectedServices: []string{"api", "web"},
			expectedLevel:    LogLevel,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			t.Chdir(dir)

			err := os.WriteFile(ConfigFile, []byte(tt.base), 0644)
			require.NoError(t, err)

			err = os.WriteFile(tt.overrideFile, []byte(tt.override), 0644)
			require.NoError(t, err)

			cfg, topology, err := Load()
			require.NoError(t, err)
			assert.NotNil(t, cfg)
			assert.NotNil(t, topology)

			for _, svc := range tt.expectedServices {
				assert.Contains(t, cfg.Services, svc)
			}

			assert.Equal(t, tt.expectedLevel, cfg.Logging.Level)
		})
	}
}

func Test_Load_Override_AllExtensionCombinations(t *testing.T) {
	tests := []struct {
		name         string
		baseFile     string
		overrideFile string
	}{
		{
			name:         "fuku.yaml + fuku.override.yaml",
			baseFile:     ConfigFile,
			overrideFile: OverrideConfigFile,
		},
		{
			name:         "fuku.yaml + fuku.override.yml",
			baseFile:     ConfigFile,
			overrideFile: OverrideConfigFileAlt,
		},
		{
			name:         "fuku.yml + fuku.override.yaml",
			baseFile:     ConfigFileAlt,
			overrideFile: OverrideConfigFile,
		},
		{
			name:         "fuku.yml + fuku.override.yml",
			baseFile:     ConfigFileAlt,
			overrideFile: OverrideConfigFileAlt,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			t.Chdir(dir)

			err := os.WriteFile(tt.baseFile, []byte("version: 1\nservices:\n  api:\n    dir: ./api\n"), 0644)
			require.NoError(t, err)

			err = os.WriteFile(tt.overrideFile, []byte("services:\n  web:\n    dir: ./web\n"), 0644)
			require.NoError(t, err)

			cfg, _, err := Load()
			require.NoError(t, err)
			assert.Contains(t, cfg.Services, "api")
			assert.Contains(t, cfg.Services, "web")
		})
	}
}

func Test_Load_Override_WithoutBaseIsIgnored(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	err := os.WriteFile(OverrideConfigFile, []byte("services:\n  api:\n    dir: ./api\n"), 0644)
	require.NoError(t, err)

	cfg, topology, err := Load()
	require.NoError(t, err)
	assert.Empty(t, cfg.Services)
	assert.True(t, topology.HasDefaultOnly)
}

func Test_Load_Override_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	err := os.WriteFile(ConfigFile, []byte("version: 1\nservices:\n  api:\n    dir: ./api\n"), 0644)
	require.NoError(t, err)

	err = os.WriteFile(OverrideConfigFile, []byte(":\ninvalid: [yaml"), 0644)
	require.NoError(t, err)

	cfg, topology, err := Load()
	require.Error(t, err)
	assert.True(t, errors.Is(err, errors.ErrFailedToParseConfig))
	assert.Nil(t, cfg)
	assert.Nil(t, topology)
}

func Test_Load_Override_MergedConfigPassesValidation(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	base := "version: 1\nservices:\n  api:\n    dir: ./api\n    watch:\n      include:\n        - '*.go'\n"
	override := "services:\n  api:\n    watch:\n      include:\n        - '*.templ'\n"

	err := os.WriteFile(ConfigFile, []byte(base), 0644)
	require.NoError(t, err)

	err = os.WriteFile(OverrideConfigFile, []byte(override), 0644)
	require.NoError(t, err)

	cfg, _, err := Load()
	require.NoError(t, err)
	assert.Equal(t, []string{"*.go", "*.templ"}, cfg.Services["api"].Watch.Include)
}

func Test_Load_Override_WatchNullRemovesBlock(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	base := "version: 1\nservices:\n  api:\n    dir: ./api\n    watch:\n      include:\n        - '*.go'\n"
	override := "services:\n  api:\n    watch: null\n"

	err := os.WriteFile(ConfigFile, []byte(base), 0644)
	require.NoError(t, err)

	err = os.WriteFile(OverrideConfigFile, []byte(override), 0644)
	require.NoError(t, err)

	cfg, _, err := Load()
	require.NoError(t, err)
	assert.Nil(t, cfg.Services["api"].Watch)
}

func Test_Load_Override_NullRespectedByRuntimeDefaults(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	base := "version: 1\nlogging:\n  level: debug\n"
	override := "logging: null\n"

	err := os.WriteFile(ConfigFile, []byte(base), 0644)
	require.NoError(t, err)

	err = os.WriteFile(OverrideConfigFile, []byte(override), 0644)
	require.NoError(t, err)

	cfg, _, err := Load()
	require.NoError(t, err)
	assert.Equal(t, LogLevel, cfg.Logging.Level)
	assert.Equal(t, LogFormat, cfg.Logging.Format)
}

func Test_Load_Override_AffectsDefaultsTierTopology(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	base := "version: 1\nservices:\n  api:\n    dir: ./api\n  web:\n    dir: ./web\n"
	override := "defaults:\n  tier: platform\n"

	err := os.WriteFile(ConfigFile, []byte(base), 0644)
	require.NoError(t, err)

	err = os.WriteFile(OverrideConfigFile, []byte(override), 0644)
	require.NoError(t, err)

	cfg, topology, err := Load()
	require.NoError(t, err)
	assert.Equal(t, []string{"platform"}, topology.Order)
	assert.Equal(t, "platform", cfg.Services["api"].Tier)
	assert.Equal(t, "platform", cfg.Services["web"].Tier)
}

func Test_Load_Override_AffectsPerServiceTierTopology(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	base := "version: 1\nservices:\n  api:\n    dir: ./api\n  web:\n    dir: ./web\n"
	override := "services:\n  api:\n    tier: foundation\n"

	err := os.WriteFile(ConfigFile, []byte(base), 0644)
	require.NoError(t, err)

	err = os.WriteFile(OverrideConfigFile, []byte(override), 0644)
	require.NoError(t, err)

	_, topology, err := Load()
	require.NoError(t, err)
	assert.Contains(t, topology.TierServices, "foundation")
	assert.Contains(t, topology.TierServices["foundation"], "api")
}

func Test_Load_Override_BaseAnchorInOverride(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	base := "version: 1\nx-watch: &watch\n  include:\n    - '*.go'\nservices:\n  api:\n    dir: ./api\n"
	override := "services:\n  api:\n    watch: *watch\n"

	err := os.WriteFile(ConfigFile, []byte(base), 0644)
	require.NoError(t, err)

	err = os.WriteFile(OverrideConfigFile, []byte(override), 0644)
	require.NoError(t, err)

	cfg, _, err := Load()
	require.NoError(t, err)
	assert.Equal(t, []string{"*.go"}, cfg.Services["api"].Watch.Include)
}

func Test_Load_Override_MergeKeyTierAgreesWithTopology(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	base := "version: 1\nx-svc: &svc\n  tier: foundation\nservices:\n  api:\n    <<: *svc\n    dir: ./api\n  web:\n    dir: ./web\n"

	err := os.WriteFile(ConfigFile, []byte(base), 0644)
	require.NoError(t, err)

	cfg, topology, err := Load()
	require.NoError(t, err)
	assert.Equal(t, "foundation", cfg.Services["api"].Tier)
	assert.Contains(t, topology.TierServices["foundation"], "api")
}

func Test_LoadFromFile(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	content := `version: 1
services:
  web:
    dir: ./web
`

	filePath := dir + "/custom.yaml"
	err := os.WriteFile(filePath, []byte(content), 0644)
	require.NoError(t, err)

	cfg, topology, err := LoadFromFile(filePath)
	require.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.NotNil(t, topology)
	assert.Contains(t, cfg.Services, "web")
}

func Test_LoadFromFile_NotFound(t *testing.T) {
	cfg, topology, err := LoadFromFile("/nonexistent/path/fuku.yaml")
	require.Error(t, err)
	assert.True(t, errors.Is(err, errors.ErrFailedToReadConfig))
	assert.Nil(t, cfg)
	assert.Nil(t, topology)
}

func Test_LoadFromFile_SkipsOverride(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	err := os.WriteFile(ConfigFile, []byte("version: 1\nservices:\n  api:\n    dir: ./api\n"), 0644)
	require.NoError(t, err)

	err = os.WriteFile(OverrideConfigFile, []byte("services:\n  debug-tool:\n    dir: ./tools/debug\n"), 0644)
	require.NoError(t, err)

	cfg, _, err := LoadFromFile(ConfigFile)
	require.NoError(t, err)
	assert.Contains(t, cfg.Services, "api")
	assert.NotContains(t, cfg.Services, "debug-tool")
}

func Test_LoadFromFile_ExplicitFukuYamlSkipsOverride(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	err := os.WriteFile(ConfigFile, []byte("version: 1\nservices:\n  api:\n    dir: ./api\n"), 0644)
	require.NoError(t, err)

	err = os.WriteFile(OverrideConfigFile, []byte("services:\n  debug-tool:\n    dir: ./tools/debug\n"), 0644)
	require.NoError(t, err)

	cfg, _, err := LoadFromFile("fuku.yaml")
	require.NoError(t, err)
	assert.Contains(t, cfg.Services, "api")
	assert.NotContains(t, cfg.Services, "debug-tool")
}

func Test_LoadEnv(t *testing.T) {
	tests := []struct {
		name     string
		goEnv    string
		envVar   string
		files    map[string]string
		existing string
		expected string
	}{
		{
			name:     "no .env files does not fail",
			goEnv:    "",
			envVar:   "TEST_LOADENV_NONE",
			expected: "",
		},
		{
			name:     "loads .env file",
			goEnv:    "",
			envVar:   "TEST_LOADENV_BASE",
			files:    map[string]string{".env": "TEST_LOADENV_BASE=from_dotenv\n"},
			expected: "from_dotenv",
		},
		{
			name:     "loads environment-specific .env file",
			goEnv:    "staging",
			envVar:   "TEST_LOADENV_SPECIFIC",
			files:    map[string]string{".env.staging": "TEST_LOADENV_SPECIFIC=from_staging\n"},
			expected: "from_staging",
		},
		{
			name:   "local file has highest priority",
			goEnv:  "staging",
			envVar: "TEST_LOADENV_LOCAL",
			files: map[string]string{
				".env.staging":       "TEST_LOADENV_LOCAL=from_env\n",
				".env.staging.local": "TEST_LOADENV_LOCAL=from_local\n",
			},
			expected: "from_local",
		},
		{
			name:   "environment-specific overrides base .env",
			goEnv:  "staging",
			envVar: "TEST_LOADENV_OVERRIDE",
			files: map[string]string{
				".env":         "TEST_LOADENV_OVERRIDE=from_base\n",
				".env.staging": "TEST_LOADENV_OVERRIDE=from_staging\n",
			},
			expected: "from_staging",
		},
		{
			name:     "does not override existing env vars",
			goEnv:    "",
			envVar:   "TEST_LOADENV_EXISTING",
			files:    map[string]string{".env": "TEST_LOADENV_EXISTING=from_dotenv\n"},
			existing: "already_set",
			expected: "already_set",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			t.Setenv("GO_ENV", tt.goEnv)

			os.Unsetenv(tt.envVar)

			if tt.existing != "" {
				t.Setenv(tt.envVar, tt.existing)
			}

			for name, content := range tt.files {
				err := os.WriteFile(name, []byte(content), 0644)
				if err != nil {
					t.Fatal(err)
				}
			}

			LoadEnv()

			assert.Equal(t, tt.expected, os.Getenv(tt.envVar))

			os.Unsetenv(tt.envVar)
		})
	}
}

func Test_ResolveEnv(t *testing.T) {
	tests := []struct {
		name     string
		goEnv    string
		expected string
	}{
		{
			name:     "Returns GO_ENV when set",
			goEnv:    "production",
			expected: "production",
		},
		{
			name:     "Defaults to production when empty",
			goEnv:    "",
			expected: EnvProduction,
		},
		{
			name:     "Returns test when set to test",
			goEnv:    EnvTest,
			expected: EnvTest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("GO_ENV", tt.goEnv)

			result := ResolveEnv()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_ResolveDefaultConfig(t *testing.T) {
	tests := []struct {
		name     string
		files    []string
		expected string
	}{
		{
			name:     "fuku.yaml wins when both exist",
			files:    []string{ConfigFile, ConfigFileAlt},
			expected: ConfigFile,
		},
		{
			name:     "falls back to fuku.yml",
			files:    []string{ConfigFileAlt},
			expected: ConfigFileAlt,
		},
		{
			name:     "empty when neither exists",
			files:    nil,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			t.Chdir(dir)

			for _, f := range tt.files {
				err := os.WriteFile(f, []byte("version: 1"), 0644)
				require.NoError(t, err)
			}

			result, err := resolveDefaultConfig()
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_ResolveOverrideFile(t *testing.T) {
	tests := []struct {
		name     string
		files    []string
		expected string
	}{
		{
			name:     "finds fuku.override.yaml",
			files:    []string{OverrideConfigFile},
			expected: OverrideConfigFile,
		},
		{
			name:     "falls back to fuku.override.yml",
			files:    []string{OverrideConfigFileAlt},
			expected: OverrideConfigFileAlt,
		},
		{
			name:     "fuku.override.yaml wins when both exist",
			files:    []string{OverrideConfigFile, OverrideConfigFileAlt},
			expected: OverrideConfigFile,
		},
		{
			name:     "empty when neither exists",
			files:    nil,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			t.Chdir(dir)

			for _, f := range tt.files {
				err := os.WriteFile(f, []byte(""), 0644)
				require.NoError(t, err)
			}

			result, err := resolveOverrideFile(ConfigFile)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_ResolveOverrideFile_StatError(t *testing.T) {
	dir := t.TempDir()

	sub := dir + "/restricted"
	require.NoError(t, os.MkdirAll(sub, 0755))
	require.NoError(t, os.WriteFile(sub+"/"+ConfigFile, []byte("version: 1"), 0644))
	require.NoError(t, os.WriteFile(sub+"/"+OverrideConfigFile, []byte(""), 0644))
	require.NoError(t, os.Chmod(sub, 0000))

	t.Cleanup(func() { os.Chmod(sub, 0755) })

	_, err := resolveOverrideFile(sub + "/" + ConfigFile)
	require.Error(t, err)
	assert.True(t, errors.Is(err, errors.ErrFailedToReadConfig))
}

func Test_ResolveExplicitConfig(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		files     []string
		expected  string
		expectErr bool
	}{
		{
			name:     "path verified and returned",
			path:     "custom.yaml",
			files:    []string{"custom.yaml"},
			expected: "custom.yaml",
		},
		{
			name:      "path not found returns error",
			path:      "missing.yaml",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			t.Chdir(dir)

			for _, f := range tt.files {
				err := os.WriteFile(f, []byte("version: 1"), 0644)
				require.NoError(t, err)
			}

			result, err := resolveExplicitConfig(tt.path)

			if tt.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}
