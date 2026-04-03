package api

import (
	"context"
	"fmt"
	"net"
	"net/http"

	"fuku/internal/app/bus"
	"fuku/internal/app/registry"
	"fuku/internal/config"
	"fuku/internal/config/logger"
)

// Server manages the HTTP API server lifecycle
type Server interface {
	Start(ctx context.Context) error
	Stop() error
}

// server implements the Server interface
type server struct {
	cfg        *config.Config
	bus        bus.Bus
	httpServer *http.Server
	store      registry.Store
	log        logger.Logger
}

// NewServer creates a new API server
func NewServer(cfg *config.Config, store registry.Store, b bus.Bus, log logger.Logger) Server {
	if !cfg.APIEnabled() {
		return &noOpServer{}
	}

	return &server{
		cfg:   cfg,
		store: store,
		bus:   b,
		log:   log.WithComponent("API"),
	}
}

// Start binds the HTTP server and begins serving requests
func (s *server) Start(_ context.Context) error {
	h := &handler{store: s.store, bus: s.bus}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/status", h.handleStatus)
	mux.HandleFunc("GET /api/v1/services", h.handleListServices)
	mux.HandleFunc("GET /api/v1/services/{id}", h.handleGetService)
	mux.HandleFunc("POST /api/v1/services/{id}/start", h.handleStartService)
	mux.HandleFunc("POST /api/v1/services/{id}/stop", h.handleStopService)
	mux.HandleFunc("POST /api/v1/services/{id}/restart", h.handleRestartService)

	ln, err := net.Listen("tcp", s.cfg.API.Listen)
	if err != nil {
		return fmt.Errorf("failed to bind API server on %s: %w", s.cfg.API.Listen, err)
	}

	s.httpServer = &http.Server{
		Handler:           corsMiddleware(authMiddleware(s.cfg.API.Auth.Token, mux)),
		ReadHeaderTimeout: config.APIReadHeaderTimeout,
	}

	s.log.Info().Msgf("API server listening on %s", ln.Addr().String())

	go func() {
		if err := s.httpServer.Serve(ln); err != nil && err != http.ErrServerClosed {
			s.log.Error().Err(err).Msg("API server error")
		}
	}()

	return nil
}

// Stop gracefully shuts down the HTTP server
func (s *server) Stop() error {
	if s.httpServer == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), config.APIShutdownTimeout)
	defer cancel()

	s.log.Info().Msg("API server shutting down")

	return s.httpServer.Shutdown(ctx)
}

// noOpServer implements Server with no-op methods
type noOpServer struct{}

func (n *noOpServer) Start(_ context.Context) error { return nil }
func (n *noOpServer) Stop() error                   { return nil }
