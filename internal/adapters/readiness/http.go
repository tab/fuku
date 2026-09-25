package readiness

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"fuku/internal/contracts"
)

// checkHTTP polls url every interval until a 2xx answer, the timeout, a cancelled ctx or the process exit
func (c *Checker) checkHTTP(ctx context.Context, url string, timeout, interval time.Duration, done <-chan struct{}) error {
	client := &http.Client{}
	deadline := time.Now().Add(timeout)

	ctx, cancel := c.contextWithDone(ctx, done)
	defer cancel()

	for {
		if time.Until(deadline) <= 0 {
			return fmt.Errorf("%w: HTTP check after %v", contracts.ErrReadinessTimeout, timeout)
		}

		attempt, expire := context.WithDeadline(ctx, deadline)

		req, err := http.NewRequestWithContext(attempt, http.MethodGet, url, nil)
		if err != nil {
			expire()

			return fmt.Errorf("failed to create request: %w", err)
		}

		resp, err := client.Do(req)
		if resp != nil {
			resp.Body.Close()
		}

		expire()

		if err == nil && resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return nil
		}

		select {
		case <-ctx.Done():
			if c.isDone(done) {
				return contracts.ErrProcessExited
			}

			return ctx.Err()
		case <-time.After(min(interval, time.Until(deadline))):
		}
	}
}
