package rest

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strconv"

	"fuku/internal/contracts"
	"fuku/internal/model"
)

// Logger is the logging surface the API server writes through
type Logger interface {
	Debug(msg string, args ...any)
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
}

// Server manages the HTTP API server lifecycle
type Server struct {
	options    Options
	registry   Registry
	control    Control
	publisher  contracts.Publisher
	identity   model.Instance
	httpServer *http.Server
	log        Logger
}

// NewServer creates the API server
func NewServer(options Options, registry Registry, control Control, publisher contracts.Publisher, identity model.Instance, log Logger) *Server {
	return &Server{
		options:   options,
		registry:  registry,
		control:   control,
		publisher: publisher,
		identity:  identity,
		log:       log,
	}
}

// Start binds the HTTP server with port retry and serves on its own goroutine (a bind failure is logged, not returned)
func (s *Server) Start(context.Context) error {
	h := &handler{registry: s.registry, control: s.control, identity: s.identity}

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

	token := s.options.Token
	mux.Handle("/api/v1/", authMiddleware(token, authedMux))

	ln, addr := s.listen()
	if ln == nil {
		return nil
	}

	s.httpServer = &http.Server{
		Handler:           telemetryMiddleware(s.publisher, corsMiddleware(mux)),
		ReadHeaderTimeout: readHeaderTimeout,
	}

	s.log.Info("API server listening on " + addr)

	s.publish(contracts.Message{
		Type: contracts.EventAPIStarted,
		Data: contracts.APIStarted{Listen: addr},
	})

	go func() {
		if err := s.httpServer.Serve(ln); err != nil && err != http.ErrServerClosed {
			s.log.Error("API server error", "error", err)
		}
	}()

	return nil
}

// Stop gracefully shuts down the HTTP server
func (s *Server) Stop(ctx context.Context) error {
	if s.httpServer == nil {
		return nil
	}

	s.log.Debug("API server shutting down")

	//nolint:errcheck // best-effort graceful shutdown
	s.httpServer.Shutdown(ctx)

	s.publish(contracts.Message{
		Type: contracts.EventAPIStopped,
		Data: contracts.APIStopped{},
	})

	s.log.Debug("API server stopped")

	return nil
}

// publish announces a listener change and logs a rejected publish
func (s *Server) publish(msg contracts.Message) {
	if err := s.publisher.Publish(msg); err != nil {
		s.log.Error(fmt.Sprintf("Failed to publish %s", msg.Type), "error", err)
	}
}

// listen attempts to bind the configured address with port retry
func (s *Server) listen() (net.Listener, string) {
	address := s.options.Listen

	host, portStr, _ := net.SplitHostPort(address)
	port, _ := strconv.Atoi(portStr)

	for i := range PortRetries {
		addr := net.JoinHostPort(host, strconv.Itoa(port+i))

		ln, err := net.Listen("tcp", addr)
		if err != nil {
			continue
		}

		return ln, ln.Addr().String()
	}

	s.log.Warn(fmt.Sprintf("Failed to bind API server on ports %d-%d, continuing without API", port, port+PortRetries-1))

	return nil, ""
}
