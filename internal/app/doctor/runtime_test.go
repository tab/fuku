package doctor

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

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

func Test_checkInstance_NoSocket(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	env := &Env{Profile: "test-no-such-instance-" + filepath.Base(dir)}

	r := checkInstance(env)

	assert.Equal(t, StatusIdle, r.Status)
}

func Test_checkStaleSockets(t *testing.T) {
	r := checkStaleSockets()

	assert.Equal(t, "runtime.sockets", r.ID)
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
	assert.Equal(t, "runtime.ports", r.ID)
}
