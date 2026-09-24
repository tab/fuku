package process

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fuku/internal/contracts"
)

func Test_prepare(t *testing.T) {
	dir := t.TempDir()

	var stdout, stderr bytes.Buffer

	prepared, err := prepare("echo test", dir, &stdout, &stderr)

	require.NoError(t, err)
	assert.Equal(t, dir, prepared.dir)
	assert.Equal(t, dir, prepared.cmd.Dir)
	assert.Equal(t, []string{"sh", "-c", "echo test"}, prepared.cmd.Args)
	assert.True(t, prepared.cmd.SysProcAttr.Setpgid)
	assert.Same(t, &stdout, prepared.cmd.Stdout)
	assert.Same(t, &stderr, prepared.cmd.Stderr)
	assert.Equal(t, ShutdownTimeout, prepared.cmd.WaitDelay)
}

func Test_prepare_MissingDirectory(t *testing.T) {
	tests := []struct {
		name      string
		directory string
	}{
		{
			name:      "an absolute directory that does not exist",
			directory: "/nonexistent/directory/path",
		},
		{
			name:      "a relative directory resolved from the working directory",
			directory: "nonexistent",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prepared, err := prepare("echo test", tt.directory, io.Discard, io.Discard)

			require.ErrorIs(t, err, contracts.ErrServiceDirectoryNotExist)
			assert.Nil(t, prepared)
		})
	}
}

func Test_resolveDir_Relative(t *testing.T) {
	wd, err := os.Getwd()
	require.NoError(t, err)

	dir, err := resolveDir(".")

	require.NoError(t, err)
	assert.Equal(t, filepath.Clean(wd), dir)
}

func Test_buildCommand(t *testing.T) {
	cmd := buildCommand("go run cmd/main.go")

	assert.Equal(t, []string{"sh", "-c", "go run cmd/main.go"}, cmd.Args)
}
