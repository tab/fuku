package rest

import (
	"net/http"
	"net/http/httptest"
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

				mockPublisher.EXPECT().Publish(gomock.Any()).Do(func(msg contracts.Message) {
					assert.Equal(t, contracts.EventAPIRequested, msg.Type)

					data, ok := msg.Data.(contracts.APIRequested)
					require.True(t, ok)
					assert.Equal(t, http.MethodGet, data.Method)
					assert.Equal(t, "/api/v1/status", data.Path)
					assert.Equal(t, http.StatusOK, data.Status)
					assert.Greater(t, data.Duration, time.Duration(0))
				})

				return httptest.NewRequest(http.MethodGet, "/api/v1/status", nil), httptest.NewRecorder()
			},
			status: http.StatusOK,
		},
		{
			name: "captures a non-default status code",
			before: func() (*http.Request, *httptest.ResponseRecorder) {
				status = http.StatusNotFound

				mockPublisher.EXPECT().Publish(gomock.Any()).Do(func(msg contracts.Message) {
					data, ok := msg.Data.(contracts.APIRequested)
					require.True(t, ok)
					assert.Equal(t, "/api/v1/services/unknown", data.Path)
					assert.Equal(t, http.StatusNotFound, data.Status)
				})

				return httptest.NewRequest(http.MethodGet, "/api/v1/services/unknown", nil), httptest.NewRecorder()
			},
			status: http.StatusNotFound,
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
