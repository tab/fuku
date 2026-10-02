package detach

import (
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_NewStderr(t *testing.T) {
	assert.Equal(t, os.Stderr, NewStderr().file)
}

func Test_Stderr_Release(t *testing.T) {
	reader, writer, err := os.Pipe()
	require.NoError(t, err)

	defer reader.Close()
	defer writer.Close()

	stderr := &Stderr{file: writer}

	_, err = stderr.Write([]byte("before\n"))
	require.NoError(t, err)

	require.NoError(t, stderr.Release())

	_, err = stderr.Write([]byte("after\n"))
	require.NoError(t, err)

	read, err := io.ReadAll(reader)

	require.NoError(t, err)
	assert.Equal(t, "before\n", string(read))
}
