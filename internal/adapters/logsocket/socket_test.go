package logsocket

import (
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fuku/internal/adapters/instance"
	"fuku/internal/contracts"
)

func createStaleSocket(t *testing.T, path string) {
	t.Helper()

	fd, err := syscall.Socket(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
	require.NoError(t, err)

	err = syscall.Bind(fd, &syscall.SockaddrUnix{Name: path})
	require.NoError(t, err)

	err = syscall.Close(fd)
	require.NoError(t, err)
}

func Test_findSocket(t *testing.T) {
	//nolint:usetesting // socket path length exceeds macOS limit with t.TempDir
	tmpDir, err := os.MkdirTemp("/tmp", "fuku-test-")
	require.NoError(t, err)

	defer os.RemoveAll(tmpDir)

	socketPath := instance.SocketPath(tmpDir, "0123456789abcdef")

	tests := []struct {
		name     string
		before   func(t *testing.T) func()
		expected string
		wantErr  error
	}{
		{
			name: "live socket",
			before: func(t *testing.T) func() {
				t.Helper()

				listener, err := net.Listen("unix", socketPath)
				require.NoError(t, err)

				return func() { listener.Close() }
			},
			expected: socketPath,
		},
		{
			name: "absent path",
			before: func(*testing.T) func() {
				return func() {}
			},
			wantErr: contracts.ErrNoInstanceRunning,
		},
		{
			name: "regular file",
			before: func(t *testing.T) func() {
				t.Helper()

				require.NoError(t, os.WriteFile(socketPath, []byte("not a socket"), 0600))

				return func() { os.Remove(socketPath) }
			},
			wantErr: contracts.ErrNoInstanceRunning,
		},
		{
			name: "unsearchable directory",
			before: func(t *testing.T) func() {
				t.Helper()

				require.NoError(t, os.Chmod(tmpDir, 0o600))

				return func() { os.Chmod(tmpDir, 0o700) }
			},
			wantErr: fs.ErrPermission,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanup := tt.before(t)
			defer cleanup()

			result, err := findSocket(tmpDir, "0123456789abcdef")

			require.ErrorIs(t, err, tt.wantErr)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_cleanup_NoSockets(t *testing.T) {
	//nolint:usetesting // socket path length exceeds macOS limit with t.TempDir
	tmpDir, err := os.MkdirTemp("/tmp", "fuku-test-")
	require.NoError(t, err)

	defer os.RemoveAll(tmpDir)

	err = cleanup(tmpDir, "own")
	require.NoError(t, err)
}

func Test_cleanup_BadPattern(t *testing.T) {
	err := cleanup("[", "own")

	require.ErrorIs(t, err, filepath.ErrBadPattern)
}

func Test_cleanup_RemovesStaleSocket(t *testing.T) {
	//nolint:usetesting // socket path length exceeds macOS limit with t.TempDir
	tmpDir, err := os.MkdirTemp("/tmp", "fuku-test-")
	require.NoError(t, err)

	defer os.RemoveAll(tmpDir)

	socketPath := instance.SocketPath(tmpDir, "stale")
	createStaleSocket(t, socketPath)

	_, err = os.Stat(socketPath)
	require.NoError(t, err)

	err = cleanup(tmpDir, "own")
	require.NoError(t, err)

	_, err = os.Stat(socketPath)
	assert.True(t, os.IsNotExist(err))
}

func Test_cleanup_SkipsOwnSocket(t *testing.T) {
	//nolint:usetesting // socket path length exceeds macOS limit with t.TempDir
	tmpDir, err := os.MkdirTemp("/tmp", "fuku-test-")
	require.NoError(t, err)

	defer os.RemoveAll(tmpDir)

	socketPath := instance.SocketPath(tmpDir, "own")
	createStaleSocket(t, socketPath)

	err = cleanup(tmpDir, "own")
	require.NoError(t, err)

	_, err = os.Stat(socketPath)
	require.NoError(t, err)
}

func Test_cleanup_SkipsNonSocket(t *testing.T) {
	//nolint:usetesting // socket path length exceeds macOS limit with t.TempDir
	tmpDir, err := os.MkdirTemp("/tmp", "fuku-test-")
	require.NoError(t, err)

	defer os.RemoveAll(tmpDir)

	regularFile := instance.SocketPath(tmpDir, "regular")
	err = os.WriteFile(regularFile, []byte("not a socket"), 0600)
	require.NoError(t, err)

	err = cleanup(tmpDir, "own")
	require.NoError(t, err)

	_, err = os.Stat(regularFile)
	require.NoError(t, err)
}

func Test_cleanup_PreservesActiveSocket(t *testing.T) {
	//nolint:usetesting // socket path length exceeds macOS limit with t.TempDir
	tmpDir, err := os.MkdirTemp("/tmp", "fuku-test-")
	require.NoError(t, err)

	defer os.RemoveAll(tmpDir)

	socketPath := instance.SocketPath(tmpDir, "active")
	listener, err := net.Listen("unix", socketPath)
	require.NoError(t, err)

	defer listener.Close()

	err = cleanup(tmpDir, "own")
	require.NoError(t, err)

	_, err = os.Stat(socketPath)
	require.NoError(t, err)
}

func Test_cleanup_RemoveFailsReturnsError(t *testing.T) {
	//nolint:usetesting // socket path length exceeds macOS limit with t.TempDir
	tmpDir, err := os.MkdirTemp("/tmp", "fuku-test-")
	require.NoError(t, err)

	defer func() {
		os.Chmod(tmpDir, 0700)
		os.RemoveAll(tmpDir)
	}()

	socketPath := instance.SocketPath(tmpDir, "locked")
	createStaleSocket(t, socketPath)

	err = os.Chmod(tmpDir, 0500)
	require.NoError(t, err)

	err = cleanup(tmpDir, "own")
	require.Error(t, err)
	assert.ErrorContains(t, err, "failed to cleanup stale socket")
}

func Test_cleanup_MixedSockets(t *testing.T) {
	//nolint:usetesting // socket path length exceeds macOS limit with t.TempDir
	tmpDir, err := os.MkdirTemp("/tmp", "fuku-test-")
	require.NoError(t, err)

	defer os.RemoveAll(tmpDir)

	activePath := instance.SocketPath(tmpDir, "active")
	activeListener, err := net.Listen("unix", activePath)
	require.NoError(t, err)

	defer activeListener.Close()

	stalePath := instance.SocketPath(tmpDir, "stale")
	createStaleSocket(t, stalePath)

	regularPath := instance.SocketPath(tmpDir, "regular")
	err = os.WriteFile(regularPath, []byte("not a socket"), 0600)
	require.NoError(t, err)

	err = cleanup(tmpDir, "own")
	require.NoError(t, err)

	_, err = os.Stat(activePath)
	require.NoError(t, err)

	_, err = os.Stat(stalePath)
	assert.True(t, os.IsNotExist(err))

	_, err = os.Stat(regularPath)
	require.NoError(t, err)
}
