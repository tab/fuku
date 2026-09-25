package diagnostics

import (
	"net"
	"os"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fuku/internal/adapters/instance"
	"fuku/internal/model"
)

func Test_NewRuntime(t *testing.T) {
	r := NewRuntime()

	assert.NotNil(t, r)
}

func Test_socketAt(t *testing.T) {
	//nolint:usetesting // socket path length exceeds macOS limit with t.TempDir
	dir, err := os.MkdirTemp("/tmp", "fuku-test-")
	require.NoError(t, err)

	t.Cleanup(func() { os.RemoveAll(dir) })

	socketPath := instance.SocketPath(dir, "0123456789abcdef")

	tests := []struct {
		name        string
		before      func(t *testing.T)
		present     bool
		reachable   bool
		expectedErr error
	}{
		{
			name:   "absent socket",
			before: func(*testing.T) {},
		},
		{
			name: "live socket",
			before: func(t *testing.T) {
				listener, err := net.Listen("unix", socketPath)
				require.NoError(t, err)

				t.Cleanup(func() {
					listener.Close()
					os.Remove(socketPath)
				})
			},
			present:   true,
			reachable: true,
		},
		{
			name: "stale socket",
			before: func(t *testing.T) {
				fd, err := syscall.Socket(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
				require.NoError(t, err)
				require.NoError(t, syscall.Bind(fd, &syscall.SockaddrUnix{Name: socketPath}))
				require.NoError(t, syscall.Close(fd))

				t.Cleanup(func() { os.Remove(socketPath) })
			},
			present:     true,
			expectedErr: syscall.ECONNREFUSED,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.before(t)

			got := socketAt(socketPath)

			require.ErrorIs(t, got.Error, tt.expectedErr)
			assert.Equal(t, socketPath, got.Path)
			assert.Equal(t, tt.present, got.Present)
			assert.Equal(t, tt.reachable, got.Reachable)
		})
	}
}

func Test_scanSockets(t *testing.T) {
	//nolint:usetesting // socket path length exceeds macOS limit with t.TempDir
	dir, err := os.MkdirTemp("/tmp", "fuku-test-")
	require.NoError(t, err)

	t.Cleanup(func() { os.RemoveAll(dir) })

	socketPath := instance.SocketPath(dir, "0123456789abcdef")
	regularPath := instance.SocketPath(dir, "0123456789abcdef-regular")

	fd, err := syscall.Socket(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
	require.NoError(t, err)
	require.NoError(t, syscall.Bind(fd, &syscall.SockaddrUnix{Name: socketPath}))
	require.NoError(t, syscall.Close(fd))
	require.NoError(t, os.WriteFile(regularPath, []byte("not a socket"), 0o600))

	scan := scanSockets(dir)

	assert.Equal(t, dir, scan.Dir)
	assert.Equal(t, instance.SocketPath(dir, "*"), scan.Pattern)
	assert.Equal(t, 2, scan.Files)

	found := make(map[string]model.Socket, len(scan.Sockets))
	for _, socket := range scan.Sockets {
		found[socket.Path] = socket
	}

	require.Contains(t, found, socketPath)
	assert.NotContains(t, found, regularPath)
	assert.True(t, found[socketPath].Present)
	assert.False(t, found[socketPath].Reachable)
	require.ErrorIs(t, found[socketPath].Error, syscall.ECONNREFUSED)
}

func Test_Runtime_ProbePort(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	t.Cleanup(func() { listener.Close() })

	busy := listener.Addr().String()

	free, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	freeAddress := free.Addr().String()
	require.NoError(t, free.Close())

	subject := NewRuntime()

	tests := []struct {
		name      string
		readiness model.Readiness
		expected  model.Port
	}{
		{
			name:      "tcp address in use",
			readiness: model.Readiness{Type: model.ReadinessTCP, Address: busy},
			expected:  model.Port{Address: busy, InUse: true},
		},
		{
			name:      "http url in use",
			readiness: model.Readiness{Type: model.ReadinessHTTP, URL: "http://" + busy + "/health"},
			expected:  model.Port{Address: busy, InUse: true},
		},
		{
			name:      "tcp address free",
			readiness: model.Readiness{Type: model.ReadinessTCP, Address: freeAddress},
			expected:  model.Port{Address: freeAddress},
		},
		{
			name:      "log probe has no address",
			readiness: model.Readiness{Type: model.ReadinessLog, Pattern: "ready"},
			expected:  model.Port{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			port := subject.ProbePort(t.Context(), tt.readiness)

			assert.Equal(t, tt.expected, port)
		})
	}
}
