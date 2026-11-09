package mcp

import (
	"context"
	"errors"
	"fmt"
	"time"

	"fuku/internal/contracts"
	"fuku/internal/model"
)

// API calls the REST API of the running instance
type API interface {
	Live(ctx context.Context) (model.Instance, error)
	Status(ctx context.Context) (model.Snapshot, error)
	Services(ctx context.Context) ([]model.Service, error)
	Service(ctx context.Context, id string) (model.Service, error)
	Act(ctx context.Context, id string, action contracts.Action, wait time.Duration) (model.Service, bool, error)
}

// APIDialer returns the API client of a loopback address, sending the token when it is set
type APIDialer func(address, token string) (API, error)

// findAPI returns the API the status frame announces, or the configured one once its liveness answer names this project
func (s *Server) findAPI(ctx context.Context, status contracts.LogStatus, server model.Server, fingerprint string) (API, error) {
	if status.API != "" {
		return s.dialAPI(status.API, server.Token)
	}

	if server.Listen == "" {
		return nil, contracts.ErrAPIUnreachable
	}

	candidate, err := s.dialAPI(server.Listen, "")
	if err != nil {
		return nil, err
	}

	live, err := candidate.Live(ctx)
	if err != nil || live.Fingerprint != fingerprint {
		return nil, contracts.ErrAPIUnreachable
	}

	return s.dialAPI(server.Listen, server.Token)
}

// explain adds what the agent can do next to an error of the API
func explain(err error) error {
	switch {
	case errors.Is(err, contracts.ErrAPIUnauthorized):
		return errAPIUnauthorized
	case errors.Is(err, contracts.ErrAPIRejected):
		return fmt.Errorf("%w; call status to see the service state", err)
	case errors.Is(err, contracts.ErrAPIOverloaded):
		return fmt.Errorf("%w; retry the action", err)
	default:
		return err
	}
}

// uptime returns the whole seconds since a start time, or zero for a service without a process
func uptime(startedAt time.Time) int64 {
	if startedAt.IsZero() {
		return 0
	}

	return int64(time.Since(startedAt) / time.Second)
}
