package readiness

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"fuku/internal/app/errors"
)

func Test_HTTPChecker_Check_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	checker := NewHTTPChecker(server.URL, 5*time.Second, 100*time.Millisecond)
	ctx := context.Background()

	err := checker.Check(ctx)
	assert.NoError(t, err)
}

func Test_HTTPChecker_Check_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	checker := NewHTTPChecker(server.URL, 500*time.Millisecond, 100*time.Millisecond)
	ctx := context.Background()

	err := checker.Check(ctx)
	assert.Equal(t, errors.ErrReadinessCheckFailed, err)
}

func Test_HTTPChecker_Check_ContextCancelled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	checker := NewHTTPChecker(server.URL, 5*time.Second, 200*time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	err := checker.Check(ctx)
	assert.Equal(t, context.Canceled, err)
}

func Test_HTTPChecker_Check_EventualSuccess(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
		} else {
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	checker := NewHTTPChecker(server.URL, 5*time.Second, 100*time.Millisecond)
	ctx := context.Background()

	err := checker.Check(ctx)
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, callCount, 3)
}

func Test_HTTPChecker_Check_InvalidURL(t *testing.T) {
	checker := NewHTTPChecker("http://localhost:99999", 500*time.Millisecond, 100*time.Millisecond)
	ctx := context.Background()

	err := checker.Check(ctx)
	assert.Equal(t, errors.ErrReadinessCheckFailed, err)
}
