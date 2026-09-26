package e2e

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_Doctor_NoConfig(t *testing.T) {
	dir := t.TempDir()

	result := RunOnce(t, dir, "doctor")

	assert.Equal(t, 2, result.ExitCode)
	assert.Contains(t, result.Stdout, "fuku doctor v")
	assert.Contains(t, result.Stdout, "config.file")
	assert.Contains(t, result.Stdout, "no fuku.yaml found")
	assert.Contains(t, result.Stdout, "✗")
}

func Test_Doctor_ValidConfig_TextReport(t *testing.T) {
	result := RunOnce(t, "testdata/yml-config", "doctor")

	assert.Equal(t, 0, result.ExitCode)
	assert.Contains(t, result.Stdout, "Configuration")
	assert.Contains(t, result.Stdout, "Services")
	assert.Contains(t, result.Stdout, "Runtime")
	assert.Contains(t, result.Stdout, "config.file")
	assert.Contains(t, result.Stdout, "active profile: default")
}

func Test_Doctor_ValidConfig_Summary(t *testing.T) {
	result := RunOnce(t, "testdata/yml-config", "doctor", "--summary")

	assert.Equal(t, 0, result.ExitCode)
	assert.Contains(t, result.Stdout, "config.file")
	assert.NotContains(t, result.Stdout, "Notes\n")
	assert.NotContains(t, result.Stdout, "remediation")
}

func Test_Doctor_ValidConfig_JSON(t *testing.T) {
	result := RunOnce(t, "testdata/yml-config", "doctor", "--json")

	assert.Equal(t, 0, result.ExitCode)

	var report map[string]any
	require.NoError(t, json.Unmarshal([]byte(result.Stdout), &report))

	assert.InDelta(t, float64(1), report["schemaVersion"], 0)
	assert.Contains(t, report, "tally")
	assert.Contains(t, report, "checks")
	assert.Contains(t, report, "sections")

	checks, ok := report["checks"].(map[string]any)
	require.True(t, ok, "checks must be an object")
	assert.Contains(t, checks, "config.file")
	assert.Contains(t, checks, "services.directories")
	assert.Contains(t, checks, "runtime.sockets")
}

func Test_Doctor_InvalidYAML_Fails(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "fuku.yaml"), []byte("services: [unterminated"), 0o600))

	result := RunOnce(t, dir, "doctor")

	assert.Equal(t, 2, result.ExitCode)
	assert.Contains(t, result.Stdout, "config.file")
	assert.Contains(t, result.Stdout, "failed to load")
}

func Test_Doctor_InvalidSchema_Fails(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "api"), 0o755))

	yaml := `version: 1

services:
  api:
    dir: api
    readiness:
      type: bogus
      url: http://localhost:8080
profiles:
  default: "*"
`

	require.NoError(t, os.WriteFile(filepath.Join(dir, "fuku.yaml"), []byte(yaml), 0o600))

	result := RunOnce(t, dir, "doctor")

	assert.Equal(t, 2, result.ExitCode)
	assert.Contains(t, result.Stdout, "config.validate")
	assert.Contains(t, result.Stdout, "schema validation failed")
	assert.Contains(t, result.Stdout, "found and parsed")
	assert.NotContains(t, result.Stdout, "failed to load")
}

func Test_Doctor_MissingServiceDir_Warns(t *testing.T) {
	dir := t.TempDir()

	yaml := `version: 1

services:
  api:
    dir: nonexistent-api
profiles:
  default: "*"
`

	require.NoError(t, os.WriteFile(filepath.Join(dir, "fuku.yaml"), []byte(yaml), 0o600))

	result := RunOnce(t, dir, "doctor")

	assert.Equal(t, 0, result.ExitCode, "missing dir is a warn, not a fail")
	assert.Contains(t, result.Stdout, "services.directories")
	assert.Contains(t, result.Stdout, "MISSING")
	assert.Contains(t, result.Stdout, "⚠")
}

func Test_Doctor_MalformedHTTPURL_Fails(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "api"), 0o755))

	yaml := `version: 1

services:
  api:
    dir: api
    readiness:
      type: http
      url: "localhost:8080/health"
profiles:
  default: "*"
`

	require.NoError(t, os.WriteFile(filepath.Join(dir, "fuku.yaml"), []byte(yaml), 0o600))

	result := RunOnce(t, dir, "doctor")

	assert.Equal(t, 2, result.ExitCode)
	assert.Contains(t, result.Stdout, "services.readiness")
	assert.Contains(t, result.Stdout, "scheme must be http or https")
}

func Test_Doctor_ExplicitConfig_OverrideSkipped(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "api"), 0o755))

	base := `version: 1

services:
  api:
    dir: api
profiles:
  default: "*"
`

	require.NoError(t, os.WriteFile(filepath.Join(dir, "fuku.yaml"), []byte(base), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "fuku.override.yaml"), []byte("logging:\n  level: debug\n"), 0o600))

	result := RunOnce(t, dir, "--config", "fuku.yaml", "doctor")

	assert.Equal(t, 0, result.ExitCode)
	assert.Contains(t, result.Stdout, "config.override")
	assert.Contains(t, result.Stdout, "override file present but skipped")
	assert.NotContains(t, result.Stdout, "override applied")
}

func Test_Doctor_DefaultLoad_OverrideApplied(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "api"), 0o755))

	base := `version: 1

services:
  api:
    dir: api
profiles:
  default: "*"
`

	require.NoError(t, os.WriteFile(filepath.Join(dir, "fuku.yaml"), []byte(base), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "fuku.override.yaml"), []byte("logging:\n  level: debug\n"), 0o600))

	result := RunOnce(t, dir, "doctor")

	assert.Equal(t, 0, result.ExitCode)
	assert.Contains(t, result.Stdout, "config.override")
	assert.Contains(t, result.Stdout, "override applied")
}

func Test_Doctor_StaleSocketRemediation_NoFixFlag(t *testing.T) {
	result := RunOnce(t, "testdata/yml-config", "doctor")
	assert.NotContains(t, result.Stdout, "doctor --fix")
}

func Test_Doctor_UnknownProfile_Fails(t *testing.T) {
	result := RunOnce(t, "testdata/yml-config", "doctor", "no-such-profile")

	assert.Equal(t, 2, result.ExitCode)
	assert.Contains(t, result.Stdout, "no-such-profile")
}

// doctorCheck is one check of the doctor JSON report
type doctorCheck struct {
	Status  string            `json:"status"`
	Summary string            `json:"summary"`
	Details map[string]string `json:"details"`
}

// doctorChecks parses the checks of a doctor JSON report by ID
func doctorChecks(t *testing.T, stdout string) map[string]doctorCheck {
	t.Helper()

	var report struct {
		Checks map[string]doctorCheck `json:"checks"`
	}

	require.NoError(t, json.Unmarshal([]byte(stdout), &report))

	return report.Checks
}

func Test_Doctor_Dotenv(t *testing.T) {
	tests := []struct {
		name     string
		env      string
		present  []string
		expected doctorCheck
	}{
		{
			name:    "missing file warns",
			env:     "    env:\n      files: [.env, .env.local]\n",
			present: []string{".env"},
			expected: doctorCheck{
				Status:  "warn",
				Summary: "1 of 2 referenced .env files missing",
				Details: map[string]string{"api/.env.local": "MISSING"},
			},
		},
		{
			name:    "every file present",
			env:     "    env:\n      files: [.env, .env.local]\n",
			present: []string{".env", ".env.local"},
			expected: doctorCheck{
				Status:  "ok",
				Summary: "2 files referenced, all readable",
			},
		},
		{
			name: "default files are not checked",
			env:  "",
			expected: doctorCheck{
				Status:  "idle",
				Summary: "no .env files referenced",
			},
		},
		{
			name: "explicit empty list",
			env:  "    env:\n      files: []\n",
			expected: doctorCheck{
				Status:  "idle",
				Summary: "no .env files referenced",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			require.NoError(t, os.MkdirAll(filepath.Join(dir, "api"), 0o755))
			require.NoError(t, os.WriteFile(filepath.Join(dir, "fuku.yaml"), []byte("version: 1\n\nservices:\n  api:\n    dir: api\n"+tt.env), 0o600))

			for _, file := range tt.present {
				require.NoError(t, os.WriteFile(filepath.Join(dir, "api", file), []byte("KEY=value\n"), 0o600))
			}

			result := RunOnce(t, dir, "doctor", "--json")

			assert.Equal(t, 0, result.ExitCode, "a missing .env file is a warn, not a fail")
			assert.Equal(t, tt.expected, doctorChecks(t, result.Stdout)["services.dotenv"])
		})
	}
}

func Test_Doctor_RunningInstance_Notes(t *testing.T) {
	runner := NewRunner(t, "testdata/default-tier")
	defer runner.Stop()

	require.NoError(t, runner.Start("default"))
	require.NoError(t, runner.WaitForRunning(15*time.Second))

	result := RunOnce(t, "testdata/default-tier", "doctor", "--json")

	assert.Equal(t, 0, result.ExitCode)

	instance := doctorChecks(t, result.Stdout)["runtime.instance"]

	assert.Equal(t, "note", instance.Status)
	assert.Equal(t, "another fuku is running for this project", instance.Summary)
	assert.Equal(t, SocketPath(t, "testdata/default-tier"), instance.Details["socket"])
}

func Test_Doctor_BusyReadinessPort_Warns(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:19882")
	require.NoError(t, err)

	defer listener.Close()

	result := RunOnce(t, "testdata/readiness", "doctor", "unhealthy", "--json")

	assert.Equal(t, 0, result.ExitCode, "a busy port is a warn, not a fail")
	assert.Equal(t, doctorCheck{
		Status:  "warn",
		Summary: "1 readiness port(s) already bound",
		Details: map[string]string{"unreachable": "127.0.0.1:19882 already LISTENING"},
	}, doctorChecks(t, result.Stdout)["runtime.ports"])
}
