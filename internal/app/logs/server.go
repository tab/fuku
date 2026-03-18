package logs

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"fuku/internal/app/errors"
	"fuku/internal/config"
	"fuku/internal/config/logger"
)

// Server manages the Unix socket server for log streaming
type Server interface {
	Start(ctx context.Context, profile string, services []string) error
	Stop() error
	Broadcast(service, message string)
	SocketPath() string
}

// server implements the Server interface
type server struct {
	socketPath  string
	profile     string
	services    []string
	bufferSize  int
	historySize int
	listener    net.Listener
	hub         Hub
	running     atomic.Bool
	wg          sync.WaitGroup
	connID      atomic.Int64
	cancel      context.CancelFunc
	log         logger.Logger
}

// NewServer creates a new log streaming server
func NewServer(cfg *config.Config, log logger.Logger) Server {
	return &server{
		bufferSize:  cfg.Logs.Buffer,
		historySize: cfg.Logs.History,
		hub:         NewHub(cfg.Logs.Buffer, cfg.Logs.History, log.WithComponent("HUB")),
		log:         log.WithComponent("SERVER"),
	}
}

// SocketPath returns the socket path for this server
func (s *server) SocketPath() string {
	return s.socketPath
}

// SocketPathForProfile constructs the socket path for a given profile
func SocketPathForProfile(socketDir, profile string) string {
	return filepath.Join(socketDir, fmt.Sprintf("%s%s%s", config.SocketPrefix, profile, config.SocketSuffix))
}

// Cleanup removes all stale fuku socket files from the given directory
func Cleanup(socketDir string) error {
	pattern := SocketPathForProfile(socketDir, "*")

	matches, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Errorf("failed to glob for stale sockets: %w", err)
	}

	var failed []string

	for _, socketPath := range matches {
		info, err := os.Lstat(socketPath)
		if err != nil || info.Mode()&os.ModeSocket == 0 {
			continue
		}

		conn, err := net.DialTimeout("unix", socketPath, config.SocketDialTimeout)
		if err == nil {
			conn.Close()
			continue
		}

		if err := os.Remove(socketPath); err != nil {
			failed = append(failed, filepath.Base(socketPath))
		}
	}

	if len(failed) > 0 {
		return fmt.Errorf("%w: %v", errors.ErrFailedToCleanupSocket, failed)
	}

	return nil
}

// Start starts the Unix socket server
func (s *server) Start(ctx context.Context, profile string, services []string) error {
	s.profile = profile
	s.services = services
	s.socketPath = SocketPathForProfile(config.SocketDir, profile)

	conn, err := net.DialTimeout("unix", s.socketPath, config.SocketDialTimeout)
	if err == nil {
		conn.Close()
		return fmt.Errorf("%w: %s", errors.ErrSocketAlreadyInUse, s.socketPath)
	}

	if err := os.Remove(s.socketPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("%w: %w", errors.ErrFailedToCleanupSocket, err)
	}

	listener, err := net.Listen("unix", s.socketPath)
	if err != nil {
		return fmt.Errorf("%w %s: %w", errors.ErrFailedToListenSocket, s.socketPath, err)
	}

	s.listener = listener
	s.running.Store(true)
	s.log.Info().Msgf("Server listening on %s", s.socketPath)

	serverCtx, cancel := context.WithCancel(ctx)
	s.cancel = cancel

	s.wg.Go(func() {
		s.hub.Run(serverCtx)
	})

	s.wg.Go(func() {
		s.acceptConnections(serverCtx)
	})

	return nil
}

// Stop stops the server and cleans up resources
func (s *server) Stop() error {
	if !s.running.Load() {
		return nil
	}

	s.running.Store(false)

	if s.cancel != nil {
		s.cancel()
	}

	if s.listener != nil {
		s.listener.Close()
	}

	s.wg.Wait()

	if err := os.Remove(s.socketPath); err != nil && !os.IsNotExist(err) {
		s.log.Warn().Err(err).Msgf("Failed to remove socket file: %s", s.socketPath)
	}

	s.log.Info().Msg("Server stopped")

	return nil
}

// Broadcast sends a log message to all connected clients
func (s *server) Broadcast(service, message string) {
	if s.running.Load() {
		s.hub.Broadcast(service, message)
	}
}

// acceptConnections handles incoming client connections
func (s *server) acceptConnections(ctx context.Context) {
	for s.running.Load() {
		conn, err := s.listener.Accept()
		if err != nil && s.running.Load() {
			s.log.Error().Err(err).Msg("Failed to accept connection")
		}

		if err != nil {
			continue
		}

		s.wg.Add(1)

		go func(c net.Conn) {
			defer s.wg.Done()

			s.handleConnection(ctx, c)
		}(conn)
	}
}

// handleConnection processes a single client connection
func (s *server) handleConnection(ctx context.Context, conn net.Conn) {
	defer conn.Close()

	connID := s.connID.Add(1)
	clientID := fmt.Sprintf("client-%d", connID)
	client := NewClientConn(clientID, s.bufferSize+s.historySize)

	s.log.Debug().Msgf("Client connected: %s", clientID)

	reader := bufio.NewReader(conn)

	line, err := reader.ReadBytes('\n')
	if err != nil {
		s.log.Debug().Err(err).Msgf("Client %s disconnected before subscribing", clientID)
		return
	}

	var req SubscribeRequest
	if err := json.Unmarshal(line, &req); err != nil {
		s.log.Error().Err(err).Msgf("Failed to parse subscribe request from %s", clientID)
		return
	}

	if req.Type != MessageSubscribe {
		s.log.Error().Msgf("Expected subscribe message from %s, got %s", clientID, req.Type)
		return
	}

	client.SetSubscription(req.Services)

	s.log.Debug().Msgf("Client %s subscribed to services: %v", clientID, req.Services)

	s.hello(conn, clientID)

	done := make(chan struct{})

	go func() {
		defer close(done)

		s.writePump(ctx, conn, client)
	}()

	s.hub.Register(client)
	defer s.hub.Unregister(client)

	<-done
}

// writePump reads messages from SendChan and writes them to the connection
func (s *server) writePump(ctx context.Context, conn net.Conn, client *ClientConn) {
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-client.SendChan:
			if !ok {
				return
			}

			data, err := json.Marshal(msg)
			if err != nil {
				s.log.Error().Err(err).Msgf("Failed to marshal message for %s", client.ID)
				continue
			}

			data = append(data, '\n')

			if err := conn.SetWriteDeadline(time.Now().Add(config.SocketWriteTimeout)); err != nil {
				s.log.Debug().Err(err).Msgf("Client %s disconnected", client.ID)
				return
			}

			if _, err := conn.Write(data); err != nil {
				s.log.Debug().Err(err).Msgf("Client %s disconnected", client.ID)
				return
			}
		}
	}
}

// hello sends a status message to a newly connected client
func (s *server) hello(conn net.Conn, clientID string) {
	status := StatusMessage{
		Type:     MessageStatus,
		Version:  config.Version,
		Profile:  s.profile,
		Services: s.services,
	}

	data, err := json.Marshal(status)
	if err != nil {
		s.log.Error().Err(err).Msgf("Failed to marshal status for %s", clientID)
		return
	}

	data = append(data, '\n')

	if err := conn.SetWriteDeadline(time.Now().Add(config.SocketWriteTimeout)); err != nil {
		s.log.Debug().Err(err).Msgf("Failed to set write deadline for %s", clientID)
		return
	}

	if _, err := conn.Write(data); err != nil {
		s.log.Debug().Err(err).Msgf("Failed to send status to %s", clientID)
	}
}
