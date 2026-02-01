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

	"fuku/internal/app/errors"
	"fuku/internal/config"
	"fuku/internal/config/logger"
)

// Server manages the Unix socket server for log streaming
type Server interface {
	Start(ctx context.Context, profile string) error
	Stop() error
	Broadcast(service, message string)
	SocketPath() string
}

// server implements the Server interface
type server struct {
	socketPath string
	bufferSize int
	listener   net.Listener
	hub        Hub
	running    atomic.Bool
	wg         sync.WaitGroup
	connID     atomic.Int64
	cancel     context.CancelFunc
	log        logger.Logger
}

// NewServer creates a new log streaming server
func NewServer(cfg *config.Config, log logger.Logger) Server {
	return &server{
		bufferSize: cfg.Logs.Buffer,
		hub:        NewHub(cfg.Logs.Buffer),
		log:        log.WithComponent("SERVER"),
	}
}

// SocketPath returns the socket path for this server
func (s *server) SocketPath() string {
	return s.socketPath
}

// SocketPathForProfile constructs the socket path for a given profile
func SocketPathForProfile(socketDir, profile string) string {
	return filepath.Join(socketDir, config.SocketPrefix+profile+config.SocketSuffix)
}

// Start starts the Unix socket server
func (s *server) Start(ctx context.Context, profile string) error {
	s.socketPath = SocketPathForProfile(config.SocketDir, profile)

	if err := s.cleanupStaleSocket(); err != nil {
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

	s.wg.Add(1)

	go func() {
		defer s.wg.Done()

		s.hub.Run(serverCtx)
	}()

	s.wg.Add(1)

	go func() {
		defer s.wg.Done()

		s.acceptConnections(serverCtx)
	}()

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

// cleanupStaleSocket removes stale socket file if not in use
func (s *server) cleanupStaleSocket() error {
	if _, err := os.Stat(s.socketPath); os.IsNotExist(err) {
		return nil
	}

	conn, err := net.DialTimeout("unix", s.socketPath, config.SocketDialTimeout)
	if err == nil {
		conn.Close()

		return fmt.Errorf("%w: %s", errors.ErrSocketAlreadyInUse, s.socketPath)
	}

	s.log.Info().Msgf("Removing stale socket: %s", s.socketPath)

	return os.Remove(s.socketPath)
}

// acceptConnections handles incoming client connections
func (s *server) acceptConnections(ctx context.Context) {
	for s.running.Load() {
		conn, err := s.listener.Accept()
		if err != nil {
			if s.running.Load() {
				s.log.Error().Err(err).Msg("Failed to accept connection")
			}

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
	client := NewClientConn(clientID, s.bufferSize)

	s.log.Debug().Msgf("Client connected: %s", clientID)

	reader := bufio.NewReader(conn)

	line, err := reader.ReadBytes('\n')
	if err != nil {
		s.log.Error().Err(err).Msgf("Failed to read from client %s", clientID)
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
	s.hub.Register(client)

	defer s.hub.Unregister(client)

	s.log.Debug().Msgf("Client %s subscribed to services: %v", clientID, req.Services)

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
				s.log.Error().Err(err).Msgf("Failed to marshal message for %s", clientID)
				continue
			}

			data = append(data, '\n')
			if _, err := conn.Write(data); err != nil {
				s.log.Debug().Err(err).Msgf("Client %s disconnected", clientID)
				return
			}
		}
	}
}
