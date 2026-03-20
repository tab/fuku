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

func Test_SocketPathForProfile(t *testing.T) {
	tests := []struct {
		name     string
		dir      string
		profile  string
		expected string
	}{
		{
			name:     "default profile",
			dir:      "/tmp",
			profile:  "default",
			expected: "/tmp/fuku-default.sock",
		},
		{
			name:     "custom profile",
			dir:      "/tmp",
			profile:  "backend",
			expected: "/tmp/fuku-backend.sock",
		},
		{
			name:     "custom directory",
			dir:      "/var/run",
			profile:  "core",
			expected: "/var/run/fuku-core.sock",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SocketPathForProfile(tt.dir, tt.profile)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_FindSocket_ByProfile(t *testing.T) {
	//nolint:usetesting // socket path length exceeds macOS limit with t.TempDir
	tmpDir, err := os.MkdirTemp("/tmp", "fuku-test-")
	require.NoError(t, err)

	defer os.RemoveAll(tmpDir)

	socketPath := SocketPathForProfile(tmpDir, "myprofile")
	err = os.WriteFile(socketPath, []byte{}, 0600)
	require.NoError(t, err)

	result, err := FindSocket(tmpDir, "myprofile")
	require.NoError(t, err)
	assert.Equal(t, socketPath, result)
}

func Test_FindSocket_ProfileNotFound(t *testing.T) {
	//nolint:usetesting // socket path length exceeds macOS limit with t.TempDir
	tmpDir, err := os.MkdirTemp("/tmp", "fuku-test-")
	require.NoError(t, err)

	defer os.RemoveAll(tmpDir)

	_, err = FindSocket(tmpDir, "nonexistent")
	require.Error(t, err)
	assert.True(t, errors.Is(err, errors.ErrInstanceNotFound))
}

func Test_FindSocket_NoProfile_SingleSocket(t *testing.T) {
	//nolint:usetesting // socket path length exceeds macOS limit with t.TempDir
	tmpDir, err := os.MkdirTemp("/tmp", "fuku-test-")
	require.NoError(t, err)

	defer os.RemoveAll(tmpDir)

	socketPath := SocketPathForProfile(tmpDir, "default")
	err = os.WriteFile(socketPath, []byte{}, 0600)
	require.NoError(t, err)

	result, err := FindSocket(tmpDir, "")
	require.NoError(t, err)
	assert.Equal(t, socketPath, result)
}

func Test_FindSocket_NoSockets(t *testing.T) {
	//nolint:usetesting // socket path length exceeds macOS limit with t.TempDir
	tmpDir, err := os.MkdirTemp("/tmp", "fuku-test-")
	require.NoError(t, err)

	defer os.RemoveAll(tmpDir)

	_, err = FindSocket(tmpDir, "")
	require.Error(t, err)
	assert.True(t, errors.Is(err, errors.ErrNoInstanceRunning))
}

func Test_FindSocket_MultipleSockets(t *testing.T) {
	//nolint:usetesting // socket path length exceeds macOS limit with t.TempDir
	tmpDir, err := os.MkdirTemp("/tmp", "fuku-test-")
	require.NoError(t, err)

	defer os.RemoveAll(tmpDir)

	err = os.WriteFile(SocketPathForProfile(tmpDir, "core"), []byte{}, 0600)
	require.NoError(t, err)
	err = os.WriteFile(SocketPathForProfile(tmpDir, "backend"), []byte{}, 0600)
	require.NoError(t, err)

	_, err = FindSocket(tmpDir, "")
	require.Error(t, err)
	assert.True(t, errors.Is(err, errors.ErrMultipleInstancesRunning))
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

	socketPath := SocketPathForProfile(tmpDir, "stale")
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

	regularFile := SocketPathForProfile(tmpDir, "regular")
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

	socketPath := SocketPathForProfile(tmpDir, "active")
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

	socketPath := SocketPathForProfile(tmpDir, "locked")
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

	activePath := SocketPathForProfile(tmpDir, "active")
	activeListener, err := net.Listen("unix", activePath)
	require.NoError(t, err)

	defer activeListener.Close()

	stalePath := SocketPathForProfile(tmpDir, "stale")
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
