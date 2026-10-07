package rest

import (
	"bufio"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"fuku/internal/contracts"
)

func Test_AuthMiddleware(t *testing.T) {
	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		nextCalled = true

		w.WriteHeader(http.StatusOK)
	})

	unauthorized := `{"error":"unauthorized"}` + "\n"

	tests := []struct {
		name         string
		header       string
		expectStatus int
		expectNext   bool
		expectBody   string
	}{
		{
			name:         "valid token",
			header:       "Bearer test-token",
			expectStatus: http.StatusOK,
			expectNext:   true,
		},
		{
			name:         "missing header",
			header:       "",
			expectStatus: http.StatusUnauthorized,
			expectNext:   false,
			expectBody:   unauthorized,
		},
		{
			name:         "wrong token",
			header:       "Bearer wrong-token",
			expectStatus: http.StatusUnauthorized,
			expectNext:   false,
			expectBody:   unauthorized,
		},
		{
			name:         "missing bearer prefix",
			header:       "test-token",
			expectStatus: http.StatusUnauthorized,
			expectNext:   false,
			expectBody:   unauthorized,
		},
		{
			name:         "lowercase bearer prefix",
			header:       "bearer test-token",
			expectStatus: http.StatusOK,
			expectNext:   true,
		},
		{
			name:         "mixed case bearer prefix",
			header:       "BEARER test-token",
			expectStatus: http.StatusOK,
			expectNext:   true,
		},
		{
			name:         "basic auth instead of bearer",
			header:       "Basic dXNlcjpwYXNz",
			expectStatus: http.StatusUnauthorized,
			expectNext:   false,
			expectBody:   unauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nextCalled = false

			handler := authMiddleware("test-token", next)

			req := httptest.NewRequest(http.MethodGet, "/api/v1/status", nil)
			req.Header.Set("Authorization", tt.header)

			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			assert.Equal(t, tt.expectStatus, w.Code)
			assert.Equal(t, tt.expectNext, nextCalled)
			assert.Equal(t, tt.expectBody, w.Body.String())
		})
	}
}

func Test_TelemetryMiddleware(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPublisher := NewMockPublisher(ctrl)

	var status int

	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
	})

	handler := telemetryMiddleware(mockPublisher, next)

	tests := []struct {
		name   string
		before func() (*http.Request, *httptest.ResponseRecorder)
		status int
	}{
		{
			name: "publishes the request with its status",
			before: func() (*http.Request, *httptest.ResponseRecorder) {
				status = http.StatusOK

				assertRequestPublished := func(msg contracts.Message) {
					assert.Equal(t, contracts.EventAPIRequested, msg.Type)

					data, ok := msg.Data.(contracts.APIRequested)
					require.True(t, ok)
					assert.Equal(t, http.MethodGet, data.Method)
					assert.Equal(t, "/api/v1/status", data.Path)
					assert.Equal(t, http.StatusOK, data.Status)
					assert.Greater(t, data.Duration, time.Duration(0))
				}

				mockPublisher.EXPECT().Publish(gomock.Any()).Do(assertRequestPublished)

				return httptest.NewRequest(http.MethodGet, "/api/v1/status", nil), httptest.NewRecorder()
			},
			status: http.StatusOK,
		},
		{
			name: "captures a non-default status code",
			before: func() (*http.Request, *httptest.ResponseRecorder) {
				status = http.StatusNotFound

				assertNotFoundPublished := func(msg contracts.Message) {
					data, ok := msg.Data.(contracts.APIRequested)
					require.True(t, ok)
					assert.Equal(t, "/api/v1/services/unknown", data.Path)
					assert.Equal(t, http.StatusNotFound, data.Status)
				}

				mockPublisher.EXPECT().Publish(gomock.Any()).Do(assertNotFoundPublished)

				return httptest.NewRequest(http.MethodGet, "/api/v1/services/unknown", nil), httptest.NewRecorder()
			},
			status: http.StatusNotFound,
		},
		{
			name: "captures the matched route without the method",
			before: func() (*http.Request, *httptest.ResponseRecorder) {
				status = http.StatusOK

				assertRoutePublished := func(msg contracts.Message) {
					data, ok := msg.Data.(contracts.APIRequested)
					require.True(t, ok)
					assert.Equal(t, "/api/v1/services/{id}", data.Route)
				}

				mockPublisher.EXPECT().Publish(gomock.Any()).Do(assertRoutePublished)

				req := httptest.NewRequest(http.MethodGet, "/api/v1/services/test-id", nil)
				req.Pattern = "GET /api/v1/services/{id}"

				return req, httptest.NewRecorder()
			},
			status: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, w := tt.before()

			handler.ServeHTTP(w, req)

			assert.Equal(t, tt.status, w.Code)
		})
	}
}

func Test_route(t *testing.T) {
	tests := []struct {
		name     string
		pattern  string
		expected string
	}{
		{
			name:     "strips the method from a matched pattern",
			pattern:  "GET /api/v1/services/{id}",
			expected: "/api/v1/services/{id}",
		},
		{
			name:     "a pattern registered without a method is returned as-is",
			pattern:  "/api/v1/",
			expected: "/api/v1/",
		},
		{
			name:     "an unmatched request has no pattern",
			pattern:  "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := route(tt.pattern)

			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_ResponseWriter_DefaultStatus(t *testing.T) {
	w := httptest.NewRecorder()
	rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}

	rw.Write([]byte("hello"))

	assert.Equal(t, http.StatusOK, rw.status)
}

func Test_ResponseWriter_WriteHeader(t *testing.T) {
	w := httptest.NewRecorder()
	rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}

	rw.WriteHeader(http.StatusCreated)

	assert.Equal(t, http.StatusCreated, rw.status)
	assert.Equal(t, http.StatusCreated, w.Code)
}

func Test_GuardMiddleware(t *testing.T) {
	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		nextCalled = true

		w.WriteHeader(http.StatusOK)
	})

	handler := guardMiddleware(next)

	forbidden := `{"error":"forbidden"}` + "\n"

	tests := []struct {
		name         string
		host         string
		origin       []string
		expectStatus int
		expectNext   bool
		expectBody   string
	}{
		{
			name:         "localhost with a port",
			host:         "localhost:3858",
			expectStatus: http.StatusOK,
			expectNext:   true,
		},
		{
			name:         "localhost without a port",
			host:         "localhost",
			expectStatus: http.StatusOK,
			expectNext:   true,
		},
		{
			name:         "ip6-localhost with a port",
			host:         "ip6-localhost:3858",
			expectStatus: http.StatusOK,
			expectNext:   true,
		},
		{
			name:         "ip6-localhost without a port",
			host:         "ip6-localhost",
			expectStatus: http.StatusOK,
			expectNext:   true,
		},
		{
			name:         "127.0.0.1 with a port",
			host:         "127.0.0.1:3858",
			expectStatus: http.StatusOK,
			expectNext:   true,
		},
		{
			name:         "127.0.0.1 without a port",
			host:         "127.0.0.1",
			expectStatus: http.StatusOK,
			expectNext:   true,
		},
		{
			name:         "127.0.0.2 with a port",
			host:         "127.0.0.2:3858",
			expectStatus: http.StatusOK,
			expectNext:   true,
		},
		{
			name:         "127.0.0.2 without a port",
			host:         "127.0.0.2",
			expectStatus: http.StatusOK,
			expectNext:   true,
		},
		{
			name:         "IPv6 loopback with a port",
			host:         "[::1]:3858",
			expectStatus: http.StatusOK,
			expectNext:   true,
		},
		{
			name:         "IPv6 loopback without a port",
			host:         "[::1]",
			expectStatus: http.StatusOK,
			expectNext:   true,
		},
		{
			name:         "a domain is forbidden",
			host:         "example.com:3858",
			expectStatus: http.StatusForbidden,
			expectNext:   false,
			expectBody:   forbidden,
		},
		{
			name:         "a private address is forbidden",
			host:         "10.0.0.1:3858",
			expectStatus: http.StatusForbidden,
			expectNext:   false,
			expectBody:   forbidden,
		},
		{
			name:         "a missing host is forbidden",
			host:         "",
			expectStatus: http.StatusForbidden,
			expectNext:   false,
			expectBody:   forbidden,
		},
		{
			name:         "a localhost subdomain is forbidden",
			host:         "localhost.example.com:3858",
			expectStatus: http.StatusForbidden,
			expectNext:   false,
			expectBody:   forbidden,
		},
		{
			name:         "a loopback origin is forbidden",
			host:         "127.0.0.1:3858",
			origin:       []string{"http://localhost:3858"},
			expectStatus: http.StatusForbidden,
			expectNext:   false,
			expectBody:   forbidden,
		},
		{
			name:         "a null origin is forbidden",
			host:         "127.0.0.1:3858",
			origin:       []string{"null"},
			expectStatus: http.StatusForbidden,
			expectNext:   false,
			expectBody:   forbidden,
		},
		{
			name:         "an empty origin is forbidden",
			host:         "127.0.0.1:3858",
			origin:       []string{""},
			expectStatus: http.StatusForbidden,
			expectNext:   false,
			expectBody:   forbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nextCalled = false

			req := httptest.NewRequest(http.MethodGet, "/api/v1/status", nil)
			req.Host = tt.host
			req.Header["Origin"] = tt.origin

			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			assert.Equal(t, tt.expectStatus, w.Code)
			assert.Equal(t, tt.expectNext, nextCalled)
			assert.Equal(t, tt.expectBody, w.Body.String())
		})
	}
}

func Test_GuardMiddleware_AbsoluteTarget(t *testing.T) {
	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		nextCalled = true

		w.WriteHeader(http.StatusOK)
	})

	handler := guardMiddleware(next)

	forbidden := `{"error":"forbidden"}` + "\n"

	tests := []struct {
		name string
		raw  string
	}{
		{
			name: "an absolute target without a Host header is forbidden",
			raw:  "GET http://127.0.0.1:3858/api/v1/status HTTP/1.0\r\n\r\n",
		},
		{
			name: "an absolute target with a loopback Host header is forbidden",
			raw:  "GET http://127.0.0.1:3858/api/v1/status HTTP/1.0\r\nHost: 127.0.0.1:3858\r\n\r\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nextCalled = false

			req, err := http.ReadRequest(bufio.NewReader(strings.NewReader(tt.raw)))
			require.NoError(t, err)

			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			assert.Equal(t, http.StatusForbidden, w.Code)
			assert.False(t, nextCalled)
			assert.Equal(t, forbidden, w.Body.String())
		})
	}
}
