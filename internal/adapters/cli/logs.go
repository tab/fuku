package cli

import (
	"context"

	"fuku/internal/app/logs"
)

// Session streams the logs of the running instance
type Session interface {
	Run(ctx context.Context, request logs.Request) error
}

// Logs runs the log session until the stream ends or the run is cancelled
type Logs struct {
	request logs.Request
	session Session
}

// NewLogs creates the logs command for one request
func NewLogs(request logs.Request, session Session) *Logs {
	return &Logs{request: request, session: session}
}

// Run streams the requested logs and returns the exit code (1 with the session's failure)
func (c *Logs) Run(ctx context.Context) (int, error) {
	if err := c.session.Run(ctx, c.request); err != nil {
		return 1, err
	}

	return 0, nil
}
