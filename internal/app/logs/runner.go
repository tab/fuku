package logs

import (
	"context"
	"io"
	"os"
	"os/signal"
	"syscall"

	"fuku/internal/config"
	"fuku/internal/config/logger"
)

// Runner handles logs mode operations
type Runner interface {
	Run(profile string, services []string) int
}

// runner implements the Runner interface
type runner struct {
	client Client
	log    logger.Logger
}

// NewRunner creates a new logs runner
func NewRunner(client Client, log logger.Logger) Runner {
	return &runner{
		client: client,
		log:    log.WithComponent("LOGS"),
	}
}

// Run handles the logs command to stream logs from a running instance
func (r *runner) Run(profile string, services []string) int {
	socketPath, err := FindSocket(config.SocketDir, profile)
	if err != nil {
		r.log.Error().Err(err).Msg("Failed to find socket")
		return 1
	}

	return r.streamLogs(socketPath, services, os.Stdout)
}

// streamLogs connects to a running fuku instance and streams logs
func (r *runner) streamLogs(socketPath string, services []string, output io.Writer) int {
	if err := r.client.Connect(socketPath); err != nil {
		r.log.Error().Err(err).Msg("Failed to connect to socket")
		return 1
	}

	defer r.client.Close()

	if err := r.client.Subscribe(services); err != nil {
		r.log.Error().Err(err).Msg("Failed to subscribe to services")
		return 1
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if err := r.client.Stream(ctx, output); err != nil {
		r.log.Error().Err(err).Msg("Failed to stream logs")
		return 1
	}

	return 0
}
