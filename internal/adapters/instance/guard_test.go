package instance

import (
	"io"
	"net"
	"os"
	"runtime"
	"strings"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fuku/internal/contracts"
	"fuku/internal/model"
)

func Test_Guard_acquire(t *testing.T) {
	//nolint:usetesting // socket path length exceeds macOS limit with t.TempDir
	dir, err := os.MkdirTemp("/tmp", "fuku-test-")
	require.NoError(t, err)

	t.Cleanup(func() { os.RemoveAll(dir) })

	tests := []struct {
		name         string
		before       func(t *testing.T) (model.Instance, string)
		expected     error
		expectLocked bool
	}{
		{
			name: "takes the free lock of the project",
			before: func(t *testing.T) (model.Instance, string) {
				identity := model.Instance{Fingerprint: Fingerprint("/Users/dev/projects/" + t.Name())}

				return identity, ""
			},
			expectLocked: true,
		},
		{
			name: "refuses while another run holds the lock and names its socket",
			before: func(t *testing.T) (model.Instance, string) {
				identity := model.Instance{Fingerprint: Fingerprint("/Users/dev/projects/" + t.Name())}

				holder := NewGuard(identity, io.Discard)
				require.NoError(t, holder.acquire(dir))
				t.Cleanup(func() { holder.lock.Close() })

				socketPath := SocketPath(dir, identity.Fingerprint)
				listener, err := net.Listen("unix", socketPath)
				require.NoError(t, err)
				t.Cleanup(func() { listener.Close() })

				return identity, "Error: fuku is already running for this project (socket " + socketPath + ")\nRun 'fuku logs' to follow it or stop that instance before starting another.\n"
			},
			expected: contracts.ErrInstanceAlreadyRunning,
		},
		{
			name: "names the lock file while the other run's socket is not up yet",
			before: func(t *testing.T) (model.Instance, string) {
				identity := model.Instance{Fingerprint: Fingerprint("/Users/dev/projects/" + t.Name())}

				holder := NewGuard(identity, io.Discard)
				require.NoError(t, holder.acquire(dir))
				t.Cleanup(func() { holder.lock.Close() })

				return identity, "Error: fuku is already running for this project (lock " + lockPath(dir, identity.Fingerprint) + ")\nRun 'fuku logs' to follow it or stop that instance before starting another.\n"
			},
			expected: contracts.ErrInstanceAlreadyRunning,
		},
		{
			name: "takes the lock a finished run released",
			before: func(t *testing.T) (model.Instance, string) {
				identity := model.Instance{Fingerprint: Fingerprint("/Users/dev/projects/" + t.Name())}

				holder := NewGuard(identity, io.Discard)
				require.NoError(t, holder.acquire(dir))
				require.NoError(t, holder.lock.Close())

				return identity, ""
			},
			expectLocked: true,
		},
		{
			name: "takes the lock beside a stale socket nobody answers on",
			before: func(t *testing.T) (model.Instance, string) {
				identity := model.Instance{Fingerprint: Fingerprint("/Users/dev/projects/" + t.Name())}

				socketPath := SocketPath(dir, identity.Fingerprint)
				fd, err := syscall.Socket(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
				require.NoError(t, err)
				require.NoError(t, syscall.Bind(fd, &syscall.SockaddrUnix{Name: socketPath}))
				require.NoError(t, syscall.Close(fd))

				return identity, ""
			},
			expectLocked: true,
		},
		{
			name: "fails when the lock file cannot be opened",
			before: func(t *testing.T) (model.Instance, string) {
				identity := model.Instance{Fingerprint: Fingerprint("/Users/dev/projects/" + t.Name())}
				path := lockPath(dir, identity.Fingerprint)
				require.NoError(t, os.Mkdir(path, 0o700))

				return identity, ""
			},
			expected: ErrFailedToLockProject,
		},
		{
			name: "fails when the lock file refuses a lock",
			before: func(t *testing.T) (model.Instance, string) {
				if runtime.GOOS != "darwin" {
					t.Skip("only darwin refuses to flock a FIFO")
				}

				identity := model.Instance{Fingerprint: Fingerprint("/Users/dev/projects/" + t.Name())}
				require.NoError(t, syscall.Mkfifo(lockPath(dir, identity.Fingerprint), 0o600))

				return identity, ""
			},
			expected: ErrFailedToLockProject,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf strings.Builder

			identity, stderr := tt.before(t)
			guard := NewGuard(identity, &buf)

			t.Cleanup(func() { guard.lock.Close() })

			err := guard.acquire(dir)

			require.ErrorIs(t, err, tt.expected)
			assert.Equal(t, tt.expectLocked, guard.lock != nil)
			assert.Equal(t, stderr, buf.String())
		})
	}
}
