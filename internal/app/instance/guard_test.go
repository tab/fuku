package instance

import (
	"context"
	"io"
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

const testProject = "/Users/dev/projects/shop"

var _ http.RoundTripper = (*countingTransport)(nil)

// countingTransport answers every probe from a canned response and records the addresses it saw
type countingTransport struct {
	requests []*http.Request
	match    string
}

func (c *countingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	c.requests = append(c.requests, req)

	if req.URL.Host != c.match {
		return nil, net.ErrClosed
	}

	body := `{"status":"alive","product":"fuku","instance":"other","fingerprint":"` + Fingerprint(testProject) + `"}`

	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
		Request:    req,
	}, nil
}

func Test_NewGuard(t *testing.T) {
	var buf strings.Builder

	cfg := &config.Config{}
	cfg.Server.Listen = "127.0.0.1:9876"

	g, ok := NewGuard(cfg, Identity{Fingerprint: Fingerprint(testProject)}, &buf).(*guard)
	require.True(t, ok)

	assert.Equal(t, ProbeTimeout, g.httpClient.Timeout)
	require.NotNil(t, g.httpClient.CheckRedirect)
	assert.ErrorIs(t, g.httpClient.CheckRedirect(nil, nil), http.ErrUseLastResponse)
}

func Test_Guard_Check(t *testing.T) {
	tests := []struct {
		name    string
		listen  func(t *testing.T) string
		refused bool
	}{
		{
			name: "same project",
			listen: func(t *testing.T) string {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path != livePath {
						w.WriteHeader(http.StatusNotFound)

						return
					}

					w.Header().Set("Content-Type", "application/json")
					w.Write([]byte(`{"status":"alive","product":"fuku","instance":"other","fingerprint":"` + Fingerprint(testProject) + `"}`))
				}))
				t.Cleanup(server.Close)

				return strings.TrimPrefix(server.URL, "http://")
			},
			refused: true,
		},
		{
			name: "another project",
			listen: func(t *testing.T) string {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					w.Write([]byte(`{"status":"alive","product":"fuku","fingerprint":"0123456789abcdef"}`))
				}))
				t.Cleanup(server.Close)

				return strings.TrimPrefix(server.URL, "http://")
			},
			refused: false,
		},
		{
			name: "another product",
			listen: func(t *testing.T) string {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					w.Write([]byte(`{"status":"alive","product":"other","fingerprint":"` + Fingerprint(testProject) + `"}`))
				}))
				t.Cleanup(server.Close)

				return strings.TrimPrefix(server.URL, "http://")
			},
			refused: false,
		},
		{
			name: "missing fingerprint",
			listen: func(t *testing.T) string {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					w.Write([]byte(`{"status":"alive","product":"fuku"}`))
				}))
				t.Cleanup(server.Close)

				return strings.TrimPrefix(server.URL, "http://")
			},
			refused: false,
		},
		{
			name: "invalid JSON",
			listen: func(t *testing.T) string {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					w.Write([]byte(`not json`))
				}))
				t.Cleanup(server.Close)

				return strings.TrimPrefix(server.URL, "http://")
			},
			refused: false,
		},
		{
			name: "matching JSON followed by trailing data",
			listen: func(t *testing.T) string {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					w.Write([]byte(`{"status":"alive","product":"fuku","fingerprint":"` + Fingerprint(testProject) + `"}{"product":"fuku"}`))
				}))
				t.Cleanup(server.Close)

				return strings.TrimPrefix(server.URL, "http://")
			},
			refused: false,
		},
		{
			name: "matching JSON followed by trailing garbage",
			listen: func(t *testing.T) string {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					w.Write([]byte(`{"status":"alive","product":"fuku","fingerprint":"` + Fingerprint(testProject) + `"}` + "\x00garbage"))
				}))
				t.Cleanup(server.Close)

				return strings.TrimPrefix(server.URL, "http://")
			},
			refused: false,
		},
		{
			name: "non-200 response",
			listen: func(t *testing.T) string {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusServiceUnavailable)
					w.Write([]byte(`{"status":"alive","product":"fuku","fingerprint":"` + Fingerprint(testProject) + `"}`))
				}))
				t.Cleanup(server.Close)

				return strings.TrimPrefix(server.URL, "http://")
			},
			refused: false,
		},
		{
			name: "redirect to a matching instance",
			listen: func(t *testing.T) string {
				target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					w.Write([]byte(`{"status":"alive","product":"fuku","fingerprint":"` + Fingerprint(testProject) + `"}`))
				}))
				t.Cleanup(target.Close)

				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					http.Redirect(w, r, target.URL+livePath, http.StatusFound)
				}))
				t.Cleanup(server.Close)

				return strings.TrimPrefix(server.URL, "http://")
			},
			refused: false,
		},
		{
			name: "unreachable address",
			listen: func(t *testing.T) string {
				listener, err := net.Listen("tcp", "127.0.0.1:0")
				require.NoError(t, err)

				address := listener.Addr().String()
				require.NoError(t, listener.Close())

				return address
			},
			refused: false,
		},
		{
			name: "listen address without a port",
			listen: func(_ *testing.T) string {
				return "127.0.0.1"
			},
			refused: false,
		},
		{
			name: "listen port that is not a number",
			listen: func(_ *testing.T) string {
				return "127.0.0.1:http"
			},
			refused: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf strings.Builder

			listen := tt.listen(t)

			cfg := &config.Config{}
			cfg.Server.Listen = listen

			err := NewGuard(cfg, Identity{Fingerprint: Fingerprint(testProject)}, &buf).Check(t.Context())

			if !tt.refused {
				require.NoError(t, err)
				assert.Empty(t, buf.String())

				return
			}

			require.ErrorIs(t, err, errors.ErrInstanceAlreadyRunning)
			assert.Contains(t, buf.String(), "fuku is already running for this project")
			assert.Contains(t, buf.String(), listen)
			assert.Contains(t, buf.String(), "fuku logs")
		})
	}
}

func Test_Guard_Check_ProbesBoundedPortRange(t *testing.T) {
	const base = 19000

	var buf strings.Builder

	last := net.JoinHostPort("127.0.0.1", strconv.Itoa(base+config.APIPortRetries-1))
	transport := &countingTransport{match: last}

	cfg := &config.Config{}
	cfg.Server.Listen = net.JoinHostPort("127.0.0.1", strconv.Itoa(base))

	g, ok := NewGuard(cfg, Identity{Fingerprint: Fingerprint(testProject)}, &buf).(*guard)
	require.True(t, ok)

	g.httpClient.Transport = transport

	err := g.Check(t.Context())

	require.ErrorIs(t, err, errors.ErrInstanceAlreadyRunning)
	assert.Contains(t, buf.String(), last)
	require.Len(t, transport.requests, config.APIPortRetries)

	for i, req := range transport.requests {
		assert.Equal(t, net.JoinHostPort("127.0.0.1", strconv.Itoa(base+i)), req.URL.Host)
		assert.Equal(t, livePath, req.URL.Path)
		assert.Empty(t, req.Header.Get("Authorization"))
	}
}

func Test_Guard_Check_RejectsOversizedBody(t *testing.T) {
	var buf strings.Builder

	body := `{"status":"alive","product":"fuku","fingerprint":"` + Fingerprint(testProject) + `","padding":"` + strings.Repeat("x", ProbeResponseSize) + `"}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(body))
	}))
	defer server.Close()

	cfg := &config.Config{}
	cfg.Server.Listen = strings.TrimPrefix(server.URL, "http://")

	err := NewGuard(cfg, Identity{Fingerprint: Fingerprint(testProject)}, &buf).Check(t.Context())

	require.NoError(t, err)
	assert.Empty(t, buf.String())
}

func Test_Guard_Check_AcceptsBodyAtSizeLimit(t *testing.T) {
	var buf strings.Builder

	head := `{"product":"fuku","fingerprint":"` + Fingerprint(testProject) + `","padding":"`
	tail := `"}`
	body := head + strings.Repeat("x", ProbeResponseSize-len(head)-len(tail)) + tail

	require.Len(t, body, ProbeResponseSize)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(body))
	}))
	defer server.Close()

	cfg := &config.Config{}
	cfg.Server.Listen = strings.TrimPrefix(server.URL, "http://")

	err := NewGuard(cfg, Identity{Fingerprint: Fingerprint(testProject)}, &buf).Check(t.Context())

	require.ErrorIs(t, err, errors.ErrInstanceAlreadyRunning)
	assert.Contains(t, buf.String(), cfg.Server.Listen)
}

func Test_Guard_Check_SendsNoToken(t *testing.T) {
	var (
		buf     strings.Builder
		headers http.Header
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		headers = r.Header.Clone()

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"alive","product":"fuku","fingerprint":"` + Fingerprint(testProject) + `"}`))
	}))
	defer server.Close()

	cfg := &config.Config{}
	cfg.Server.Listen = strings.TrimPrefix(server.URL, "http://")

	err := NewGuard(cfg, Identity{Fingerprint: Fingerprint(testProject)}, &buf).Check(t.Context())

	require.ErrorIs(t, err, errors.ErrInstanceAlreadyRunning)
	assert.Empty(t, headers.Get("Authorization"))
}

func Test_Guard_Check_CancelledContext(t *testing.T) {
	var buf strings.Builder

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"alive","product":"fuku","fingerprint":"` + Fingerprint(testProject) + `"}`))
	}))
	defer server.Close()

	cfg := &config.Config{}
	cfg.Server.Listen = strings.TrimPrefix(server.URL, "http://")

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	err := NewGuard(cfg, Identity{Fingerprint: Fingerprint(testProject)}, &buf).Check(ctx)

	require.NoError(t, err)
	assert.Empty(t, buf.String())
}
