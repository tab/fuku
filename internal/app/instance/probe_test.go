package instance

import (
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fuku/internal/app/errors"
	"fuku/internal/config"
)

// listenAddress returns the host:port a test server is bound to
func listenAddress(t *testing.T, server *httptest.Server) string {
	t.Helper()

	return strings.TrimPrefix(server.URL, "http://")
}

// portBefore returns the address one port below the given one, so the probe has to walk the range
func portBefore(t *testing.T, address string) string {
	t.Helper()

	host, portText, err := net.SplitHostPort(address)
	require.NoError(t, err)

	port, err := strconv.Atoi(portText)
	require.NoError(t, err)

	return net.JoinHostPort(host, strconv.Itoa(port-1))
}

func Test_Running(t *testing.T) {
	fingerprint := Fingerprint("/tmp/project")

	tests := []struct {
		name    string
		payload string
		expect  bool
	}{
		{
			name:    "instance serving this project",
			payload: `{"status":"alive","product":"fuku","instance":"id","project":"` + fingerprint + `"}`,
			expect:  true,
		},
		{
			name:    "instance serving another project",
			payload: `{"status":"alive","product":"fuku","instance":"id","project":"0123456789abcdef"}`,
			expect:  false,
		},
		{
			name:    "another product on the port",
			payload: `{"status":"alive","product":"other","instance":"id","project":"` + fingerprint + `"}`,
			expect:  false,
		},
		{
			name:    "older instance without a project",
			payload: `{"status":"alive"}`,
			expect:  false,
		},
		{
			name:    "response is not JSON",
			payload: "alive",
			expect:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, livePath, r.URL.Path)
				w.Write([]byte(tt.payload))
			}))
			defer server.Close()

			address, found := Running(t.Context(), listenAddress(t, server), fingerprint)

			assert.Equal(t, tt.expect, found)

			if !tt.expect {
				assert.Empty(t, address)

				return
			}

			assert.Equal(t, listenAddress(t, server), address)
		})
	}
}

func Test_Running_WalksThePortRange(t *testing.T) {
	fingerprint := Fingerprint("/tmp/project")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`{"status":"alive","product":"fuku","instance":"id","project":"` + fingerprint + `"}`))
	}))
	defer server.Close()

	address, found := Running(t.Context(), portBefore(t, listenAddress(t, server)), fingerprint)

	require.True(t, found)
	assert.Equal(t, listenAddress(t, server), address)
}

func Test_Running_RejectsUnhealthyInstance(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	_, found := Running(t.Context(), listenAddress(t, server), Fingerprint("/tmp/project"))

	assert.False(t, found)
}

func Test_Running_NoInstance(t *testing.T) {
	tests := []struct {
		name   string
		listen string
	}{
		{name: "listen is empty", listen: ""},
		{name: "listen has no port", listen: "127.0.0.1"},
		{name: "port is not a number", listen: "127.0.0.1:http"},
		{name: "nothing is bound", listen: "127.0.0.1:1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			address, found := Running(t.Context(), tt.listen, Fingerprint("/tmp/project"))

			assert.False(t, found)
			assert.Empty(t, address)
		})
	}
}

func Test_SplitListen(t *testing.T) {
	tests := []struct {
		name   string
		listen string
		host   string
		port   int
		err    bool
	}{
		{name: "loopback address", listen: "127.0.0.1:1234", host: "127.0.0.1", port: 1234},
		{name: "any host", listen: ":9876", host: "", port: 9876},
		{name: "missing port", listen: "127.0.0.1", err: true},
		{name: "named port", listen: "127.0.0.1:http", err: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host, port, err := SplitListen(tt.listen)

			if tt.err {
				require.Error(t, err)
				assert.True(t, errors.Is(err, errors.ErrAPIInvalidListen))

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.host, host)
			assert.Equal(t, tt.port, port)
		})
	}
}

func Test_Running_StopsAfterThePortRange(t *testing.T) {
	probed := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		probed++

		w.Write([]byte(`{"status":"alive","product":"fuku","project":"0123456789abcdef"}`))
	}))
	defer server.Close()

	_, found := Running(t.Context(), listenAddress(t, server), Fingerprint("/tmp/project"))

	assert.False(t, found)
	assert.LessOrEqual(t, probed, config.APIPortRetries)
}

// serveConsecutive starts one server per payload on consecutive ports and returns the first address
func serveConsecutive(t *testing.T, payloads ...string) string {
	t.Helper()

	for range 20 {
		probe, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)

		_, portText, err := net.SplitHostPort(probe.Addr().String())
		require.NoError(t, err)
		require.NoError(t, probe.Close())

		port, err := strconv.Atoi(portText)
		require.NoError(t, err)

		listeners := make([]net.Listener, 0, len(payloads))

		for i := range payloads {
			listener, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port+i)))
			if err != nil {
				break
			}

			listeners = append(listeners, listener)
		}

		if len(listeners) != len(payloads) {
			for _, listener := range listeners {
				listener.Close()
			}

			continue
		}

		for i, listener := range listeners {
			server := httptest.NewUnstartedServer(respond(payloads[i]))
			require.NoError(t, server.Listener.Close())

			server.Listener = listener
			server.Start()

			t.Cleanup(server.Close)
		}

		return net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	}

	t.Fatal("could not bind consecutive ports for the scan test")

	return ""
}

// respond serves a fixed body on every request
func respond(payload string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(payload))
	})
}

func Test_Scan(t *testing.T) {
	address := serveConsecutive(t,
		`{"status":"alive","product":"fuku","instance":"first","project":"aaaaaaaaaaaaaaaa"}`,
		`{"status":"alive","product":"other"}`,
		`{"status":"alive","product":"fuku","instance":"third"}`,
	)

	found := Scan(t.Context(), address)

	require.Len(t, found, 2, "a server that is not fuku must not be reported as an instance")
	assert.Equal(t, "first", found[0].ID)
	assert.Equal(t, "aaaaaaaaaaaaaaaa", found[0].Project)
	assert.Equal(t, "third", found[1].ID)
	assert.Empty(t, found[1].Project, "an instance without project identity is reported with an empty project")
}

func Test_Running_SkipsAForeignInstanceInTheRange(t *testing.T) {
	mine := Fingerprint("/tmp/project")
	address := serveConsecutive(t,
		`{"status":"alive","product":"fuku","instance":"theirs","project":"bbbbbbbbbbbbbbbb"}`,
		`{"status":"alive","product":"fuku","instance":"mine","project":"`+mine+`"}`,
	)

	found, ok := Running(t.Context(), address, mine)

	require.True(t, ok)
	assert.NotEqual(t, address, found, "the guard must skip the instance serving another project")
}

func Test_Scan_NoInstances(t *testing.T) {
	assert.Empty(t, Scan(t.Context(), "127.0.0.1:1"))
	assert.Empty(t, Scan(t.Context(), "not-an-address"))
}

func Test_Running_StopsAtTheFirstMatch(t *testing.T) {
	fingerprint := Fingerprint("/tmp/project")
	probes := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		probes++

		w.Write([]byte(`{"status":"alive","product":"fuku","project":"` + fingerprint + `"}`))
	}))
	defer server.Close()

	_, found := Running(t.Context(), listenAddress(t, server), fingerprint)

	require.True(t, found)
	assert.Equal(t, 1, probes, "the guard must stop probing once it finds its own instance")
}

func Test_Running_RejectsAMalformedHost(t *testing.T) {
	tests := []struct {
		name   string
		listen string
	}{
		{name: "control character", listen: "\x7f:9876"},
		{name: "space in host", listen: "a b:9876"},
		{name: "broken IP literal", listen: "][:9876"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			address, found := Running(t.Context(), tt.listen, Fingerprint("/tmp/project"))

			assert.False(t, found, "a listen address that cannot form a URL must not report an instance")
			assert.Empty(t, address)
			assert.Empty(t, Scan(t.Context(), tt.listen))
		})
	}
}
