package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
