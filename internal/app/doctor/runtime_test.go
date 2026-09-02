package doctor

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fuku/internal/app/instance"
	"fuku/internal/app/relay"
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

	r := checkInstance(t.Context(), env)

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

func Test_instanceResult(t *testing.T) {
	tests := []struct {
		name    string
		local   string
		served  string
		summary string
		detail  string
	}{
		{
			name:    "same project",
			local:   "aaaaaaaaaaaaaaaa",
			served:  "aaaaaaaaaaaaaaaa",
			summary: "another fuku is running for profile 'default'",
		},
		{
			name:    "another project",
			local:   "aaaaaaaaaaaaaaaa",
			served:  "bbbbbbbbbbbbbbbb",
			summary: "profile 'default' socket belongs to another project",
			detail:  "another directory",
		},
		{
			name:    "instance does not report a project",
			local:   "aaaaaaaaaaaaaaaa",
			served:  "",
			summary: "another fuku is running for profile 'default'",
			detail:  "not reported by that instance",
		},
		{
			name:    "local project is unknown",
			local:   "",
			served:  "bbbbbbbbbbbbbbbb",
			summary: "another fuku is running for profile 'default'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := &Env{Profile: "default", Fingerprint: tt.local}
			status := relay.StatusMessage{Profile: "default", Project: tt.served}

			r := instanceResult(env, "/tmp/fuku-default.sock", status)

			assert.Equal(t, "runtime.instance", r.ID)
			assert.Equal(t, StatusNote, r.Status)
			assert.Equal(t, tt.summary, r.Summary)

			if tt.detail == "" {
				assert.Len(t, r.Details, 1)

				return
			}

			assert.Contains(t, r.Details, Detail{Key: "project", Value: tt.detail})
		})
	}
}

func Test_apiResult(t *testing.T) {
	const (
		mine    = "aaaaaaaaaaaaaaaa"
		theirs  = "bbbbbbbbbbbbbbbb"
		unknown = ""
	)

	tests := []struct {
		name     string
		local    string
		found    []instance.Instance
		expected Status
		summary  string
		detail   Detail
		remedy   string
	}{
		{
			name:     "nothing answering",
			local:    mine,
			found:    nil,
			expected: StatusIdle,
			summary:  "no instance answering on 9876-9885",
		},
		{
			name:     "this project is answering",
			local:    mine,
			found:    []instance.Instance{{Address: "127.0.0.1:9877", ID: "abc", Project: mine}},
			expected: StatusOK,
			summary:  "this project's instance is answering on 127.0.0.1:9877",
			detail:   Detail{Key: "bound", Value: "127.0.0.1:9877"},
		},
		{
			name:  "this project answers behind a foreign instance",
			local: mine,
			found: []instance.Instance{
				{Address: "127.0.0.1:9876", ID: "theirs", Project: theirs},
				{Address: "127.0.0.1:9877", ID: "mine", Project: mine},
			},
			expected: StatusOK,
			summary:  "this project's instance is answering on 127.0.0.1:9877",
			detail:   Detail{Key: "configured", Value: "127.0.0.1:9876"},
		},
		{
			name:     "only a foreign instance",
			local:    mine,
			found:    []instance.Instance{{Address: "127.0.0.1:9876", ID: "theirs", Project: theirs}},
			expected: StatusNote,
			summary:  "1 instance(s) on 9876-9885 do not belong to this project",
			detail:   Detail{Key: "127.0.0.1:9876", Value: "serves another project"},
			remedy:   "give this project its own server.listen range",
		},
		{
			name:     "instance did not report a project",
			local:    mine,
			found:    []instance.Instance{{Address: "127.0.0.1:9876", ID: "older"}},
			expected: StatusNote,
			summary:  "1 instance(s) on 9876-9885 do not belong to this project",
			detail:   Detail{Key: "127.0.0.1:9876", Value: "did not report its project"},
			remedy:   "upgrade fuku so the instance reports the project it serves",
		},
		{
			name:  "a foreign instance beside one that did not identify itself",
			local: mine,
			found: []instance.Instance{
				{Address: "127.0.0.1:9876", ID: "theirs", Project: theirs},
				{Address: "127.0.0.1:9877", ID: "older"},
			},
			expected: StatusNote,
			summary:  "2 instance(s) on 9876-9885 do not belong to this project",
			detail:   Detail{Key: "127.0.0.1:9877", Value: "did not report its project"},
			remedy:   "give this project its own server.listen range",
		},
		{
			name:     "local project is unknown",
			local:    unknown,
			found:    []instance.Instance{{Address: "127.0.0.1:9876", ID: "mine", Project: mine}},
			expected: StatusNote,
			summary:  "1 instance(s) on 9876-9885 do not belong to this project",
			detail:   Detail{Key: "127.0.0.1:9876", Value: "serves another project"},
			remedy:   "give this project its own server.listen range",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := apiResult(&Env{Fingerprint: tt.local}, "127.0.0.1:9876", tt.found)

			assert.Equal(t, "runtime.api", r.ID)
			assert.Equal(t, tt.expected, r.Status)
			assert.Equal(t, tt.summary, r.Summary)

			if tt.detail.Key != "" {
				assert.Contains(t, r.Details, tt.detail)
			}

			if tt.remedy == "" {
				assert.Empty(t, r.Remediation)

				return
			}

			assert.Contains(t, r.Remediation, tt.remedy)
		})
	}
}

func Test_checkAPI_Skips(t *testing.T) {
	tests := []struct {
		name    string
		env     *Env
		summary string
	}{
		{name: "config did not load", env: &Env{}, summary: "config did not load"},
		{name: "api not configured", env: &Env{Config: config.DefaultConfig()}, summary: "server.listen is not configured"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := checkAPI(t.Context(), tt.env)

			assert.Equal(t, StatusIdle, r.Status)
			assert.Contains(t, r.Summary, tt.summary)
		})
	}
}

// serveLive answers the liveness probe with a fixed payload and returns the address it bound
func serveLive(t *testing.T, payload string) string {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(payload))
	}))

	t.Cleanup(server.Close)

	return strings.TrimPrefix(server.URL, "http://")
}

// apiEnv builds an Env whose config points the API at the given address
func apiEnv(listen, fingerprint string) *Env {
	cfg := config.DefaultConfig()
	cfg.Server.Listen = listen
	cfg.Server.Auth.Token = "not-a-real-token"

	return &Env{Config: cfg, Fingerprint: fingerprint}
}

func Test_checkAPI_FindsThisProjectsInstance(t *testing.T) {
	fingerprint := instance.Fingerprint("/tmp/project")
	listen := serveLive(t, `{"status":"alive","product":"fuku","instance":"abc","project":"`+fingerprint+`"}`)

	r := checkAPI(t.Context(), apiEnv(listen, fingerprint))

	assert.Equal(t, "runtime.api", r.ID)
	assert.Equal(t, StatusOK, r.Status)
	assert.Contains(t, r.Summary, "this project's instance is answering on "+listen)
	assert.Contains(t, r.Details, Detail{Key: "bound", Value: listen})
	assert.Contains(t, r.Details, Detail{Key: "instance", Value: "abc"})
}

func Test_checkAPI_FindsAnInstanceServingAnotherProject(t *testing.T) {
	listen := serveLive(t, `{"status":"alive","product":"fuku","instance":"abc","project":"bbbbbbbbbbbbbbbb"}`)

	r := checkAPI(t.Context(), apiEnv(listen, instance.Fingerprint("/tmp/project")))

	assert.Equal(t, StatusNote, r.Status)
	assert.Contains(t, r.Summary, "do not belong to this project")
	assert.Contains(t, r.Details, Detail{Key: listen, Value: "serves another project"})
}

func Test_checkAPI_FindsAnInstanceWithoutProjectIdentity(t *testing.T) {
	listen := serveLive(t, `{"status":"alive","product":"fuku","instance":"older"}`)

	r := checkAPI(t.Context(), apiEnv(listen, instance.Fingerprint("/tmp/project")))

	assert.Equal(t, StatusNote, r.Status)
	assert.Contains(t, r.Details, Detail{Key: listen, Value: "did not report its project"})
	assert.Contains(t, r.Remediation, "upgrade fuku")
}

func Test_checkInstance_ReadsTheBannerFromALiveSocket(t *testing.T) {
	tests := []struct {
		name    string
		local   string
		served  string
		summary string
	}{
		{
			name:    "socket serves this project",
			local:   "aaaaaaaaaaaaaaaa",
			served:  "aaaaaaaaaaaaaaaa",
			summary: "another fuku is running for profile",
		},
		{
			name:    "socket serves another project",
			local:   "aaaaaaaaaaaaaaaa",
			served:  "bbbbbbbbbbbbbbbb",
			summary: "belongs to another project",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			profile := fmt.Sprintf("doctor-instance-%d", time.Now().UnixNano())
			socketPath := relay.SocketPathForProfile(config.SocketDir, profile)

			listener, err := net.Listen("unix", socketPath)
			require.NoError(t, err)

			t.Cleanup(func() {
				listener.Close()
				os.Remove(socketPath)
			})

			banner := fmt.Sprintf(`{"type":"status","profile":%q,"project":%q}`, profile, tt.served)
			go answerBanner(listener, banner)

			r := checkInstance(t.Context(), &Env{Profile: profile, Fingerprint: tt.local})

			assert.Equal(t, "runtime.instance", r.ID)
			assert.Equal(t, StatusNote, r.Status)
			assert.Contains(t, r.Summary, tt.summary)
		})
	}
}

func Test_checkInstance_StaleSocketWarns(t *testing.T) {
	profile := fmt.Sprintf("doctor-stale-%d", time.Now().UnixNano())
	socketPath := relay.SocketPathForProfile(config.SocketDir, profile)

	listener, err := net.Listen("unix", socketPath)
	require.NoError(t, err)

	// keep the socket file behind after the listener goes away, which is what a crashed instance leaves
	unix, ok := listener.(*net.UnixListener)
	require.True(t, ok)

	unix.SetUnlinkOnClose(false)
	require.NoError(t, listener.Close())

	t.Cleanup(func() { os.Remove(socketPath) })

	r := checkInstance(t.Context(), &Env{Profile: profile})

	assert.Equal(t, StatusWarn, r.Status)
	assert.Contains(t, r.Summary, "socket present but unreachable")
	assert.Contains(t, r.Remediation, "rm "+socketPath)
}

// answerBanner consumes one subscribe request and replies with the given status banner
func answerBanner(listener net.Listener, banner string) {
	conn, err := listener.Accept()
	if err != nil {
		return
	}

	defer conn.Close()

	if _, err := bufio.NewReader(conn).ReadBytes('\n'); err != nil {
		return
	}

	conn.Write([]byte(banner + "\n"))
}
