package main

import (
	"context"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const mainTestEnv = "FUKU_TEST_MAIN"

func Test_main(t *testing.T) {
	if os.Getenv(mainTestEnv) == "1" {
		os.Args = []string{os.Args[0], "version"}

		main()

		return
	}

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^Test_main$")
	cmd.Env = append(cmd.Environ(), mainTestEnv+"=1")

	output, err := cmd.Output()

	require.NoError(t, err)
	assert.Contains(t, string(output), "Version:")
}
