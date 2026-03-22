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
	assert.Contains(t, output, "service_ready service=echo-api")
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
	assert.Contains(t, output, "service_ready service=echo-api")
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
	assert.Contains(t, output, "service_ready service=echo-api")
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
