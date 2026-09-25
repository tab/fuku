package config

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fuku/internal/contracts"
	"fuku/internal/model"
)

func Test_loadDefault(t *testing.T) {
	tests := []struct {
		name      string
		setupFunc func() func()
		error     error
		loaded    bool
	}{
		{
			name: "no config file found - uses default",
			setupFunc: func() func() {
				return func() {}
			},
			error:  nil,
			loaded: true,
		},
		{
			name: "valid config file",
			setupFunc: func() func() {
				content := `version: 1
services:
  test-service:
    dir: ./test
profiles:
  test:
    - test-service
logging:
  level: debug
  format: json
`

				require.NoError(t, os.WriteFile("fuku.yaml", []byte(content), 0644))

				return func() { os.Remove("fuku.yaml") }
			},
			error:  nil,
			loaded: true,
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

				require.NoError(t, os.WriteFile("fuku.yaml", []byte(content), 0644))

				return func() { os.Remove("fuku.yaml") }
			},
			error:  nil,
			loaded: true,
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

				require.NoError(t, os.WriteFile("fuku.yaml", []byte(content), 0644))

				return func() { os.Remove("fuku.yaml") }
			},
			error: contracts.ErrInvalidConfig,
		},
		{
			name: "invalid yaml structure for unmarshal",
			setupFunc: func() func() {
				content := `version: "invalid_version_type"
services: "this should be a map not a string"
`

				require.NoError(t, os.WriteFile("fuku.yaml", []byte(content), 0644))

				return func() { os.Remove("fuku.yaml") }
			},
			error: ErrFailedToParseConfig,
		},
		{
			name: "permission denied error",
			setupFunc: func() func() {
				require.NoError(t, os.WriteFile("fuku.yaml", []byte("test"), 0644))
				require.NoError(t, os.Chmod("fuku.yaml", 0000))

				return func() {
					_ = os.Chmod("fuku.yaml", 0644)

					os.Remove("fuku.yaml")
				}
			},
			error: contracts.ErrFailedToReadConfig,
		},
		{
			name: "config file that cannot be checked",
			setupFunc: func() func() {
				require.NoError(t, os.Symlink(ConfigFile, ConfigFile))

				return func() {}
			},
			error: contracts.ErrFailedToReadConfig,
		},
		{
			name: "override file that cannot be checked",
			setupFunc: func() func() {
				require.NoError(t, os.WriteFile(ConfigFile, []byte("version: 1\n"), 0644))
				require.NoError(t, os.Symlink(OverrideConfigFile, OverrideConfigFile))

				return func() {}
			},
			error: contracts.ErrFailedToReadConfig,
		},
		{
			name: "override path that is a directory",
			setupFunc: func() func() {
				require.NoError(t, os.WriteFile(ConfigFile, []byte("version: 1\n"), 0644))
				require.NoError(t, os.Mkdir(OverrideConfigFile, 0755))

				return func() {}
			},
			error: contracts.ErrFailedToReadConfig,
		},
		{
			name: "config document that is not a mapping",
			setupFunc: func() func() {
				require.NoError(t, os.WriteFile(ConfigFile, []byte("- api\n"), 0644))

				return func() {}
			},
			error: ErrFailedToParseConfig,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Chdir(t.TempDir())

			cleanup := tt.setupFunc()
			defer cleanup()

			cfg, topology, _, err := loadDefault()

			require.ErrorIs(t, err, tt.error)
			assert.Equal(t, tt.loaded, cfg != nil)
			assert.Equal(t, tt.loaded, topology != nil)
		})
	}
}

func Test_loadDefault_YmlFallback(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	content := `version: 1
services:
  api:
    dir: ./api
`

	err := os.WriteFile(ConfigFileAlt, []byte(content), 0644)
	require.NoError(t, err)

	cfg, topology, _, err := loadDefault()
	require.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.NotNil(t, topology)
	assert.Contains(t, cfg.Services, "api")
}

func Test_loadDefault_SentryDSN(t *testing.T) {
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

			cfg, _, _, err := loadDefault()
			require.NoError(t, err)
			assert.Equal(t, tt.expected, cfg.SentryDSN)
		})
	}
}

func Test_loadDefault_Telemetry(t *testing.T) {
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

			cfg, _, _, err := loadDefault()
			require.NoError(t, err)
			assert.Equal(t, tt.expected, cfg.Telemetry)
		})
	}
}

func Test_loadDefault_EnvironmentFieldsIgnoreTheConfigFile(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("FUKU_TELEMETRY_DISABLED", "1")
	t.Setenv("FUKU_UPDATER_DISABLED", "1")
	t.Setenv("SENTRY_DSN", "")
	t.Setenv("GO_ENV", "")

	content := "version: 2\nappenv: development\nsentrydsn: https://yaml@sentry.io/1\ntelemetry: true\nupdater: true\nservices:\n  api:\n    dir: ./api\n"

	err := os.WriteFile(ConfigFile, []byte(content), 0644)
	require.NoError(t, err)

	cfg, _, _, err := loadDefault()
	require.NoError(t, err)
	assert.False(t, cfg.Telemetry)
	assert.False(t, cfg.Updater)
	assert.Empty(t, cfg.SentryDSN)
	assert.Equal(t, EnvProduction, cfg.AppEnv)
}

func Test_loadDefault_Updater(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		expected bool
	}{
		{
			name:     "disabled when FUKU_UPDATER_DISABLED=1",
			envValue: "1",
			expected: false,
		},
		{
			name:     "enabled when FUKU_UPDATER_DISABLED not set",
			envValue: "",
			expected: true,
		},
		{
			name:     "enabled when FUKU_UPDATER_DISABLED=0",
			envValue: "0",
			expected: true,
		},
		{
			name:     "enabled when FUKU_UPDATER_DISABLED=true",
			envValue: "true",
			expected: true,
		},
		{
			name:     "enabled when FUKU_UPDATER_DISABLED is arbitrary string",
			envValue: "anything",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			t.Setenv("FUKU_UPDATER_DISABLED", tt.envValue)

			cfg, _, _, err := loadDefault()
			require.NoError(t, err)
			assert.Equal(t, tt.expected, cfg.Updater)
		})
	}
}

func Test_loadDefault_ConcurrencyConfig(t *testing.T) {
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

			require.NoError(t, os.WriteFile("fuku.yaml", []byte(tt.yaml), 0644))

			cfg, _, _, err := loadDefault()
			require.NoError(t, err)
			assert.Equal(t, tt.expectedWorkers, cfg.Concurrency.Workers)
		})
	}
}

func Test_loadDefault_RetryConfig(t *testing.T) {
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

			require.NoError(t, os.WriteFile("fuku.yaml", []byte(tt.yaml), 0644))

			cfg, _, _, err := loadDefault()
			require.NoError(t, err)
			assert.Equal(t, tt.expectedAttempts, cfg.Retry.Attempts)
			assert.Equal(t, tt.expectedBackoff, cfg.Retry.Backoff)
		})
	}
}

func Test_loadDefault_LogsConfig(t *testing.T) {
	tests := []struct {
		name            string
		yaml            string
		expectedBuffer  int
		expectedHistory int
	}{
		{
			name:            "default values when not specified",
			yaml:            `version: 1`,
			expectedBuffer:  SocketLogsBufferSize,
			expectedHistory: SocketLogsHistorySize,
		},
		{
			name: "custom buffer value",
			yaml: `version: 1
logs:
  buffer: 500`,
			expectedBuffer:  500,
			expectedHistory: SocketLogsHistorySize,
		},
		{
			name: "custom history value",
			yaml: `version: 1
logs:
  history: 10000`,
			expectedBuffer:  SocketLogsBufferSize,
			expectedHistory: 10000,
		},
		{
			name: "custom buffer and history",
			yaml: `version: 1
logs:
  buffer: 500
  history: 2000`,
			expectedBuffer:  500,
			expectedHistory: 2000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Chdir(t.TempDir())

			require.NoError(t, os.WriteFile("fuku.yaml", []byte(tt.yaml), 0644))

			cfg, _, _, err := loadDefault()
			require.NoError(t, err)
			assert.Equal(t, tt.expectedBuffer, cfg.Logs.Buffer)
			assert.Equal(t, tt.expectedHistory, cfg.Logs.History)
		})
	}
}

func Test_loadDefault_ServiceLogsConfig(t *testing.T) {
	tests := []struct {
		name         string
		yaml         string
		expectedLogs map[string]*Logs
		error        error
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
		},
		{
			name: "service with invalid logs output",
			yaml: `version: 1
services:
  api:
    dir: ./api
    logs:
      output: [invalid]`,
			error: ErrInvalidLogsOutput,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Chdir(t.TempDir())

			require.NoError(t, os.WriteFile("fuku.yaml", []byte(tt.yaml), 0644))

			cfg, _, _, err := loadDefault()

			require.ErrorIs(t, err, tt.error)

			for name, expectedLogs := range tt.expectedLogs {
				require.Contains(t, cfg.Services, name)
				assert.Equal(t, expectedLogs, cfg.Services[name].Logs)
			}
		})
	}
}

func Test_loadDefault_WatchConfig(t *testing.T) {
	tests := []struct {
		name          string
		yaml          string
		expectedWatch map[string]*Watch
		error         error
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
		},
		{
			name: "service with watch but empty include",
			yaml: `version: 1
services:
  api:
    dir: ./api
    watch:
      include: []`,
			error: ErrWatchIncludeRequired,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Chdir(t.TempDir())

			require.NoError(t, os.WriteFile("fuku.yaml", []byte(tt.yaml), 0644))

			cfg, _, _, err := loadDefault()

			require.ErrorIs(t, err, tt.error)

			for name, expectedWatch := range tt.expectedWatch {
				require.Contains(t, cfg.Services, name)
				assert.Equal(t, expectedWatch, cfg.Services[name].Watch)
			}
		})
	}
}

func Test_loadDefault_Exclude(t *testing.T) {
	tests := []struct {
		name     string
		yaml     string
		expected []string
	}{
		{
			name: "exclude list parsed",
			yaml: `version: 1
services:
  api:
    dir: ./api
  web:
    dir: ./web
exclude:
  - web
`,
			expected: []string{"web"},
		},
		{
			name: "exclude empty when not set",
			yaml: `version: 1
services:
  api:
    dir: ./api
`,
			expected: nil,
		},
		{
			name: "exclude normalizes whitespace and dedups",
			yaml: `version: 1
services:
  api:
    dir: ./api
  web:
    dir: ./web
  worker:
    dir: ./worker
exclude:
  - "  web  "
  - ""
  - web
  - worker
`,
			expected: []string{"web", "worker"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Chdir(t.TempDir())

			err := os.WriteFile(ConfigFile, []byte(tt.yaml), 0644)
			require.NoError(t, err)

			cfg, _, _, err := loadDefault()
			require.NoError(t, err)
			assert.Equal(t, tt.expected, cfg.Exclude)
		})
	}
}

func Test_loadDefault_Exclude_OverrideConcatenatesAndDedups(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	base := `version: 1
services:
  api:
    dir: ./api
  web:
    dir: ./web
  worker:
    dir: ./worker
exclude:
  - web
`
	override := `exclude:
  - worker
  - web
`

	err := os.WriteFile(ConfigFile, []byte(base), 0644)
	require.NoError(t, err)

	err = os.WriteFile(OverrideConfigFile, []byte(override), 0644)
	require.NoError(t, err)

	cfg, _, _, err := loadDefault()
	require.NoError(t, err)
	assert.Equal(t, []string{"web", "worker"}, cfg.Exclude)
}

func Test_loadDefault_Override(t *testing.T) {
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
			expectedLevel:    DefaultLogLevel,
		},
		{
			name:             "fuku.override.yml fallback",
			base:             "version: 1\nservices:\n  api:\n    dir: ./api\n",
			override:         "services:\n  web:\n    dir: ./web\n",
			overrideFile:     OverrideConfigFileAlt,
			expectedServices: []string{"api", "web"},
			expectedLevel:    DefaultLogLevel,
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

			cfg, topology, _, err := loadDefault()
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

func Test_loadDefault_Override_AllExtensionCombinations(t *testing.T) {
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

			cfg, _, source, err := loadDefault()
			require.NoError(t, err)
			assert.Contains(t, cfg.Services, "api")
			assert.Contains(t, cfg.Services, "web")
			assert.Equal(t, files{path: tt.baseFile, override: tt.overrideFile}, source)
		})
	}
}

func Test_loadDefault_Override_WithoutBaseIsIgnored(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	err := os.WriteFile(OverrideConfigFile, []byte("services:\n  api:\n    dir: ./api\n"), 0644)
	require.NoError(t, err)

	cfg, topology, _, err := loadDefault()
	require.NoError(t, err)
	assert.Empty(t, cfg.Services)
	assert.True(t, topology.DefaultOnly())
}

func Test_loadDefault_Override_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	err := os.WriteFile(ConfigFile, []byte("version: 1\nservices:\n  api:\n    dir: ./api\n"), 0644)
	require.NoError(t, err)

	err = os.WriteFile(OverrideConfigFile, []byte(":\ninvalid: [yaml"), 0644)
	require.NoError(t, err)

	cfg, topology, _, err := loadDefault()
	require.ErrorIs(t, err, ErrFailedToParseConfig)
	assert.Nil(t, cfg)
	assert.Nil(t, topology)
}

func Test_loadDefault_Override_MergedConfigPassesValidation(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	base := "version: 1\nservices:\n  api:\n    dir: ./api\n    watch:\n      include:\n        - '*.go'\n"
	override := "services:\n  api:\n    watch:\n      include:\n        - '*.templ'\n"

	err := os.WriteFile(ConfigFile, []byte(base), 0644)
	require.NoError(t, err)

	err = os.WriteFile(OverrideConfigFile, []byte(override), 0644)
	require.NoError(t, err)

	cfg, _, _, err := loadDefault()
	require.NoError(t, err)
	assert.Equal(t, []string{"*.go", "*.templ"}, cfg.Services["api"].Watch.Include)
}

func Test_loadDefault_Override_WatchNullRemovesBlock(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	base := "version: 1\nservices:\n  api:\n    dir: ./api\n    watch:\n      include:\n        - '*.go'\n"
	override := "services:\n  api:\n    watch: null\n"

	err := os.WriteFile(ConfigFile, []byte(base), 0644)
	require.NoError(t, err)

	err = os.WriteFile(OverrideConfigFile, []byte(override), 0644)
	require.NoError(t, err)

	cfg, _, _, err := loadDefault()
	require.NoError(t, err)
	assert.Nil(t, cfg.Services["api"].Watch)
}

func Test_loadDefault_Override_NullRespectedByRuntimeDefaults(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	base := "version: 1\nlogging:\n  level: debug\n"
	override := "logging: null\n"

	err := os.WriteFile(ConfigFile, []byte(base), 0644)
	require.NoError(t, err)

	err = os.WriteFile(OverrideConfigFile, []byte(override), 0644)
	require.NoError(t, err)

	cfg, _, _, err := loadDefault()
	require.NoError(t, err)
	assert.Equal(t, DefaultLogLevel, cfg.Logging.Level)
	assert.Equal(t, DefaultLogFormat, cfg.Logging.Format)
}

func Test_loadDefault_Override_AffectsDefaultsTierTopology(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	base := "version: 1\nservices:\n  api:\n    dir: ./api\n  web:\n    dir: ./web\n"
	override := "defaults:\n  tier: platform\n"

	err := os.WriteFile(ConfigFile, []byte(base), 0644)
	require.NoError(t, err)

	err = os.WriteFile(OverrideConfigFile, []byte(override), 0644)
	require.NoError(t, err)

	cfg, topology, _, err := loadDefault()
	require.NoError(t, err)
	assert.Equal(t, []string{"platform"}, topology.Order)
	assert.Equal(t, "platform", cfg.Services["api"].Tier)
	assert.Equal(t, "platform", cfg.Services["web"].Tier)
}

func Test_loadDefault_Override_AffectsPerServiceTierTopology(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	base := "version: 1\nservices:\n  api:\n    dir: ./api\n  web:\n    dir: ./web\n"
	override := "services:\n  api:\n    tier: foundation\n"

	err := os.WriteFile(ConfigFile, []byte(base), 0644)
	require.NoError(t, err)

	err = os.WriteFile(OverrideConfigFile, []byte(override), 0644)
	require.NoError(t, err)

	_, topology, _, err := loadDefault()
	require.NoError(t, err)
	assert.Contains(t, topology.TierServices, "foundation")
	assert.Contains(t, topology.TierServices["foundation"], "api")
}

func Test_loadDefault_Override_BaseAnchorInOverride(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	base := "version: 1\nx-watch: &watch\n  include:\n    - '*.go'\nservices:\n  api:\n    dir: ./api\n"
	override := "services:\n  api:\n    watch: *watch\n"

	err := os.WriteFile(ConfigFile, []byte(base), 0644)
	require.NoError(t, err)

	err = os.WriteFile(OverrideConfigFile, []byte(override), 0644)
	require.NoError(t, err)

	cfg, _, _, err := loadDefault()
	require.NoError(t, err)
	assert.Equal(t, []string{"*.go"}, cfg.Services["api"].Watch.Include)
}

func Test_loadDefault_Override_BaseAnchorKeepsBaseListsSingle(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	base := "version: 1\nx-common: &common\n  logs:\n    output: [stdout]\n  watch:\n    include: ['*.go']\nservices:\n  api:\n    <<: *common\n    dir: ./api\n"
	override := "services:\n  web:\n    <<: *common\n    dir: ./web\n"

	err := os.WriteFile(ConfigFile, []byte(base), 0644)
	require.NoError(t, err)

	err = os.WriteFile(OverrideConfigFile, []byte(override), 0644)
	require.NoError(t, err)

	cfg, _, _, err := loadDefault()
	require.NoError(t, err)
	assert.Equal(t, []string{"stdout"}, cfg.Services["api"].Logs.Output)
	assert.Equal(t, []string{"*.go"}, cfg.Services["api"].Watch.Include)
	assert.Equal(t, []string{"stdout"}, cfg.Services["web"].Logs.Output)
	assert.Equal(t, []string{"*.go"}, cfg.Services["web"].Watch.Include)
}

func Test_loadDefault_Override_MergeKeyTierAgreesWithTopology(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	base := "version: 1\nx-svc: &svc\n  tier: foundation\nservices:\n  api:\n    <<: *svc\n    dir: ./api\n  web:\n    dir: ./web\n"

	err := os.WriteFile(ConfigFile, []byte(base), 0644)
	require.NoError(t, err)

	cfg, topology, _, err := loadDefault()
	require.NoError(t, err)
	assert.Equal(t, "foundation", cfg.Services["api"].Tier)
	assert.Contains(t, topology.TierServices["foundation"], "api")
}

func Test_loadDefault_AliasedServiceTopology(t *testing.T) {
	tests := []struct {
		name             string
		content          string
		expectedOrder    []string
		expectedServices map[string][]string
	}{
		{
			name:             "anchor inside services",
			content:          "version: 1\nservices:\n  storage: &base\n    dir: ./storage\n    tier: foundation\n  api: *base\n  web:\n    dir: ./web\n    tier: edge\n",
			expectedOrder:    []string{"foundation", "edge"},
			expectedServices: map[string][]string{"foundation": {"api", "storage"}, "edge": {"web"}},
		},
		{
			name:             "top-level anchor",
			content:          "version: 1\nx-base: &base\n  dir: ./api\n  tier: platform\nservices:\n  api: *base\n  web:\n    dir: ./web\n",
			expectedOrder:    []string{"platform", model.TierDefault},
			expectedServices: map[string][]string{"platform": {"api"}, model.TierDefault: {"web"}},
		},
		{
			name:             "aliased defaults",
			content:          "version: 1\nx-defaults: &d\n  tier: platform\ndefaults: *d\nservices:\n  api:\n    dir: ./api\n",
			expectedOrder:    []string{"platform"},
			expectedServices: map[string][]string{"platform": {"api"}},
		},
		{
			name:             "aliased services",
			content:          "version: 1\nx-services: &s\n  api:\n    dir: ./api\n    tier: platform\nservices: *s\n",
			expectedOrder:    []string{"platform"},
			expectedServices: map[string][]string{"platform": {"api"}},
		},
		{
			name:             "aliased tier",
			content:          "version: 1\nx-tier: &t platform\nservices:\n  api:\n    dir: ./api\n    tier: *t\n",
			expectedOrder:    []string{"platform"},
			expectedServices: map[string][]string{"platform": {"api"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			require.NoError(t, os.WriteFile(ConfigFile, []byte(tt.content), 0644))

			_, topology, _, err := loadDefault()

			require.NoError(t, err)
			assert.Equal(t, tt.expectedOrder, topology.Order)
			assert.Equal(t, tt.expectedServices, topology.TierServices)
		})
	}
}

func Test_loadDefault_RejectsAnEmptyService(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{
			name:    "explicit null body",
			content: "services: {api: null}\n",
		},
		{
			name:    "empty mapping",
			content: "services: {api: {}}\n",
		},
		{
			name:    "missing body beside a valid service",
			content: "services:\n  api:\n  web:\n    dir: web\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			require.NoError(t, os.WriteFile(ConfigFile, []byte(tt.content), 0644))

			cfg, topology, _, err := loadDefault()

			require.ErrorIs(t, err, contracts.ErrInvalidConfig)
			require.ErrorIs(t, err, ErrEmptyService)
			require.ErrorContains(t, err, "service api")
			assert.Nil(t, cfg)
			assert.Nil(t, topology)
		})
	}
}

func Test_loadFromFile(t *testing.T) {
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

	cfg, topology, _, err := loadFromFile(filePath)
	require.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.NotNil(t, topology)
	assert.Contains(t, cfg.Services, "web")
}

func Test_loadFromFile_NotFound(t *testing.T) {
	cfg, topology, _, err := loadFromFile("/nonexistent/path/fuku.yaml")
	require.ErrorIs(t, err, contracts.ErrFailedToReadConfig)
	assert.Nil(t, cfg)
	assert.Nil(t, topology)
}

func Test_loadFromFile_Directory(t *testing.T) {
	dir := t.TempDir()

	cfg, topology, _, err := loadFromFile(dir)

	require.ErrorIs(t, err, contracts.ErrFailedToReadConfig)
	assert.Nil(t, cfg)
	assert.Nil(t, topology)
}

func Test_loadFromFile_SkipsOverride(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	err := os.WriteFile(ConfigFile, []byte("version: 1\nservices:\n  api:\n    dir: ./api\n"), 0644)
	require.NoError(t, err)

	err = os.WriteFile(OverrideConfigFile, []byte("services:\n  debug-tool:\n    dir: ./tools/debug\n"), 0644)
	require.NoError(t, err)

	cfg, _, source, err := loadFromFile(ConfigFile)
	require.NoError(t, err)
	assert.Contains(t, cfg.Services, "api")
	assert.NotContains(t, cfg.Services, "debug-tool")
	assert.Equal(t, files{path: ConfigFile, override: OverrideConfigFile}, source)
}

func Test_loadEnv(t *testing.T) {
	tests := []struct {
		name     string
		before   func(t *testing.T)
		goEnv    string
		envVar   string
		files    map[string]string
		expected string
	}{
		{
			name:     "no .env files does not fail",
			before:   func(*testing.T) {},
			goEnv:    "",
			envVar:   "TEST_LOADENV_NONE",
			expected: "",
		},
		{
			name:     "loads .env file",
			before:   func(*testing.T) {},
			goEnv:    "",
			envVar:   "TEST_LOADENV_BASE",
			files:    map[string]string{".env": "TEST_LOADENV_BASE=from_dotenv\n"},
			expected: "from_dotenv",
		},
		{
			name:     "loads environment-specific .env file",
			before:   func(*testing.T) {},
			goEnv:    "staging",
			envVar:   "TEST_LOADENV_SPECIFIC",
			files:    map[string]string{".env.staging": "TEST_LOADENV_SPECIFIC=from_staging\n"},
			expected: "from_staging",
		},
		{
			name:   "local file has highest priority",
			before: func(*testing.T) {},
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
			before: func(*testing.T) {},
			goEnv:  "staging",
			envVar: "TEST_LOADENV_OVERRIDE",
			files: map[string]string{
				".env":         "TEST_LOADENV_OVERRIDE=from_base\n",
				".env.staging": "TEST_LOADENV_OVERRIDE=from_staging\n",
			},
			expected: "from_staging",
		},
		{
			name: "does not override existing env vars",
			before: func(t *testing.T) {
				t.Setenv("TEST_LOADENV_EXISTING", "already_set")
			},
			goEnv:    "",
			envVar:   "TEST_LOADENV_EXISTING",
			files:    map[string]string{".env": "TEST_LOADENV_EXISTING=from_dotenv\n"},
			expected: "already_set",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			t.Setenv("GO_ENV", tt.goEnv)

			os.Unsetenv(tt.envVar)

			tt.before(t)

			for name, content := range tt.files {
				require.NoError(t, os.WriteFile(name, []byte(content), 0644))
			}

			loadEnv()

			assert.Equal(t, tt.expected, os.Getenv(tt.envVar))

			os.Unsetenv(tt.envVar)
		})
	}
}

func Test_resolveEnv(t *testing.T) {
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
			goEnv:    "test",
			expected: "test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("GO_ENV", tt.goEnv)

			result := resolveEnv()
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
	require.ErrorIs(t, err, contracts.ErrFailedToReadConfig)
}

func Test_ResolveExplicitConfig(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		files    []string
		expected string
		error    error
	}{
		{
			name:     "path verified and returned",
			path:     "custom.yaml",
			files:    []string{"custom.yaml"},
			expected: "custom.yaml",
		},
		{
			name:  "path not found returns error",
			path:  "missing.yaml",
			error: contracts.ErrFailedToReadConfig,
		},
		{
			name:  "path under a file fails the check",
			path:  "custom.yaml/fuku.yaml",
			files: []string{"custom.yaml"},
			error: contracts.ErrFailedToReadConfig,
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

			require.ErrorIs(t, err, tt.error)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_Telemetry(t *testing.T) {
	tests := []struct {
		name     string
		before   func(t *testing.T)
		disabled string
		env      string
		expected model.Telemetry
	}{
		{
			name:     "enabled by default in production",
			before:   func(*testing.T) {},
			expected: model.Telemetry{Enabled: true, Environment: EnvProduction},
		},
		{
			name: "carries the DSN and the environment",
			before: func(t *testing.T) {
				t.Setenv("SENTRY_DSN", "https://env@sentry.io/1")
			},
			env:      "test",
			expected: model.Telemetry{Enabled: true, DSN: "https://env@sentry.io/1", Environment: "test"},
		},
		{
			name:     "the opt-out disables it",
			before:   func(*testing.T) {},
			disabled: "1",
			expected: model.Telemetry{Environment: EnvProduction},
		},
		{
			name: "reads the DSN from the env files",
			before: func(t *testing.T) {
				require.NoError(t, os.WriteFile(".env", []byte("SENTRY_DSN=https://file@sentry.io/3\n"), 0o644))
			},
			expected: model.Telemetry{Enabled: true, DSN: "https://file@sentry.io/3", Environment: EnvProduction},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			t.Setenv("FUKU_TELEMETRY_DISABLED", tt.disabled)
			t.Setenv("SENTRY_DSN", "")
			t.Setenv("GO_ENV", tt.env)

			os.Unsetenv("SENTRY_DSN")

			tt.before(t)

			assert.Equal(t, tt.expected, Telemetry())
		})
	}
}

func Test_parseConfig_KeepsTheCause(t *testing.T) {
	tests := []struct {
		name   string
		config *Config
		data   []byte
		cause  string
	}{
		{
			name:   "invalid yaml carries the yaml error",
			config: defaultConfig(),
			data:   []byte("services: [\n"),
			cause:  "did not find expected node content",
		},
		{
			name:   "a type mismatch carries the decode error",
			config: defaultConfig(),
			data:   []byte("services: \"not a map\"\n"),
			cause:  "'Services' expected type 'map[string]*config.Service'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, topology, err := parseConfig(tt.config, tt.data)

			require.ErrorIs(t, err, ErrFailedToParseConfig)
			require.ErrorContains(t, err, tt.cause)
			assert.Nil(t, cfg)
			assert.Nil(t, topology)
		})
	}
}
