package e2e

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_YmlConfig_StartServices(t *testing.T) {
	runner := NewRunner(t, "testdata/yml-config")
	defer runner.Stop()

	err := runner.Start("default")
	require.NoError(t, err)

	err = runner.WaitForServiceStarted("echo-api", 10*time.Second)
	require.NoError(t, err)

	err = runner.WaitForRunning(15 * time.Second)
	require.NoError(t, err)

	output := runner.Output()

	assert.Contains(t, output, "profile_resolved profile=default")
	assert.Contains(t, output, "service_ready")
	assert.Contains(t, output, "service=echo-api")
}

func Test_ConfigFlag_CrossDirectory(t *testing.T) {
	configPath, err := filepath.Abs("testdata/yml-config/fuku.yml")
	require.NoError(t, err)

	runner := NewRunner(t, t.TempDir())
	defer runner.Stop()

	err = runner.StartWithConfig(configPath, "default")
	require.NoError(t, err)

	err = runner.WaitForServiceStarted("echo-api", 10*time.Second)
	require.NoError(t, err)

	err = runner.WaitForRunning(15 * time.Second)
	require.NoError(t, err)

	output := runner.Output()

	assert.Contains(t, output, "profile_resolved profile=default")
	assert.Contains(t, output, "service_ready")
	assert.Contains(t, output, "service=echo-api")
}

func Test_ConfigFlag_RelativePath(t *testing.T) {
	runner := NewRunner(t, "testdata")
	defer runner.Stop()

	err := runner.StartWithConfig("yml-config/fuku.yml", "default")
	require.NoError(t, err)

	err = runner.WaitForServiceStarted("echo-api", 10*time.Second)
	require.NoError(t, err)

	err = runner.WaitForRunning(15 * time.Second)
	require.NoError(t, err)

	output := runner.Output()

	assert.Contains(t, output, "profile_resolved profile=default")
	assert.Contains(t, output, "service_ready")
	assert.Contains(t, output, "service=echo-api")
}

func Test_NoConfigFile(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{
			name: "run",
			args: []string{"run", "default", "--no-ui"},
		},
		{
			name: "stop",
			args: []string{"stop", "default"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RunOnce(t, t.TempDir(), tt.args...)

			assert.Equal(t, 1, result.ExitCode)
			assert.Contains(t, result.Stderr, "no services defined")
		})
	}
}

func Test_NoServicesDefined(t *testing.T) {
	tests := []struct {
		name string
		yaml string
	}{
		{
			name: "no services section",
			yaml: "version: 1\n",
		},
		{
			name: "empty services section",
			yaml: "version: 1\nservices:\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()

			err := os.WriteFile(filepath.Join(dir, "fuku.yaml"), []byte(tt.yaml), 0644)
			require.NoError(t, err)

			result := RunOnce(t, dir, "run", "default", "--no-ui")

			assert.Equal(t, 1, result.ExitCode)
			assert.Contains(t, result.Stderr, "no services defined")
		})
	}
}

func Test_ConfigFlag_MissingFile(t *testing.T) {
	dir := t.TempDir()
	result := RunOnce(t, dir, "--config", filepath.Join(dir, "nonexistent.yaml"), "run", "default", "--no-ui")

	assert.Equal(t, 1, result.ExitCode)
	assert.Contains(t, result.Stderr, "failed to read config file")
}

func Test_ConfigFlag_MissingDirectory(t *testing.T) {
	result := RunOnce(t, t.TempDir(), "--config", "/tmp/e2e-nonexistent-dir/fuku.yaml", "run", "default", "--no-ui")

	assert.Equal(t, 1, result.ExitCode)
	assert.Contains(t, result.Stderr, "failed to read config file")
}

func Test_ProfileReferenceUndefined_FailsFast(t *testing.T) {
	dir := t.TempDir()

	yaml := `version: 1

services:
  echo-api:
    dir: echo-api

profiles:
  default: "*"
  backend: [echo-api, nonexistent-service]
`

	err := os.WriteFile(filepath.Join(dir, "fuku.yaml"), []byte(yaml), 0644)
	require.NoError(t, err)

	result := RunOnce(t, dir, "run", "backend", "--no-ui")

	assert.Equal(t, 1, result.ExitCode)
	assert.Contains(t, result.Stderr, "profile references undefined service")
	assert.Contains(t, result.Stderr, "backend")
	assert.Contains(t, result.Stderr, "nonexistent-service")
}

func Test_Config_InvalidLogLevel(t *testing.T) {
	dir := t.TempDir()

	yaml := `version: 1

services:
  api:
    dir: .
    command: sleep 60

logging:
  level: trace
`

	require.NoError(t, os.WriteFile(filepath.Join(dir, "fuku.yaml"), []byte(yaml), 0o600))

	result := RunOnce(t, dir, "run", "default", "--no-ui")

	assert.Equal(t, 1, result.ExitCode)
	assert.Contains(t, result.Stderr, "Error: invalid configuration")
	assert.Contains(t, result.Stderr, "'trace'")
	assert.NotContains(t, result.Stdout, "Started service")
}

func Test_Config_OverrideSymlinkLoop(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{
			name: "default load",
			args: []string{"run", "default", "--no-ui"},
		},
		{
			name: "config flag",
			args: []string{"--config", "fuku.yaml", "run", "default", "--no-ui"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			override := filepath.Join(dir, "fuku.override.yaml")

			require.NoError(t, os.WriteFile(filepath.Join(dir, "fuku.yaml"), []byte("version: 1\nservices:\n  api:\n    dir: .\n"), 0o600))
			require.NoError(t, os.Symlink(override, override))

			result := RunOnce(t, dir, tt.args...)

			assert.Equal(t, 1, result.ExitCode)
			assert.Contains(t, result.Stderr, "Error: failed to read config file")
			assert.Contains(t, result.Stderr, "fuku.override.yaml")
			assert.NotContains(t, result.Stdout, "Started service")
		})
	}
}

func Test_Config_AliasedServiceKeepsTier(t *testing.T) {
	dir := t.TempDir()

	yaml := `version: 1

x-base: &base
  dir: .
  command: sleep 60
  tier: foundation

services:
  db:
    dir: .
    command: sleep 60
    tier: foundation
  api: *base
  web:
    dir: .
    command: sleep 60
`

	require.NoError(t, os.WriteFile(filepath.Join(dir, "fuku.yaml"), []byte(yaml), 0o600))

	result := RunOnce(t, dir, "doctor", "--json")

	require.Equal(t, 0, result.ExitCode)

	checks := doctorChecks(t, result.Stdout)
	tiers := checks["topology.tiers"]

	assert.Equal(t, "foundation → default", tiers.Summary)
	assert.Equal(t, "2 services", tiers.Details["foundation"])
	assert.Equal(t, "profile 'default' resolves to 3 services", checks["topology.profile"].Summary)
}

func Test_Config_TierDefaultsAndNormalization(t *testing.T) {
	dir := t.TempDir()

	yaml := `version: 1

defaults:
  tier: Platform

services:
  db:
    dir: .
    tier: " Foundation "
  api:
    dir: .
  web:
    dir: .
    tier: edge
`

	require.NoError(t, os.WriteFile(filepath.Join(dir, "fuku.yaml"), []byte(yaml), 0o600))

	result := RunOnce(t, dir, "doctor", "--json")

	require.Equal(t, 0, result.ExitCode)

	tiers := doctorChecks(t, result.Stdout)["topology.tiers"]

	assert.Equal(t, "ok", tiers.Status)
	assert.Equal(t, "foundation → platform → edge", tiers.Summary)
}
