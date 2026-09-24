package logsocket

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"fuku/internal/adapters/instance"
	"fuku/internal/app/logs"
	"fuku/internal/model"
)

// The pause after a failed accept: it starts at the minimum and doubles up to the maximum, as net/http does
const (
	acceptDelayMin = 5 * time.Millisecond
	acceptDelayMax = time.Second
)

// Hub hands out the subscriptions the server streams to its clients
type Hub interface {
	Subscribe(services []string, replay model.ReplayOptions) *logs.Subscription
	Unsubscribe(sub *logs.Subscription)
}

// Registry supplies the resolved profile the server announces to its clients
type Registry interface {
	WaitResolved(ctx context.Context)
	Read(fn func(*model.Snapshot))
}

// Logger is the logging surface the server writes through
type Logger interface {
	Debug(msg string, args ...any)
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
}

// Server manages the Unix socket server for log streaming
type Server struct {
	hub         Hub
	registry    Registry
	instanceID  string
	fingerprint string
	halt        context.CancelFunc
	done        chan struct{}
	socketPath  string
	profile     string
	services    []string
	listener    net.Listener
	running     atomic.Bool
	wg          sync.WaitGroup
	connID      atomic.Int64
	log         Logger
}

// NewServer creates a new log streaming server
func NewServer(hub Hub, registry Registry, identity model.Instance, log Logger) *Server {
	return &Server{
		hub:         hub,
		registry:    registry,
		instanceID:  identity.ID,
		fingerprint: identity.Fingerprint,
		done:        make(chan struct{}),
		log:         log,
	}
}

// Start runs the server on its own goroutine until Stop
func (s *Server) Start(ctx context.Context) error {
	ctx, s.halt = context.WithCancel(ctx)

	go func() {
		defer close(s.done)

		s.run(ctx)
	}()

	return nil
}

// Stop ends a pending start, waits for it, and closes the server
func (s *Server) Stop(ctx context.Context) error {
	if s.halt == nil {
		return nil
	}

	s.halt()

	select {
	case <-s.done:
	case <-ctx.Done():
		return ctx.Err()
	}

	s.close()

	return nil
}

// run binds the socket once the profile resolves (a bind failure is logged and the run continues without the server)
func (s *Server) run(ctx context.Context) {
	s.registry.WaitResolved(ctx)

	if ctx.Err() != nil {
		return
	}

	s.registry.Read(func(snapshot *model.Snapshot) {
		s.profile = snapshot.Profile
		s.services = make([]string, 0, len(snapshot.Services))

		for _, tier := range snapshot.Tiers {
			for _, svc := range tier.Services {
				s.services = append(s.services, svc.Name)
			}
		}
	})

	if err := cleanup(instance.SocketDir); err != nil {
		s.log.Warn("Socket cleanup failed, continuing startup", "error", err)
	}

	if err := s.start(ctx); err != nil {
		s.log.Warn("Failed to start logs server, continuing without it", "error", err)
	}
}

func (s *Server) start(ctx context.Context) error {
	s.socketPath = instance.SocketPath(instance.SocketDir, s.fingerprint)

	conn, err := net.DialTimeout("unix", s.socketPath, instance.SocketDialTimeout)
	if err == nil {
		conn.Close()

		return fmt.Errorf("socket is already in use: %s", s.socketPath)
	}

	if err := os.Remove(s.socketPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to cleanup stale socket: %w", err)
	}

	listener, err := net.Listen("unix", s.socketPath)
	if err != nil {
		return fmt.Errorf("failed to listen on socket %s: %w", s.socketPath, err)
	}

	s.listener = listener
	s.running.Store(true)
	s.log.Info("Server listening on " + s.socketPath)

	s.wg.Go(func() {
		s.acceptConnections(ctx)
	})

	return nil
}

// close closes the listener, waits for connections to drain, and removes the socket file
func (s *Server) close() {
	if !s.running.Load() {
		return
	}

	s.running.Store(false)

	if s.listener != nil {
		s.listener.Close()
	}

	s.wg.Wait()

	if err := os.Remove(s.socketPath); err != nil && !os.IsNotExist(err) {
		s.log.Warn("Failed to remove socket file: "+s.socketPath, "error", err)
	}

	s.log.Debug("Server stopped")
}

// acceptConnections serves clients until the listener closes, pausing on a failed accept so full fd tables cannot spin
func (s *Server) acceptConnections(ctx context.Context) {
	var delay time.Duration

	for {
		conn, err := s.listener.Accept()
		if errors.Is(err, net.ErrClosed) {
			return
		}

		if err != nil {
			delay = min(max(2*delay, acceptDelayMin), acceptDelayMax)
			s.log.Error(fmt.Sprintf("Failed to accept connection, retrying in %v", delay), "error", err)

			select {
			case <-ctx.Done():
				return
			case <-time.After(delay):
			}

			continue
		}

		delay = 0

		s.wg.Add(1)

		go func(c net.Conn) {
			defer s.wg.Done()

			// sync: a stop closes the connection, so a client that never subscribes cannot hold the shutdown in the read
			stop := context.AfterFunc(ctx, func() { c.Close() })
			defer stop()

			s.handleConnection(ctx, c)
		}(conn)
	}
}
