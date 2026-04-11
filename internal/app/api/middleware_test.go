package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fuku/internal/app/bus"
	"fuku/internal/config"
)

func Test_AuthMiddleware(t *testing.T) {
	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		nextCalled = true

		w.WriteHeader(http.StatusOK)
	})

	tests := []struct {
		name         string
		header       string
		expectStatus int
		expectNext   bool
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
		},
		{
			name:         "wrong token",
			header:       "Bearer wrong-token",
			expectStatus: http.StatusUnauthorized,
			expectNext:   false,
		},
		{
			name:         "missing bearer prefix",
			header:       "test-token",
			expectStatus: http.StatusUnauthorized,
			expectNext:   false,
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
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nextCalled = false

			handler := authMiddleware("test-token", next)

			req := httptest.NewRequest(http.MethodGet, "/api/v1/status", nil)
			if tt.header != "" {
				req.Header.Set("Authorization", tt.header)
			}

			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			assert.Equal(t, tt.expectStatus, w.Code)
			assert.Equal(t, tt.expectNext, nextCalled)

			if !tt.expectNext {
				var body map[string]string
				require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
				assert.Equal(t, "unauthorized", body["error"])
			}
		})
	}
}

func Test_CorsMiddleware(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	tests := []struct {
		name         string
		method       string
		expectStatus int
		expectCORS   bool
	}{
		{
			name:         "GET request sets CORS headers",
			method:       http.MethodGet,
			expectStatus: http.StatusOK,
			expectCORS:   true,
		},
		{
			name:         "OPTIONS preflight returns 204",
			method:       http.MethodOptions,
			expectStatus: http.StatusNoContent,
			expectCORS:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := corsMiddleware(next)

			req := httptest.NewRequest(tt.method, "/api/v1/status", nil)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			assert.Equal(t, tt.expectStatus, w.Code)
			assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
			assert.Contains(t, w.Header().Get("Access-Control-Allow-Methods"), "GET")
			assert.Contains(t, w.Header().Get("Access-Control-Allow-Headers"), "Authorization")
		})
	}
}

func Test_TelemetryMiddleware(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Logs.Buffer = 10

	b := bus.NewBus(cfg, nil, nil)
	defer b.Close()

	ch := b.Subscribe(t.Context())

	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := telemetryMiddleware(b, next)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/status", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	select {
	case msg := <-ch:
		assert.Equal(t, bus.EventAPIRequest, msg.Type)

		data, ok := msg.Data.(bus.APIRequest)
		require.True(t, ok)
		assert.Equal(t, http.MethodGet, data.Method)
		assert.Equal(t, "/api/v1/status", data.Path)
		assert.Equal(t, http.StatusOK, data.Status)
		assert.Greater(t, data.Duration, time.Duration(0))
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Expected telemetry event")
	}
}

func Test_TelemetryMiddleware_CapturesStatusCode(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Logs.Buffer = 10

	b := bus.NewBus(cfg, nil, nil)
	defer b.Close()

	ch := b.Subscribe(t.Context())

	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	handler := telemetryMiddleware(b, next)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/services/unknown", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	select {
	case msg := <-ch:
		data := msg.Data.(bus.APIRequest)
		assert.Equal(t, http.StatusNotFound, data.Status)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Expected telemetry event")
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
