package services

import (
	"context"
	"net"
	"os"
)

// TCPService is a TCP server service (fake RPC)
type TCPService struct {
	Service
	Addr string
}

// NewTCP creates a TCP service
func NewTCP(name, addr string) Runner {
	return &TCPService{
		Service: newService(name),
		Addr:    addr,
	}
}

// Run starts the TCP service and waits for shutdown
func (s *TCPService) Run() {
	s.Log.Info().Str("addr", s.Addr).Msg("Starting TCP service")

	ln, err := net.Listen("tcp", s.Addr)
	if err != nil {
		s.Log.Error().Err(err).Msg("Failed to bind")
		os.Exit(1)
	}
	defer ln.Close()

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				select {
				case <-ctx.Done():
					return
				default:
					s.Log.Debug().Err(err).Msg("Accept error")
				}

				continue
			}

			go s.handleConn(conn)
		}
	}()

	s.Log.Info().Msg("Service ready")

	s.WaitForShutdown(ctx)
	cancel()

	s.Log.Info().Msg("Shutting down")
}

func (s *TCPService) handleConn(conn net.Conn) {
	defer conn.Close()

	buf := make([]byte, 1024)

	for {
		n, err := conn.Read(buf)
		if err != nil {
			return
		}

		conn.Write(buf[:n])
	}
}
