package readiness

import (
	"context"
	"net/http"
	"time"

	"fuku/internal/app/errors"
)

// HTTPChecker checks service readiness via HTTP endpoint
type HTTPChecker struct {
	url      string
	timeout  time.Duration
	interval time.Duration
	client   *http.Client
}

// NewHTTPChecker creates a new HTTP readiness checker
func NewHTTPChecker(url string, timeout, interval time.Duration) *HTTPChecker {
	return &HTTPChecker{
		url:      url,
		timeout:  timeout,
		interval: interval,
		client: &http.Client{
			Timeout: 2 * time.Second,
		},
	}
}

// Check performs HTTP health check with retries until timeout
func (h *HTTPChecker) Check(ctx context.Context) error {
	deadline := time.Now().Add(h.timeout)
	ticker := time.NewTicker(h.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if time.Now().After(deadline) {
				return errors.ErrReadinessCheckFailed
			}

			resp, err := h.client.Get(h.url)
			if err == nil && resp.StatusCode >= 200 && resp.StatusCode < 300 {
				resp.Body.Close()
				return nil
			}
			if resp != nil {
				resp.Body.Close()
			}
		}
	}
}
