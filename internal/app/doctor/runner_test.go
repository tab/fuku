package doctor

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_Run_NoConfig(t *testing.T) {
	t.Chdir(t.TempDir())

	report := Run(context.Background(), Options{Profile: "default"})

	require.NotNil(t, report)
	assert.Equal(t, 1, report.SchemaVersion)
	assert.NotEmpty(t, report.FukuVersion)
	assert.NotEmpty(t, report.Platform)
	assert.Equal(t, 2, report.ExitCode())

	configCheck := findCheck(t, report, "config.file")
	assert.Equal(t, StatusFail, configCheck.Status)
}

func Test_Run_WithMinimalConfig(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	serviceDir := filepath.Join(dir, "api")
	require.NoError(t, os.MkdirAll(serviceDir, 0755))

	configContent := `version: 1
services:
  api:
    command: make run
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "fuku.yaml"), []byte(configContent), 0600))

	report := Run(context.Background(), Options{Profile: "default"})

	require.NotNil(t, report)
	assert.Equal(t, 0, report.ExitCode())

	tally := report.Tally()
	assert.Positive(t, tally.OK)
	assert.Equal(t, 0, tally.Fail)
}

func Test_Run_InvalidConfig(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	configContent := `not: valid: yaml: at: all
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "fuku.yaml"), []byte(configContent), 0600))

	report := Run(context.Background(), Options{Profile: "default"})

	require.NotNil(t, report)
	assert.Equal(t, 2, report.ExitCode())

	configCheck := findCheck(t, report, "config.file")
	assert.Equal(t, StatusFail, configCheck.Status)
}

func findCheck(t *testing.T, r *Report, id string) Result {
	t.Helper()

	for _, section := range r.Sections {
		for _, res := range section.Results {
			if res.ID == id {
				return res
			}
		}
	}

	t.Fatalf("check %q not found", id)

	return Result{}
}
