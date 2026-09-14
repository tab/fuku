package doctor

import (
	"context"
	"fmt"
	"net"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fuku/internal/app/instance"
	"fuku/internal/config"
)

func Test_extractAddress(t *testing.T) {
	tests := []struct {
		name     string
		probe    *config.Readiness
		expected string
	}{
		{
			name:     "http with explicit port",
			probe:    &config.Readiness{Type: config.TypeHTTP, URL: "http://localhost:8080/health"},
			expected: "localhost:8080",
		},
		{
			name:     "https default port",
			probe:    &config.Readiness{Type: config.TypeHTTP, URL: "https://example.com/health"},
			expected: "example.com:443",
		},
		{
			name:     "http default port",
			probe:    &config.Readiness{Type: config.TypeHTTP, URL: "http://example.com/health"},
			expected: "example.com:80",
		},
		{
			name:     "tcp address passthrough",
			probe:    &config.Readiness{Type: config.TypeTCP, Address: "localhost:5432"},
			expected: "localhost:5432",
		},
		{
			name:     "log type returns empty",
			probe:    &config.Readiness{Type: config.TypeLog, Pattern: "ready"},
			expected: "",
		},
		{
			name:     "malformed url returns empty",
			probe:    &config.Readiness{Type: config.TypeHTTP, URL: "://broken"},
			expected: "",
		},
		{
			name:     "non-http scheme returns empty",
			probe:    &config.Readiness{Type: config.TypeHTTP, URL: "ftp://host/health"},
			expected: "",
		},
		{
			name:     "scheme-less host returns empty",
			probe:    &config.Readiness{Type: config.TypeHTTP, URL: "localhost:8080/health"},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, extractAddress(tt.probe))
		})
	}
}

func Test_checkInstance(t *testing.T) {
	tests := []struct {
		name    string
		before  func(t *testing.T, socketPath string) func()
		status  Status
		summary string
	}{
		{
			name: "absent socket",
			before: func(*testing.T, string) func() {
				return func() {}
			},
			status:  StatusIdle,
			summary: "no other fuku running for this project",
		},
		{
			name: "live socket",
			before: func(t *testing.T, socketPath string) func() {
				t.Helper()

				listener, err := net.Listen("unix", socketPath)
				require.NoError(t, err)

				return func() {
					listener.Close()
					os.Remove(socketPath)
				}
			},
			status:  StatusNote,
			summary: "another fuku is running for this project",
		},
		{
			name: "stale socket",
			before: func(t *testing.T, socketPath string) func() {
				t.Helper()

				fd, err := syscall.Socket(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
				require.NoError(t, err)
				require.NoError(t, syscall.Bind(fd, &syscall.SockaddrUnix{Name: socketPath}))
				require.NoError(t, syscall.Close(fd))

				return func() { os.Remove(socketPath) }
			},
			status:  StatusWarn,
			summary: "socket present but unreachable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fingerprint := instance.Fingerprint(fmt.Sprintf("/Users/dev/projects/%s-%d", t.Name(), time.Now().UnixNano()))
			socketPath := instance.SocketPath(config.SocketDir, fingerprint)

			cleanup := tt.before(t, socketPath)
			defer cleanup()

			r := checkInstance(&Env{Profile: config.Default, Fingerprint: fingerprint})

			assert.Equal(t, CheckRuntimeInstance, r.ID)
			assert.Equal(t, tt.status, r.Status)
			assert.Equal(t, tt.summary, r.Summary)
		})
	}
}

func Test_checkStaleSockets(t *testing.T) {
	r := checkStaleSockets()

	assert.Equal(t, CheckRuntimeSockets, r.ID)
	assert.Contains(t, []Status{StatusOK, StatusWarn}, r.Status)
}

func Test_checkPorts_NoConfig(t *testing.T) {
	r := checkPorts(context.Background(), &Env{})

	assert.Equal(t, StatusIdle, r.Status)
}

func Test_checkPorts_NoReadiness(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Services["api"] = &config.Service{Dir: "api"}

	env := &Env{Config: cfg, Topology: config.DefaultTopology(), Profile: config.Default, ProfileServices: []string{"api"}}

	r := checkPorts(context.Background(), env)

	assert.Equal(t, StatusIdle, r.Status)
}

func Test_checkPorts_ProfileError(t *testing.T) {
	env := &Env{Config: config.DefaultConfig(), Topology: config.DefaultTopology(), ProfileErr: assert.AnError}

	r := checkPorts(context.Background(), env)

	assert.Equal(t, StatusIdle, r.Status)
	assert.Equal(t, CheckRuntimePorts, r.ID)
}
