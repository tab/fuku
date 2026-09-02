package relay

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"fuku/internal/app/bus"
	"fuku/internal/app/errors"
	"fuku/internal/app/instance"
	"fuku/internal/config"
	"fuku/internal/config/logger"
)

// Broadcaster sends log messages to connected clients
type Broadcaster interface {
	Broadcast(service, message string)
}

// Server manages the Unix socket server for log streaming
type Server struct {
	bus         bus.Bus
	ch          <-chan bus.Message
	cancelSub   context.CancelFunc
	cancel      context.CancelFunc
	socketPath  string
	profile     string
	project     string
	services    []string
	bufferSize  int
	historySize int
	listener    net.Listener
	hub         Hub
	running     atomic.Bool
	wg          sync.WaitGroup
	connID      atomic.Int64
	log         logger.Logger
}

// NewServer creates a new log streaming server
func NewServer(cfg *config.Config, b bus.Bus, identity instance.Identity, log logger.Logger) *Server {
	return &Server{
		bus:         b,
		project:     identity.Fingerprint,
		bufferSize:  cfg.Logs.Buffer,
		historySize: cfg.Logs.History,
		hub:         NewHub(cfg.Logs.Buffer, cfg.Logs.History, log.WithComponent("HUB")),
		log:         log.WithComponent("SERVER"),
	}
}

// SocketPath returns the socket path for this server
func (s *Server) SocketPath() string {
	return s.socketPath
}

// Start begins buffering log output and waits for the profile event, whether or not the socket can be bound later
func (s *Server) Start(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	s.cancel = cancel

	s.Subscribe(ctx)

	s.wg.Go(func() {
		s.hub.Run(ctx)
	})

	s.wg.Go(func() {
		s.Run(ctx)
	})
}

// Subscribe registers the server as a bus subscriber, must be called before Run
func (s *Server) Subscribe(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	s.ch = s.bus.Subscribe(ctx)
	s.cancelSub = cancel
}

// Run waits for the profile resolved event and starts the server
func (s *Server) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			s.cancelSub()

			return
		case msg, ok := <-s.ch:
			if !ok {
				s.cancelSub()

				return
			}

			if msg.Type != bus.EventProfileResolved {
				continue
			}

			data, ok := msg.Data.(bus.ProfileResolved)
			if !ok {
				continue
			}

			s.cancelSub()
			s.activate(ctx, data)

			return
		}
	}
}

// activate starts the server with profile data from the bus event
func (s *Server) activate(ctx context.Context, data bus.ProfileResolved) {
	s.profile = data.Profile

	s.services = make([]string, 0)

	for _, tier := range data.Tiers {
		for _, svc := range tier.Services {
			s.services = append(s.services, svc.Name)
		}
	}

	if err := Cleanup(config.SocketDir); err != nil {
		s.log.Warn().Err(err).Msg("Socket cleanup failed, continuing startup")
	}

	if err := s.start(ctx); err != nil {
		s.log.Warn().Err(err).Msg("Failed to start logs server, continuing without it")
	}
}

// History returns buffered log messages matching the query without following the live stream
// (the buffer is independent of the socket, so a read answers even when the listener never started)
func (s *Server) History(query HistoryQuery) []LogMessage {
	return s.hub.History(query)
}

// Broadcast buffers a log message and sends it to all connected clients
func (s *Server) Broadcast(service, message string) {
	s.hub.Broadcast(service, message)
}

func (s *Server) start(ctx context.Context) error {
	s.socketPath = SocketPathForProfile(config.SocketDir, s.profile)

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

	s.wg.Go(func() {
		s.acceptConnections(ctx)
	})

	return nil
}

// Stop cancels server goroutines, closes the listener, waits for connections to drain, and removes the socket file
func (s *Server) Stop() {
	listening := s.running.Swap(false)

	if s.cancel != nil {
		s.cancel()
	}

	if s.listener != nil {
		s.listener.Close()
	}

	s.wg.Wait()

	if listening {
		if err := os.Remove(s.socketPath); err != nil && !os.IsNotExist(err) {
			s.log.Warn().Err(err).Msgf("Failed to remove socket file: %s", s.socketPath)
		}
	}

	s.log.Debug().Msg("Server stopped")
}

func (s *Server) acceptConnections(ctx context.Context) {
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

func (s *Server) handleConnection(ctx context.Context, conn net.Conn) {
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
	client.Tail = req.Tail
	client.NoFollow = req.NoFollow

	if req.Since > 0 {
		client.Since = time.Now().Add(-req.Since)
	}

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

func (s *Server) writePump(ctx context.Context, conn net.Conn, client *ClientConn) {
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

func (s *Server) hello(conn net.Conn, clientID string) {
	status := StatusMessage{
		Type:     MessageStatus,
		Version:  config.Version,
		Profile:  s.profile,
		Project:  s.project,
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
