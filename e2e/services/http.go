package services

import (
	"context"
	"net"
	"net/http"
	"os"
	"time"
)

// HTTPService is an HTTP server service
type HTTPService struct {
	Service
	Addr string
}

// NewHTTP creates an HTTP service
func NewHTTP(name, addr string) Runner {
	return &HTTPService{
		Service: newService(name),
		Addr:    addr,
	}
}

// Run starts the HTTP service and waits for shutdown
func (s *HTTPService) Run() {
	s.Log.Info().Str("addr", s.Addr).Msg("Starting HTTP service")

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})

	server := &http.Server{
		Addr:              s.Addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	ln, err := net.Listen("tcp", s.Addr)
	if err != nil {
		s.Log.Error().Err(err).Msg("Failed to bind")
		os.Exit(1)
	}

	go func() {
		if err := server.Serve(ln); err != nil && err != http.ErrServerClosed {
			s.Log.Error().Err(err).Msg("Server error")
		}
	}()

	s.Log.Info().Msg("Service ready")
	s.WaitForShutdown(context.Background())
	s.Log.Info().Msg("Shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		s.Log.Error().Err(err).Msg("HTTP server shutdown error")
	}
}
