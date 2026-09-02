package e2e

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
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

func Test_Doctor_APINotConfigured(t *testing.T) {
	result := RunOnce(t, "testdata/yml-config", "doctor")

	assert.Equal(t, 0, result.ExitCode, "an unconfigured API is a choice, not a failure")
	assert.Contains(t, result.Stdout, "config.api")
	assert.Contains(t, result.Stdout, "not configured")
	assert.Contains(t, result.Stdout, "fuku.override.yaml")
	assert.Contains(t, result.Stdout, "runtime.api")
	assert.Contains(t, result.Stdout, "server.listen is not configured")
}

func Test_Doctor_APIConfigured(t *testing.T) {
	result := RunOnce(t, "testdata/api", "doctor")

	assert.Equal(t, 0, result.ExitCode)
	assert.Contains(t, result.Stdout, "config.api")
	assert.Contains(t, result.Stdout, "enabled on 127.0.0.1:19876")
	assert.Contains(t, result.Stdout, "19876-19885")
	assert.Contains(t, result.Stdout, "no instance answering")
}

func Test_Doctor_NeverPrintsTheAPIToken(t *testing.T) {
	for _, format := range []string{"", "--json", "--summary"} {
		t.Run("format "+format, func(t *testing.T) {
			args := []string{"doctor"}
			if format != "" {
				args = append(args, format)
			}

			result := RunOnce(t, "testdata/api", args...)

			assert.NotContains(t, result.Stdout, apiToken)
			assert.NotContains(t, result.Stderr, apiToken)
		})
	}
}

func Test_Doctor_ReportsThisProjectsRunningInstance(t *testing.T) {
	runner := startAPIRunner(t)
	defer runner.Stop()

	result := RunOnce(t, "testdata/api", "doctor")

	assert.Equal(t, 0, result.ExitCode)
	assert.Contains(t, result.Stdout, "this project's instance is answering on 127.0.0.1:19876")
	assert.Contains(t, result.Stdout, "another fuku is running for profile 'default'")
	assert.NotContains(t, result.Stdout, "belongs to another project")
	assert.NotContains(t, result.Stdout, apiToken)
}

func Test_Doctor_AttributesAnotherProjectsSocket(t *testing.T) {
	runner := startAPIRunner(t)
	defer runner.Stop()

	// yml-config uses the same profile name, so it resolves to the socket testdata/api holds
	result := RunOnce(t, "testdata/yml-config", "doctor")

	assert.Equal(t, 0, result.ExitCode)
	assert.Contains(t, result.Stdout, "profile 'default' socket belongs to another project")
	assert.NotContains(t, result.Stdout, "another fuku is running for profile 'default'")
}

func Test_Doctor_JSON_CarriesTheAPIChecks(t *testing.T) {
	result := RunOnce(t, "testdata/yml-config", "doctor", "--json")

	require.Equal(t, 0, result.ExitCode)

	var report map[string]any
	require.NoError(t, json.Unmarshal([]byte(result.Stdout), &report))

	checks, ok := report["checks"].(map[string]any)
	require.True(t, ok, "checks must be an object")

	api, ok := checks["config.api"].(map[string]any)
	require.True(t, ok, "config.api must be reported so an agent can see the API is missing")
	assert.Equal(t, "idle", api["status"])
	assert.NotEmpty(t, api["remediation"], "an agent needs the remediation to tell the user what to enable")

	runtimeAPI, ok := checks["runtime.api"].(map[string]any)
	require.True(t, ok, "runtime.api must be reported")
	assert.Equal(t, "idle", runtimeAPI["status"])
}

func Test_Doctor_JSON_ReportsTheBoundAddress(t *testing.T) {
	runner := startAPIRunner(t)
	defer runner.Stop()

	result := RunOnce(t, "testdata/api", "doctor", "--json")

	require.Equal(t, 0, result.ExitCode)
	assert.NotContains(t, result.Stdout, apiToken)

	var report map[string]any
	require.NoError(t, json.Unmarshal([]byte(result.Stdout), &report))

	checks, ok := report["checks"].(map[string]any)
	require.True(t, ok)

	runtimeAPI, ok := checks["runtime.api"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "ok", runtimeAPI["status"])

	details, ok := runtimeAPI["details"].(map[string]any)
	require.True(t, ok, "the bound address is what an agent needs to reach the instance")
	assert.Equal(t, "127.0.0.1:19876", details["bound"])
	assert.Equal(t, "127.0.0.1:19876", details["configured"])
	assert.NotEmpty(t, details["instance"])
}

func Test_Doctor_ReportsAForeignInstanceInThePortRange(t *testing.T) {
	runner := startAPIRunner(t)
	defer runner.Stop()

	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "api"), 0o755))

	// a different directory pointed at the port range testdata/api already occupies
	yaml := `version: 1

services:
  api:
    dir: api
profiles:
  default: "*"
server:
  listen: "127.0.0.1:19876"
  auth:
    token: "another-projects-token"
`

	require.NoError(t, os.WriteFile(filepath.Join(dir, "fuku.yaml"), []byte(yaml), 0o600))

	result := RunOnce(t, dir, "doctor")

	assert.Equal(t, 0, result.ExitCode)
	assert.Contains(t, result.Stdout, "do not belong to this project")
	assert.Contains(t, result.Stdout, "serves another project")
	assert.Contains(t, result.Stdout, "127.0.0.1:19876")
	assert.NotContains(t, result.Stdout, "this project's instance is answering")
}

// serveFakeInstance answers the liveness probe with a fixed payload and returns the address it bound
func serveFakeInstance(t *testing.T, payload string) string {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(payload))
	}))

	t.Cleanup(server.Close)

	return strings.TrimPrefix(server.URL, "http://")
}

// writeAPIConfig writes a config in a fresh directory pointing the API at the given address
func writeAPIConfig(t *testing.T, listen string) string {
	t.Helper()

	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "api"), 0o755))

	yaml := fmt.Sprintf(`version: 1

services:
  api:
    dir: api
profiles:
  default: "*"
server:
  listen: %q
  auth:
    token: "not-a-real-token"
`, listen)

	require.NoError(t, os.WriteFile(filepath.Join(dir, "fuku.yaml"), []byte(yaml), 0o600))

	return dir
}

func Test_Doctor_ReportsAnInstanceThatDoesNotIdentifyItsProject(t *testing.T) {
	listen := serveFakeInstance(t, `{"status":"alive","product":"fuku","instance":"older"}`)

	result := RunOnce(t, writeAPIConfig(t, listen), "doctor")

	assert.Equal(t, 0, result.ExitCode)
	assert.Contains(t, result.Stdout, "do not belong to this project")
	assert.Contains(t, result.Stdout, "did not report its project")
	assert.Contains(t, result.Stdout, "upgrade fuku")
	assert.NotContains(t, result.Stdout, "this project's instance is answering")
}

func Test_Doctor_IgnoresANonFukuServerInThePortRange(t *testing.T) {
	listen := serveFakeInstance(t, `{"status":"alive","product":"something-else"}`)

	result := RunOnce(t, writeAPIConfig(t, listen), "doctor")

	assert.Equal(t, 0, result.ExitCode)
	assert.Contains(t, result.Stdout, "no instance answering")
	assert.NotContains(t, result.Stdout, "do not belong to this project")
}
