package relay

import (
	"net"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fuku/internal/app/errors"
	"fuku/internal/app/instance"
	"fuku/internal/config"
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

func Test_FindSocket(t *testing.T) {
	tests := []struct {
		name    string
		before  func(t *testing.T, socketPath string) func()
		wantErr error
	}{
		{
			name: "live socket",
			before: func(t *testing.T, socketPath string) func() {
				t.Helper()

				listener, err := net.Listen("unix", socketPath)
				require.NoError(t, err)

				return func() { listener.Close() }
			},
		},
		{
			name: "absent path",
			before: func(*testing.T, string) func() {
				return func() {}
			},
			wantErr: errors.ErrNoInstanceRunning,
		},
		{
			name: "regular file",
			before: func(t *testing.T, socketPath string) func() {
				t.Helper()

				require.NoError(t, os.WriteFile(socketPath, []byte("not a socket"), 0600))

				return func() {}
			},
			wantErr: errors.ErrNoInstanceRunning,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			//nolint:usetesting // socket path length exceeds macOS limit with t.TempDir
			tmpDir, err := os.MkdirTemp("/tmp", "fuku-test-")
			require.NoError(t, err)

			defer os.RemoveAll(tmpDir)

			socketPath := instance.SocketPath(tmpDir, "0123456789abcdef")

			cleanup := tt.before(t, socketPath)
			defer cleanup()

			result, err := FindSocket(tmpDir, "0123456789abcdef")

			if tt.wantErr != nil {
				require.Error(t, err)
				assert.True(t, errors.Is(err, tt.wantErr))
				assert.Empty(t, result)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, socketPath, result)
		})
	}
}

func Test_Cleanup_NoSockets(t *testing.T) {
	//nolint:usetesting // socket path length exceeds macOS limit with t.TempDir
	tmpDir, err := os.MkdirTemp("/tmp", "fuku-test-")
	require.NoError(t, err)

	defer os.RemoveAll(tmpDir)

	err = Cleanup(tmpDir)
	require.NoError(t, err)
}

func Test_Cleanup_RemovesStaleSocket(t *testing.T) {
	//nolint:usetesting // socket path length exceeds macOS limit with t.TempDir
	tmpDir, err := os.MkdirTemp("/tmp", "fuku-test-")
	require.NoError(t, err)

	defer os.RemoveAll(tmpDir)

	socketPath := instance.SocketPath(tmpDir, "stale")
	createStaleSocket(t, socketPath)

	_, err = os.Stat(socketPath)
	require.NoError(t, err)

	err = Cleanup(tmpDir)
	require.NoError(t, err)

	_, err = os.Stat(socketPath)
	assert.True(t, os.IsNotExist(err))
}

func Test_Cleanup_SkipsNonSocket(t *testing.T) {
	//nolint:usetesting // socket path length exceeds macOS limit with t.TempDir
	tmpDir, err := os.MkdirTemp("/tmp", "fuku-test-")
	require.NoError(t, err)

	defer os.RemoveAll(tmpDir)

	regularFile := instance.SocketPath(tmpDir, "regular")
	err = os.WriteFile(regularFile, []byte("not a socket"), 0600)
	require.NoError(t, err)

	err = Cleanup(tmpDir)
	require.NoError(t, err)

	_, err = os.Stat(regularFile)
	require.NoError(t, err)
}

func Test_Cleanup_PreservesActiveSocket(t *testing.T) {
	//nolint:usetesting // socket path length exceeds macOS limit with t.TempDir
	tmpDir, err := os.MkdirTemp("/tmp", "fuku-test-")
	require.NoError(t, err)

	defer os.RemoveAll(tmpDir)

	socketPath := instance.SocketPath(tmpDir, "active")
	listener, err := net.Listen("unix", socketPath)
	require.NoError(t, err)

	defer listener.Close()

	err = Cleanup(tmpDir)
	require.NoError(t, err)

	_, err = os.Stat(socketPath)
	require.NoError(t, err)
}

func Test_Cleanup_RemoveFailsReturnsError(t *testing.T) {
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

	err = Cleanup(tmpDir)
	require.Error(t, err)
	assert.True(t, errors.Is(err, errors.ErrFailedToCleanupSocket))
}

func Test_Cleanup_MixedSockets(t *testing.T) {
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

	regularPath := filepath.Join(tmpDir, config.SocketPrefix+"regular"+config.SocketSuffix)
	err = os.WriteFile(regularPath, []byte("not a socket"), 0600)
	require.NoError(t, err)

	err = Cleanup(tmpDir)
	require.NoError(t, err)

	_, err = os.Stat(activePath)
	require.NoError(t, err)

	_, err = os.Stat(stalePath)
	assert.True(t, os.IsNotExist(err))

	_, err = os.Stat(regularPath)
	require.NoError(t, err)
}
