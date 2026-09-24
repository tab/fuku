package instance

import (
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fuku/internal/contracts"
	"fuku/internal/model"
)

func Test_NewGuard(t *testing.T) {
	var buf strings.Builder

	identity := model.Instance{Fingerprint: Fingerprint("/Users/dev/projects/shop")}

	g := NewGuard(identity, &buf)

	assert.Equal(t, identity, g.identity)
	assert.Equal(t, &buf, g.stderr)
	assert.Nil(t, g.lock)
}

func Test_Guard_Check(t *testing.T) {
	tests := []struct {
		name         string
		before       func(t *testing.T) (model.Instance, string)
		expected     error
		expectLocked bool
	}{
		{
			name: "takes the free lock of the project",
			before: func(t *testing.T) (model.Instance, string) {
				identity := model.Instance{Fingerprint: Fingerprint(fmt.Sprintf("/Users/dev/projects/%s-%d", t.Name(), time.Now().UnixNano()))}
				t.Cleanup(func() { os.Remove(lockPath(identity.Fingerprint)) })

				return identity, ""
			},
			expectLocked: true,
		},
		{
			name: "refuses while another run holds the lock and names its socket",
			before: func(t *testing.T) (model.Instance, string) {
				identity := model.Instance{Fingerprint: Fingerprint(fmt.Sprintf("/Users/dev/projects/%s-%d", t.Name(), time.Now().UnixNano()))}
				t.Cleanup(func() { os.Remove(lockPath(identity.Fingerprint)) })

				holder := NewGuard(identity, io.Discard)
				require.NoError(t, holder.Check(t.Context()))
				t.Cleanup(func() { holder.lock.Close() })

				socketPath := SocketPath(SocketDir, identity.Fingerprint)
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
				identity := model.Instance{Fingerprint: Fingerprint(fmt.Sprintf("/Users/dev/projects/%s-%d", t.Name(), time.Now().UnixNano()))}
				t.Cleanup(func() { os.Remove(lockPath(identity.Fingerprint)) })

				holder := NewGuard(identity, io.Discard)
				require.NoError(t, holder.Check(t.Context()))
				t.Cleanup(func() { holder.lock.Close() })

				return identity, "Error: fuku is already running for this project (lock " + lockPath(identity.Fingerprint) + ")\nRun 'fuku logs' to follow it or stop that instance before starting another.\n"
			},
			expected: contracts.ErrInstanceAlreadyRunning,
		},
		{
			name: "takes the lock a finished run released",
			before: func(t *testing.T) (model.Instance, string) {
				identity := model.Instance{Fingerprint: Fingerprint(fmt.Sprintf("/Users/dev/projects/%s-%d", t.Name(), time.Now().UnixNano()))}
				t.Cleanup(func() { os.Remove(lockPath(identity.Fingerprint)) })

				holder := NewGuard(identity, io.Discard)
				require.NoError(t, holder.Check(t.Context()))
				require.NoError(t, holder.lock.Close())

				return identity, ""
			},
			expectLocked: true,
		},
		{
			name: "takes the lock beside a stale socket nobody answers on",
			before: func(t *testing.T) (model.Instance, string) {
				identity := model.Instance{Fingerprint: Fingerprint(fmt.Sprintf("/Users/dev/projects/%s-%d", t.Name(), time.Now().UnixNano()))}
				t.Cleanup(func() { os.Remove(lockPath(identity.Fingerprint)) })

				socketPath := SocketPath(SocketDir, identity.Fingerprint)
				fd, err := syscall.Socket(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
				require.NoError(t, err)
				require.NoError(t, syscall.Bind(fd, &syscall.SockaddrUnix{Name: socketPath}))
				require.NoError(t, syscall.Close(fd))
				t.Cleanup(func() { os.Remove(socketPath) })

				return identity, ""
			},
			expectLocked: true,
		},
		{
			name: "fails when the lock file cannot be opened",
			before: func(t *testing.T) (model.Instance, string) {
				identity := model.Instance{Fingerprint: Fingerprint(fmt.Sprintf("/Users/dev/projects/%s-%d", t.Name(), time.Now().UnixNano()))}
				path := lockPath(identity.Fingerprint)
				require.NoError(t, os.Mkdir(path, 0o700))
				t.Cleanup(func() { os.Remove(path) })

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

			err := guard.Check(t.Context())

			require.ErrorIs(t, err, tt.expected)
			assert.Equal(t, tt.expectLocked, guard.lock != nil)
			assert.Equal(t, stderr, buf.String())
		})
	}
}
