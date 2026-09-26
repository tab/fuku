package readiness

import (
	"context"
	"log/slog"
	"net"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"fuku/internal/contracts"
	"fuku/internal/model"
)

func Test_Checker_checkTCP(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPublisher := NewMockPublisher(ctrl)

	log := slog.New(slog.DiscardHandler)

	checker := NewChecker(mockPublisher, log)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	closed := listener.Addr().String()
	require.NoError(t, listener.Close())

	tests := []struct {
		name     string
		before   func(t *testing.T) (context.Context, string, <-chan struct{})
		timeout  time.Duration
		expected error
	}{
		{
			name: "an accepting listener is ready",
			before: func(t *testing.T) (context.Context, string, <-chan struct{}) {
				listener, err := net.Listen("tcp", "127.0.0.1:0")
				require.NoError(t, err)
				t.Cleanup(func() { listener.Close() })

				return t.Context(), listener.Addr().String(), make(chan struct{})
			},
			timeout: 5 * time.Second,
		},
		{
			name: "a cancelled context stops the check",
			before: func(t *testing.T) (context.Context, string, <-chan struct{}) {
				ctx, cancel := context.WithCancel(t.Context())
				cancel()

				return ctx, closed, make(chan struct{})
			},
			timeout:  5 * time.Second,
			expected: context.Canceled,
		},
		{
			name: "an exited process stops the check",
			before: func(t *testing.T) (context.Context, string, <-chan struct{}) {
				done := make(chan struct{})
				close(done)

				return t.Context(), closed, done
			},
			timeout:  5 * time.Second,
			expected: contracts.ErrProcessExited,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, address, done := tt.before(t)

			err := checker.checkTCP(ctx, address, tt.timeout, 10*time.Millisecond, done)

			require.ErrorIs(t, err, tt.expected)
		})
	}
}

func Test_Checker_checkTCP_EndsOnTheTimeout(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	closed := listener.Addr().String()
	require.NoError(t, listener.Close())

	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockPublisher := NewMockPublisher(ctrl)

		log := slog.New(slog.DiscardHandler)

		checker := NewChecker(mockPublisher, log)

		timeout := 50 * time.Millisecond
		start := time.Now()

		err := checker.checkTCP(t.Context(), closed, timeout, time.Second, make(chan struct{}))

		require.ErrorIs(t, err, contracts.ErrReadinessTimeout)
		assert.Equal(t, timeout, time.Since(start))
	})
}

func Test_Checker_ProbePort(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPublisher := NewMockPublisher(ctrl)

	log := slog.New(slog.DiscardHandler)

	checker := NewChecker(mockPublisher, log)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	closed := listener.Addr().String()
	require.NoError(t, listener.Close())

	tests := []struct {
		name   string
		before func(t *testing.T) (model.Readiness, model.Port)
	}{
		{
			name: "a listening TCP address is in use",
			before: func(t *testing.T) (model.Readiness, model.Port) {
				listener, err := net.Listen("tcp", "127.0.0.1:0")
				require.NoError(t, err)
				t.Cleanup(func() { listener.Close() })

				address := listener.Addr().String()

				return model.Readiness{Type: model.ReadinessTCP, Address: address}, model.Port{Address: address, InUse: true}
			},
		},
		{
			name: "a listening HTTP address is in use",
			before: func(t *testing.T) (model.Readiness, model.Port) {
				listener, err := net.Listen("tcp", "127.0.0.1:0")
				require.NoError(t, err)
				t.Cleanup(func() { listener.Close() })

				address := listener.Addr().String()

				return model.Readiness{Type: model.ReadinessHTTP, URL: "http://" + address + "/health"}, model.Port{Address: address, InUse: true}
			},
		},
		{
			name: "a free address is reported with nothing listening",
			before: func(_ *testing.T) (model.Readiness, model.Port) {
				return model.Readiness{Type: model.ReadinessTCP, Address: closed}, model.Port{Address: closed}
			},
		},
		{
			name: "a log probe names no address",
			before: func(_ *testing.T) (model.Readiness, model.Port) {
				return model.Readiness{Type: model.ReadinessLog, Pattern: "ready"}, model.Port{}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			readiness, expected := tt.before(t)

			port := checker.ProbePort(readiness)

			assert.Equal(t, expected, port)
		})
	}
}

func Test_extractAddress(t *testing.T) {
	tests := []struct {
		name      string
		readiness model.Readiness
		expected  string
	}{
		{
			name:      "HTTP type with port",
			readiness: model.Readiness{Type: model.ReadinessHTTP, URL: "http://localhost:8080/health"},
			expected:  "localhost:8080",
		},
		{
			name:      "HTTP type without port",
			readiness: model.Readiness{Type: model.ReadinessHTTP, URL: "http://localhost/health"},
			expected:  "localhost:80",
		},
		{
			name:      "HTTP type with IP address",
			readiness: model.Readiness{Type: model.ReadinessHTTP, URL: "http://127.0.0.1:3000/api"},
			expected:  "127.0.0.1:3000",
		},
		{
			name:      "TCP type with address",
			readiness: model.Readiness{Type: model.ReadinessTCP, Address: "localhost:9090"},
			expected:  "localhost:9090",
		},
		{
			name:      "TCP type with IP address",
			readiness: model.Readiness{Type: model.ReadinessTCP, Address: "0.0.0.0:8080"},
			expected:  "0.0.0.0:8080",
		},
		{
			name:      "TCP type with database port",
			readiness: model.Readiness{Type: model.ReadinessTCP, Address: "localhost:5432"},
			expected:  "localhost:5432",
		},
		{
			name:      "log type returns empty",
			readiness: model.Readiness{Type: model.ReadinessLog, Pattern: "ready"},
			expected:  "",
		},
		{
			name:      "unknown type returns empty",
			readiness: model.Readiness{Type: "unknown"},
			expected:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractAddress(tt.readiness)

			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_extractFromURL(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected string
	}{
		{
			name:     "URL with explicit port",
			url:      "http://localhost:8080/health",
			expected: "localhost:8080",
		},
		{
			name:     "URL with IP address and port",
			url:      "http://127.0.0.1:3000/api",
			expected: "127.0.0.1:3000",
		},
		{
			name:     "HTTP URL without port defaults to 80",
			url:      "http://localhost/health",
			expected: "localhost:80",
		},
		{
			name:     "HTTPS URL without port defaults to 443",
			url:      "https://localhost/health",
			expected: "localhost:443",
		},
		{
			name:     "URL with 0.0.0.0",
			url:      "http://0.0.0.0:8080/health",
			expected: "0.0.0.0:8080",
		},
		{
			name:     "IPv6 address with port",
			url:      "http://[::1]:8080/health",
			expected: "[::1]:8080",
		},
		{
			name:     "invalid URL returns empty",
			url:      "://invalid",
			expected: "",
		},
		{
			name:     "empty URL returns empty",
			url:      "",
			expected: "",
		},
		{
			name:     "unknown scheme without port returns empty",
			url:      "ftp://localhost/file",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractFromURL(tt.url)

			assert.Equal(t, tt.expected, result)
		})
	}
}
