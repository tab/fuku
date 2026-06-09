package e2e

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fuku doctor exit codes (mirror internal/app/doctor.Report.ExitCode):
//   0 — no fails (warns/notes allowed)
//   2 — at least one fail
//   3 — doctor itself errored (e.g. RenderJSON write failure)

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

	// parses as YAML but fails schema validation (unknown readiness type)
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
	// the file parsed fine, so config.file must not claim a load failure
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
	// regression: doctor must not advertise a `--fix` flag that doesn't exist
	result := RunOnce(t, "testdata/yml-config", "doctor")
	assert.NotContains(t, result.Stdout, "doctor --fix")
}

func Test_Doctor_UnknownProfile_Fails(t *testing.T) {
	result := RunOnce(t, "testdata/yml-config", "doctor", "no-such-profile")

	assert.Equal(t, 2, result.ExitCode)
	assert.Contains(t, result.Stdout, "no-such-profile")
}
