package doctor

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fuku/internal/config"
)

func Test_checkServiceDirectories(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	require.NoError(t, os.MkdirAll(filepath.Join(dir, "api"), 0755))

	cfg := config.DefaultConfig()
	cfg.Services["api"] = &config.Service{Dir: "api"}
	cfg.Services["missing"] = &config.Service{Dir: "missing"}

	env := &Env{Config: cfg}

	tests := []struct {
		name     string
		names    []string
		expected Status
	}{
		{
			name:     "all present",
			names:    []string{"api"},
			expected: StatusOK,
		},
		{
			name:     "some missing",
			names:    []string{"api", "missing"},
			expected: StatusWarn,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := checkServiceDirectories(env, tt.names)
			assert.Equal(t, tt.expected, r.Status)
		})
	}
}

func Test_checkServiceDotenv(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	apiDir := filepath.Join(dir, "api")
	require.NoError(t, os.MkdirAll(apiDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(apiDir, ".env"), []byte(""), 0600))

	cfg := config.DefaultConfig()
	cfg.Services["api"] = &config.Service{Dir: "api", Env: &config.Env{Files: []string{".env"}}}
	cfg.Services["missing"] = &config.Service{Dir: "api", Env: &config.Env{Files: []string{".env.local"}}}
	cfg.Services["noenv"] = &config.Service{Dir: "api"}

	env := &Env{Config: cfg}

	tests := []struct {
		name     string
		names    []string
		expected Status
	}{
		{
			name:     "no env files referenced",
			names:    []string{"noenv"},
			expected: StatusIdle,
		},
		{
			name:     "all present",
			names:    []string{"api"},
			expected: StatusOK,
		},
		{
			name:     "some missing",
			names:    []string{"api", "missing"},
			expected: StatusWarn,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := checkServiceDotenv(env, tt.names)
			assert.Equal(t, tt.expected, r.Status)
		})
	}
}

func Test_checkServiceReadiness(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Services["http"] = &config.Service{
		Readiness: &config.Readiness{Type: config.TypeHTTP, URL: "http://localhost:8080/health"},
	}
	cfg.Services["tcp"] = &config.Service{
		Readiness: &config.Readiness{Type: config.TypeTCP, Address: "localhost:5432"},
	}
	cfg.Services["log"] = &config.Service{
		Readiness: &config.Readiness{Type: config.TypeLog, Pattern: `ready\s+\d+`},
	}
	cfg.Services["badregex"] = &config.Service{
		Readiness: &config.Readiness{Type: config.TypeLog, Pattern: `[invalid`},
	}
	cfg.Services["badaddr"] = &config.Service{
		Readiness: &config.Readiness{Type: config.TypeTCP, Address: "no-port"},
	}
	cfg.Services["urlnoscheme"] = &config.Service{
		Readiness: &config.Readiness{Type: config.TypeHTTP, URL: "localhost:8080/health"},
	}
	cfg.Services["urlnohost"] = &config.Service{
		Readiness: &config.Readiness{Type: config.TypeHTTP, URL: "http:///health"},
	}
	cfg.Services["urlwrongscheme"] = &config.Service{
		Readiness: &config.Readiness{Type: config.TypeHTTP, URL: "ftp://host/health"},
	}
	cfg.Services["none"] = &config.Service{}

	env := &Env{Config: cfg}

	tests := []struct {
		name     string
		names    []string
		expected Status
	}{
		{
			name:     "no probes",
			names:    []string{"none"},
			expected: StatusIdle,
		},
		{
			name:     "all probes parse",
			names:    []string{"http", "tcp", "log"},
			expected: StatusOK,
		},
		{
			name:     "bad regex",
			names:    []string{"badregex"},
			expected: StatusFail,
		},
		{
			name:     "bad address",
			names:    []string{"badaddr"},
			expected: StatusFail,
		},
		{
			name:     "http url without scheme is rejected",
			names:    []string{"urlnoscheme"},
			expected: StatusFail,
		},
		{
			name:     "http url with empty host is rejected",
			names:    []string{"urlnohost"},
			expected: StatusFail,
		},
		{
			name:     "http url with non-http scheme is rejected",
			names:    []string{"urlwrongscheme"},
			expected: StatusFail,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := checkServiceReadiness(env, tt.names)
			assert.Equal(t, tt.expected, r.Status)
		})
	}
}

func Test_validateHTTPURL(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "valid http", input: "http://localhost:8080/health", wantErr: false},
		{name: "valid https", input: "https://example.com/health", wantErr: false},
		{name: "missing scheme", input: "localhost:8080/health", wantErr: true},
		{name: "empty host", input: "http:///health", wantErr: true},
		{name: "wrong scheme", input: "ftp://host/health", wantErr: true},
		{name: "parse error", input: "://broken", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateHTTPURL(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
		})
	}
}

func Test_servicesSection(t *testing.T) {
	happyCfg := config.DefaultConfig()
	happyCfg.Services["api"] = &config.Service{Dir: "api"}

	tests := []struct {
		name        string
		env         *Env
		wantNote    string
		wantStatus  Status
		wantResults int
	}{
		{
			name:        "config did not load",
			env:         &Env{},
			wantNote:    "skipped (config did not load)",
			wantStatus:  StatusIdle,
			wantResults: 3,
		},
		{
			name:        "profile did not resolve",
			env:         &Env{Config: config.DefaultConfig(), ProfileErr: assert.AnError},
			wantNote:    "skipped (profile did not resolve)",
			wantStatus:  StatusIdle,
			wantResults: 3,
		},
		{
			name:        "resolves to services",
			env:         &Env{Config: happyCfg, Profile: config.Default, ProfileServices: []string{"api"}},
			wantNote:    "active profile: default · 1 services",
			wantResults: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			section := servicesSection(context.Background(), tt.env)

			assert.Equal(t, "Services", section.Title)
			assert.Equal(t, tt.wantNote, section.Note)
			require.Len(t, section.Results, tt.wantResults)

			if tt.wantStatus != StatusOK {
				for _, r := range section.Results {
					assert.Equal(t, tt.wantStatus, r.Status)
				}
			}
		})
	}
}
