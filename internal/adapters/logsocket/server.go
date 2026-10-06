package logsocket

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
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

// The private directory beside the project socket and the socket name inside it, where start binds before the rename
const (
	privateDirSuffix  = ".d"
	privateSocketName = "s"
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

// Control stops every service of the run, which ends the instance
type Control interface {
	StopAll() error
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
	control     Control
	instanceID  string
	fingerprint string
	halt        context.CancelFunc
	done        chan struct{}
	bound       chan struct{}
	bindErr     error
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
func NewServer(hub Hub, registry Registry, control Control, identity model.Instance, log Logger) *Server {
	return &Server{
		hub:         hub,
		registry:    registry,
		control:     control,
		instanceID:  identity.ID,
		fingerprint: identity.Fingerprint,
		done:        make(chan struct{}),
		bound:       make(chan struct{}),
		socketPath:  instance.SocketPath(instance.SocketDir, identity.Fingerprint),
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
	s.halt()

	var err error

	select {
	case <-s.done:
	case <-ctx.Done():
		err = ctx.Err()
	}

	s.close()

	return err
}

// Bound waits for the bind attempt and returns its failure, or the error of ctx when it ends first
func (s *Server) Bound(ctx context.Context) error {
	select {
	case <-s.bound:
		return s.bindErr
	case <-ctx.Done():
		return ctx.Err()
	}
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

	if err := cleanup(filepath.Dir(s.socketPath), s.fingerprint); err != nil {
		s.log.Warn("Socket cleanup failed, continuing startup", "error", err)
	}

	err := s.start(ctx)
	if err != nil {
		s.log.Warn("Failed to start logs server, continuing without it", "error", err)
	}

	s.bindErr = err
	close(s.bound)
}

func (s *Server) start(ctx context.Context) error {
	if instance.ProbeSocket(s.socketPath) == nil {
		return fmt.Errorf("socket is already in use: %s", s.socketPath)
	}

	if err := os.Remove(s.socketPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to cleanup stale socket: %w", err)
	}

	listener, err := s.listen()
	if err != nil {
		return err
	}

	s.listener = listener
	s.running.Store(true)
	s.log.Info("Server listening on " + s.socketPath)

	s.wg.Go(func() {
		s.acceptConnections(ctx)
	})

	return nil
}

// listen binds the restricted socket in a private directory and renames it into place, so no other user reaches it
func (s *Server) listen() (*net.UnixListener, error) {
	private := s.socketPath + privateDirSuffix

	listener, bindPath, err := bind(private)
	defer os.RemoveAll(private)

	if err != nil {
		return nil, err
	}

	if err := os.Rename(bindPath, s.socketPath); err != nil {
		listener.Close()

		return nil, fmt.Errorf("failed to move socket into place %s: %w", s.socketPath, err)
	}

	return listener, nil
}

// bind replaces the private directory left by a crashed start with a fresh 0700 one and listens on a 0600 socket inside
func bind(private string) (*net.UnixListener, string, error) {
	if err := os.RemoveAll(private); err != nil {
		return nil, "", fmt.Errorf("failed to cleanup private socket directory %s: %w", private, err)
	}

	if err := os.Mkdir(private, 0o700); err != nil {
		return nil, "", fmt.Errorf("failed to create private socket directory %s: %w", private, err)
	}

	bindPath := filepath.Join(private, privateSocketName)

	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: bindPath, Net: "unix"})
	if err != nil {
		return nil, "", fmt.Errorf("failed to listen on socket %s: %w", bindPath, err)
	}

	if err := os.Chmod(bindPath, 0o600); err != nil {
		listener.Close()

		return nil, "", fmt.Errorf("failed to restrict socket %s: %w", bindPath, err)
	}

	return listener, bindPath, nil
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
