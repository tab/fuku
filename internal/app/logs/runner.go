package logs

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"fuku/internal/config"
)

// Runner handles logs mode operations
type Runner interface {
	Run(profile string, services []string) int
}

type runner struct {
	client Client
}

// NewRunner creates a new logs runner
func NewRunner(client Client) Runner {
	return &runner{client: client}
}

// Run handles the logs command to stream logs from a running instance
func (r *runner) Run(profile string, services []string) int {
	socketPath, err := FindSocket(config.SocketDir, profile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	return r.streamLogs(socketPath, services, os.Stdout)
}

// streamLogs connects to a running fuku instance and streams logs
func (r *runner) streamLogs(socketPath string, services []string, output io.Writer) int {
	if err := r.client.Connect(socketPath); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	defer r.client.Close()

	if err := r.client.Subscribe(services); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if err := r.client.Stream(ctx, output); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	return 0
}
