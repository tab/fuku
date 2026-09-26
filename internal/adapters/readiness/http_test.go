package readiness

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"fuku/internal/contracts"
)

func Test_Checker_checkHTTP(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPublisher := NewMockPublisher(ctrl)

	log := slog.New(slog.DiscardHandler)

	checker := NewChecker(mockPublisher, log)

	tests := []struct {
		name     string
		before   func(t *testing.T) (context.Context, string, <-chan struct{})
		timeout  time.Duration
		expected error
	}{
		{
			name: "a 2xx answer is ready",
			before: func(t *testing.T) (context.Context, string, <-chan struct{}) {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusOK)
				}))
				t.Cleanup(server.Close)

				return t.Context(), server.URL, make(chan struct{})
			},
			timeout: 5 * time.Second,
		},
		{
			name: "an answer slower than the interval but within the timeout is ready",
			before: func(t *testing.T) (context.Context, string, <-chan struct{}) {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					select {
					case <-time.After(30 * time.Millisecond):
					case <-r.Context().Done():
					}

					w.WriteHeader(http.StatusOK)
				}))
				t.Cleanup(server.Close)

				return t.Context(), server.URL, make(chan struct{})
			},
			timeout: 5 * time.Second,
		},
		{
			name: "a failing endpoint times out",
			before: func(t *testing.T) (context.Context, string, <-chan struct{}) {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusInternalServerError)
				}))
				t.Cleanup(server.Close)

				return t.Context(), server.URL, make(chan struct{})
			},
			timeout:  50 * time.Millisecond,
			expected: contracts.ErrReadinessTimeout,
		},
		{
			name: "a cancelled context stops the check",
			before: func(t *testing.T) (context.Context, string, <-chan struct{}) {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusOK)
				}))
				t.Cleanup(server.Close)

				ctx, cancel := context.WithCancel(t.Context())
				cancel()

				return ctx, server.URL, make(chan struct{})
			},
			timeout:  5 * time.Second,
			expected: context.Canceled,
		},
		{
			name: "an exited process stops the check",
			before: func(t *testing.T) (context.Context, string, <-chan struct{}) {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusInternalServerError)
				}))
				t.Cleanup(server.Close)

				done := make(chan struct{})
				close(done)

				return t.Context(), server.URL, done
			},
			timeout:  5 * time.Second,
			expected: contracts.ErrProcessExited,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, url, done := tt.before(t)

			err := checker.checkHTTP(ctx, url, tt.timeout, 10*time.Millisecond, done)

			require.ErrorIs(t, err, tt.expected)
		})
	}
}

func Test_Checker_checkHTTP_EndsOnTheTimeout(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	closed := "http://" + listener.Addr().String()
	require.NoError(t, listener.Close())

	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockPublisher := NewMockPublisher(ctrl)

		log := slog.New(slog.DiscardHandler)

		checker := NewChecker(mockPublisher, log)

		timeout := 50 * time.Millisecond
		start := time.Now()

		err := checker.checkHTTP(t.Context(), closed, timeout, time.Second, make(chan struct{}))

		require.ErrorIs(t, err, contracts.ErrReadinessTimeout)
		assert.Equal(t, timeout, time.Since(start))
	})
}

func Test_Checker_checkHTTP_InvalidURL(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPublisher := NewMockPublisher(ctrl)

	log := slog.New(slog.DiscardHandler)

	checker := NewChecker(mockPublisher, log)

	err := checker.checkHTTP(t.Context(), "http://invalid\x00url", 5*time.Second, 10*time.Millisecond, make(chan struct{}))

	require.ErrorContains(t, err, "failed to create request")
}
