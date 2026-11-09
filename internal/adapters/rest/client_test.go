package rest

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fuku/internal/contracts"
	"fuku/internal/model"
)

func Test_NewClient(t *testing.T) {
	tests := []struct {
		name          string
		address       string
		expectedBase  string
		expectedError error
	}{
		{name: "an IPv4 loopback address", address: "127.0.0.1:3858", expectedBase: "http://127.0.0.1:3858"},
		{name: "localhost", address: "localhost:3858", expectedBase: "http://localhost:3858"},
		{name: "the IPv6 loopback", address: "[::1]:3858", expectedBase: "http://[::1]:3858"},
		{name: "ip6-localhost maps to ::1", address: "ip6-localhost:3858", expectedBase: "http://[::1]:3858"},
		{name: "an address off the loopback", address: "192.168.1.2:3858", expectedError: ErrClientNotLoopback},
		{name: "an address that is not host and port", address: "localhost", expectedError: ErrClientNotLoopback},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClient(NewHTTPClient(), tt.address, "secret-token")

			require.ErrorIs(t, err, tt.expectedError)

			if tt.expectedError != nil {
				assert.Nil(t, client)

				return
			}

			assert.Equal(t, tt.expectedBase, client.base)
			assert.Equal(t, "secret-token", client.token)
		})
	}
}

func Test_Client_Live(t *testing.T) {
	answer := func(product string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, pathLive, r.URL.Path)
			assert.Empty(t, r.Header.Get("Authorization"))
			fmt.Fprintf(w, `{"status":"alive","product":%q,"instance":"i-1","fingerprint":"this-project"}`, product)
		}
	}

	fuku := httptest.NewServer(answer("fuku"))
	defer fuku.Close()

	other := httptest.NewServer(answer("other"))
	defer other.Close()

	missing := httptest.NewServer(http.NotFoundHandler())
	defer missing.Close()

	closed := httptest.NewServer(http.NotFoundHandler())
	closedAddress := closed.Listener.Addr().String()
	closed.Close()

	tests := []struct {
		name          string
		address       string
		expected      model.Instance
		expectedError error
	}{
		{name: "a fuku API", address: fuku.Listener.Addr().String(), expected: model.Instance{ID: "i-1", Fingerprint: "this-project"}},
		{name: "another product", address: other.Listener.Addr().String(), expectedError: ErrClientNotFuku},
		{name: "no liveness route", address: missing.Listener.Addr().String(), expectedError: ErrClientFailed},
		{name: "no listener", address: closedAddress, expectedError: contracts.ErrAPIUnreachable},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClient(NewHTTPClient(), tt.address, "")
			require.NoError(t, err)

			result, err := client.Live(t.Context())

			require.ErrorIs(t, err, tt.expectedError)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_Client_Live_IP6Localhost(t *testing.T) {
	listener, err := net.Listen("tcp", "[::1]:0")
	if err != nil {
		t.Skipf("the IPv6 loopback ::1 is unavailable: %v", err)
	}

	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `{"status":"alive","product":"fuku","instance":"i-1","fingerprint":"this-project"}`)
	}))
	server.Listener.Close()
	server.Listener = listener
	server.Start()

	defer server.Close()

	_, port, err := net.SplitHostPort(listener.Addr().String())
	require.NoError(t, err)

	client, err := NewClient(NewHTTPClient(), "ip6-localhost:"+port, "")
	require.NoError(t, err)

	result, err := client.Live(t.Context())

	require.NoError(t, err)
	assert.Equal(t, "this-project", result.Fingerprint)
}

func Test_Client_Status(t *testing.T) {
	token := "secret-token"

	fuku := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, pathStatus, r.URL.Path)
		assert.Equal(t, "Bearer "+token, r.Header.Get("Authorization"))
		fmt.Fprint(w, `{"profile":"default","phase":"running","uptime":9}`)
	}))
	defer fuku.Close()

	denied := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer denied.Close()

	broken := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer broken.Close()

	garbled := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `not json`)
	}))
	defer garbled.Close()

	target := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Error("a redirect must not be followed with the token")
	}))
	defer target.Close()

	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+r.URL.Path, http.StatusFound)
	}))
	defer redirector.Close()

	tests := []struct {
		name          string
		address       string
		expected      model.Snapshot
		expectedError string
	}{
		{name: "the running profile", address: fuku.Listener.Addr().String(), expected: model.Snapshot{Profile: "default", Phase: model.PhaseRunning}},
		{name: "a rejected token", address: denied.Listener.Addr().String(), expectedError: contracts.ErrAPIUnauthorized.Error()},
		{name: "a failing API", address: broken.Listener.Addr().String(), expectedError: "the fuku REST API failed: /api/v1/status answered 500 Internal Server Error"},
		{name: "a redirect", address: redirector.Listener.Addr().String(), expectedError: "the fuku REST API failed: /api/v1/status answered 302 Found"},
		{name: "an answer that is not JSON", address: garbled.Listener.Addr().String(), expectedError: "failed to decode the API answer of /api/v1/status: invalid character 'o' in literal null (expecting 'u')"},
		{name: "an address that is not a valid URL host", address: "127.0.0.1:abc", expectedError: "failed to build the API request: parse \"http://127.0.0.1:abc/api/v1/status\": invalid port \":abc\" after host"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClient(NewHTTPClient(), tt.address, token)
			require.NoError(t, err)

			result, err := client.Status(t.Context())

			if tt.expectedError != "" {
				require.EqualError(t, err, tt.expectedError)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_Client_Services(t *testing.T) {
	fuku := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, pathServices, r.URL.Path)
		fmt.Fprint(w, `{"services":[{"id":"u-1","name":"api","tier":"foundation","status":"running","watching":true,"pid":42,"cpu":1.5,"memory":64,"uptime":7},{"id":"u-2","name":"db","status":"failed","error":"exit status 1","pid":0,"uptime":0}]}`)
	}))
	defer fuku.Close()

	client, err := NewClient(NewHTTPClient(), fuku.Listener.Addr().String(), "")
	require.NoError(t, err)

	result, err := client.Services(t.Context())
	require.NoError(t, err)
	require.Len(t, result, 2)

	api, db := result[0], result[1]
	assert.InDelta(t, 7, time.Since(api.Process.StartedAt).Seconds(), 1)

	api.Process.StartedAt = time.Time{}
	assert.Equal(t, model.Service{ID: "u-1", Name: "api", Tier: "foundation", Status: model.StatusRunning, Watching: true, Process: model.Process{PID: 42, CPU: 1.5, Memory: 64}}, api)
	assert.Equal(t, model.Service{ID: "u-2", Name: "db", Status: model.StatusFailed, Error: "exit status 1"}, db)

	fuku.Close()

	_, err = client.Services(t.Context())
	require.ErrorIs(t, err, contracts.ErrAPIUnreachable)
}

func Test_Client_Service(t *testing.T) {
	fuku := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case pathServices + "/u-1":
			fmt.Fprint(w, `{"id":"u-1","name":"api","status":"starting","pid":0}`)
		default:
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprint(w, `{"error":"service not found"}`)
		}
	}))
	defer fuku.Close()

	client, err := NewClient(NewHTTPClient(), fuku.Listener.Addr().String(), "")
	require.NoError(t, err)

	result, err := client.Service(t.Context(), "u-1")
	require.NoError(t, err)
	assert.Equal(t, model.Service{ID: "u-1", Name: "api", Status: model.StatusStarting}, result)

	_, err = client.Service(t.Context(), "u-2")
	require.ErrorIs(t, err, contracts.ErrAPIRejected)
	require.EqualError(t, err, "fuku rejected the action: service not found")

	fuku.Close()

	_, err = client.Service(t.Context(), "u-1")
	require.ErrorIs(t, err, contracts.ErrAPIUnreachable)
	assert.NotContains(t, err.Error(), "u-1")
}

func Test_Client_Act(t *testing.T) {
	token := "secret-token"

	serve := func(answer http.HandlerFunc) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method)
			assert.Equal(t, pathServices+"/u-1/restart", r.URL.Path)
			assert.Equal(t, "Bearer "+token, r.Header.Get("Authorization"))
			assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

			body, err := io.ReadAll(r.Body)
			assert.NoError(t, err)
			assert.JSONEq(t, `{"wait":45}`, string(body))

			answer(w, r)
		}))
	}
	status := func(code int, body string) http.HandlerFunc {
		return func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(code)
			fmt.Fprint(w, body)
		}
	}

	tests := []struct {
		name            string
		answer          http.HandlerFunc
		expected        model.Service
		expectedSettled bool
		expectedError   string
	}{
		{
			name:            "an action that settles",
			answer:          status(http.StatusOK, `{"id":"u-1","name":"api","status":"running","watching":true,"pid":77}`),
			expected:        model.Service{ID: "u-1", Name: "api", Status: model.StatusRunning, Watching: true, Process: model.Process{PID: 77}},
			expectedSettled: true,
		},
		{
			name:   "an action the wait outlives",
			answer: status(http.StatusAccepted, `{"id":"u-1","name":"api","action":"restart","status":"restarting"}`),
		},
		{
			name: "an answer after the read timeout",
			answer: func(w http.ResponseWriter, r *http.Request) {
				select {
				case <-time.After(requestTimeout + 500*time.Millisecond):
				case <-r.Context().Done():
				}

				fmt.Fprint(w, `{"id":"u-1","name":"api","status":"running","pid":78}`)
			},
			expected:        model.Service{ID: "u-1", Name: "api", Status: model.StatusRunning, Process: model.Process{PID: 78}},
			expectedSettled: true,
		},
		{name: "a rejected action", answer: status(http.StatusConflict, `{"error":"service is not running"}`), expectedError: "fuku rejected the action: service is not running"},
		{name: "a rejection without JSON", answer: status(http.StatusNotFound, `gone`), expectedError: "fuku rejected the action: 404 Not Found"},
		{name: "an overloaded instance", answer: status(http.StatusInternalServerError, `{"error":"instance is overloaded"}`), expectedError: contracts.ErrAPIOverloaded.Error()},
		{name: "a rejected token", answer: status(http.StatusUnauthorized, `{"error":"unauthorized"}`), expectedError: contracts.ErrAPIUnauthorized.Error()},
		{name: "an unexpected status", answer: status(http.StatusTeapot, ``), expectedError: "the fuku REST API failed: answered 418 I'm a teapot"},
		{name: "an answer that is not JSON", answer: status(http.StatusOK, `not json`), expectedError: "failed to decode the API answer: invalid character 'o' in literal null (expecting 'u')"},
		{
			name: "a dropped connection",
			answer: func(w http.ResponseWriter, _ *http.Request) {
				conn, _, err := w.(http.Hijacker).Hijack()
				if assert.NoError(t, err) {
					conn.Close()
				}
			},
			expectedError: "the fuku REST API is off or unreachable: EOF",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := serve(tt.answer)
			defer server.Close()

			client, err := NewClient(NewHTTPClient(), server.Listener.Addr().String(), token)
			require.NoError(t, err)

			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()

			result, settled, err := client.Act(ctx, "u-1", contracts.ActionRestart, 45*time.Second)

			if tt.expectedError != "" {
				require.EqualError(t, err, tt.expectedError)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
			assert.Equal(t, tt.expectedSettled, settled)
		})
	}
}
