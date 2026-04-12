package api

import (
	"context"
	"net"
	"net/http"
	"strconv"
	"sync/atomic"

	"fuku/internal/app/bus"
	"fuku/internal/app/registry"
	"fuku/internal/config"
	"fuku/internal/config/logger"
)

// Listener exposes the bound API server address
type Listener interface {
	Address() string
}

// Server manages the HTTP API server lifecycle
type Server struct {
	cfg        *config.Config
	bus        bus.Bus
	store      registry.Store
	httpServer *http.Server
	address    atomic.Value
	log        logger.Logger
}

// Address returns the actual bound address, or empty if not started
func (s *Server) Address() string {
	v, _ := s.address.Load().(string)

	return v
}

// NewServer creates a new API server
func NewServer(cfg *config.Config, store registry.Store, b bus.Bus, log logger.Logger) *Server {
	return &Server{
		cfg:   cfg,
		store: store,
		bus:   b,
		log:   log.WithComponent("API"),
	}
}

// Start binds the HTTP server immediately with port retry
func (s *Server) Start() {
	h := &handler{store: s.store, bus: s.bus}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/live", h.handleLive)
	mux.HandleFunc("GET /api/v1/ready", h.handleReady)

	authedMux := http.NewServeMux()
	authedMux.HandleFunc("GET /api/v1/status", h.handleStatus)
	authedMux.HandleFunc("GET /api/v1/services", h.handleListServices)
	authedMux.HandleFunc("GET /api/v1/services/{id}", h.handleGetService)
	authedMux.HandleFunc("POST /api/v1/services/{id}/start", h.handleStartService)
	authedMux.HandleFunc("POST /api/v1/services/{id}/stop", h.handleStopService)
	authedMux.HandleFunc("POST /api/v1/services/{id}/restart", h.handleRestartService)

	token := s.cfg.ServerToken()
	mux.Handle("/api/v1/", authMiddleware(token, authedMux))

	ln, addr := s.listen()
	if ln == nil {
		return
	}

	s.httpServer = &http.Server{
		Handler:           telemetryMiddleware(s.bus, corsMiddleware(mux)),
		ReadHeaderTimeout: config.APIReadHeaderTimeout,
	}

	s.address.Store(addr)
	s.log.Info().Msgf("API server listening on %s", addr)

	s.bus.Publish(bus.Message{
		Type:     bus.EventAPIStarted,
		Data:     bus.APIStarted{Listen: addr},
		Critical: true,
	})

	go func() {
		if err := s.httpServer.Serve(ln); err != nil && err != http.ErrServerClosed {
			s.log.Error().Err(err).Msg("API server error")
		}
	}()
}

// Shutdown gracefully shuts down the HTTP server
func (s *Server) Shutdown(ctx context.Context) {
	if s.httpServer == nil {
		return
	}

	s.log.Info().Msg("API server shutting down")

	//nolint:errcheck // best-effort graceful shutdown
	s.httpServer.Shutdown(ctx)

	s.bus.Publish(bus.Message{
		Type:     bus.EventAPIStopped,
		Data:     bus.APIStopped{},
		Critical: true,
	})

	s.log.Info().Msg("API server stopped")
}

// listen attempts to bind the configured address with port retry
func (s *Server) listen() (net.Listener, string) {
	address := s.cfg.ServerListen()

	host, portStr, err := net.SplitHostPort(address)
	if err != nil {
		s.log.Warn().Err(err).Msgf("Invalid API listen address: %s", address)

		return nil, ""
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		s.log.Warn().Err(err).Msgf("Invalid API port: %s", portStr)

		return nil, ""
	}

	for i := range config.APIPortRetries {
		addr := net.JoinHostPort(host, strconv.Itoa(port+i))

		ln, err := net.Listen("tcp", addr)
		if err != nil {
			continue
		}

		return ln, ln.Addr().String()
	}

	s.log.Warn().Msgf("Failed to bind API server on ports %d-%d, continuing without API", port, port+config.APIPortRetries-1)

	return nil, ""
}
