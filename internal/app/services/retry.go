package services

import (
	"context"
	"fmt"
	"time"

	"fuku/internal/contracts"
	"fuku/internal/model"
)

// startWithRetry runs start attempts with a constant backoff until one is ready, none are left or the run ends
func (r *Runtime) startWithRetry(ctx context.Context, tier string, svc model.Service) error {
	var lastErr error

	for number := 1; number <= r.options.RetryAttempts; number++ {
		if number > 1 {
			r.log.Info(fmt.Sprintf("Retrying service '%s' (attempt %d/%d)", svc.Name, number, r.options.RetryAttempts))

			select {
			case <-time.After(r.options.RetryBackoff):
			case <-ctx.Done():
				r.publishStopped(svc, tier)

				return ctx.Err()
			}
		}

		proc, err := r.attempt(ctx, tier, svc, number)
		if err == nil {
			r.watchForExit(proc)

			return nil
		}

		if ctx.Err() != nil {
			r.publishStopped(svc, tier)

			return ctx.Err()
		}

		lastErr = err
	}

	err := fmt.Errorf("%w after %d attempts: %w", contracts.ErrMaxRetriesExceeded, r.options.RetryAttempts, lastErr)
	r.log.Error(fmt.Sprintf("Failed to start service '%s'", svc.Name), "error", err)
	r.publish(contracts.Message{
		Type: contracts.EventServiceFailed,
		Data: contracts.ServiceFailed{
			ServiceEvent: contracts.ServiceEvent{Service: svc, Tier: tier},
			Error:        err,
		},
	})

	return err
}
