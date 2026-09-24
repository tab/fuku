package logsocket

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"time"

	"fuku/internal/model"
	"fuku/internal/platform/buildinfo"
)

// writeTimeout bounds one frame write to a client
const writeTimeout = 5 * time.Second

func (s *Server) handleConnection(ctx context.Context, conn net.Conn) {
	defer conn.Close()

	clientID := fmt.Sprintf("client-%d", s.connID.Add(1))

	s.log.Debug("Client connected: " + clientID)

	reader := bufio.NewReader(conn)

	line, err := reader.ReadBytes('\n')
	if err != nil {
		s.log.Debug(fmt.Sprintf("Client %s disconnected before subscribing", clientID), "error", err)

		return
	}

	var req SubscribeRequest
	if err := json.Unmarshal(line, &req); err != nil {
		s.log.Error("Failed to parse subscribe request from "+clientID, "error", err)

		return
	}

	if req.Type != MessageSubscribe {
		s.log.Error(fmt.Sprintf("Expected subscribe message from %s, got %s", clientID, req.Type))

		return
	}

	if req.Tail != nil && *req.Tail <= 0 {
		s.log.Error(fmt.Sprintf("Rejected subscribe request from %s: tail must be greater than zero", clientID))

		return
	}

	s.log.Debug(fmt.Sprintf("Client %s subscribed to services: %v", clientID, req.Services))

	s.hello(conn, clientID, req.ReplayOptions)

	sub := s.hub.Subscribe(req.Services, req.ReplayOptions)
	defer s.hub.Unsubscribe(sub)

	s.writePump(ctx, conn, clientID, sub.Lines())
}

func (s *Server) writePump(ctx context.Context, conn net.Conn, clientID string, lines <-chan model.LogLine) {
	for {
		select {
		case <-ctx.Done():
			return
		case line, ok := <-lines:
			if !ok {
				return
			}

			data, _ := json.Marshal(LogMessage{Type: MessageLog, Service: line.Service, Message: line.Message})
			data = append(data, '\n')

			if err := conn.SetWriteDeadline(time.Now().Add(writeTimeout)); err != nil {
				s.log.Debug(fmt.Sprintf("Client %s disconnected", clientID), "error", err)

				return
			}

			if _, err := conn.Write(data); err != nil {
				s.log.Debug(fmt.Sprintf("Client %s disconnected", clientID), "error", err)

				return
			}
		}
	}
}

func (s *Server) hello(conn net.Conn, clientID string, replay model.ReplayOptions) {
	status := StatusMessage{
		Type:          MessageStatus,
		Version:       buildinfo.Version,
		Instance:      s.instanceID,
		Fingerprint:   s.fingerprint,
		Profile:       s.profile,
		Services:      s.services,
		ReplayOptions: replay,
	}

	data, _ := json.Marshal(status)
	data = append(data, '\n')

	if err := conn.SetWriteDeadline(time.Now().Add(writeTimeout)); err != nil {
		s.log.Debug("Failed to set write deadline for "+clientID, "error", err)

		return
	}

	if _, err := conn.Write(data); err != nil {
		s.log.Debug("Failed to send status to "+clientID, "error", err)
	}
}
