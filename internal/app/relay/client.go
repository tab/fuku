package relay

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"

	"fuku/internal/app/errors"
)

// Handler processes messages received from the relay server
type Handler interface {
	HandleStatus(StatusMessage)
	HandleLog(LogMessage)
}

// Client connects to a running fuku instance and streams logs
type Client interface {
	Connect(socketPath string) error
	Subscribe(services []string) error
	Stream(ctx context.Context, handler Handler) error
	Close() error
}

// client implements the Client interface
type client struct {
	conn net.Conn
}

// NewClient creates a new relay client
func NewClient() Client {
	return &client{}
}

// Connect connects to the fuku socket
func (c *client) Connect(socketPath string) error {
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		return fmt.Errorf("%w: %w", errors.ErrFailedToConnectSocket, err)
	}

	c.conn = conn

	return nil
}

// Subscribe sends subscription request for the specified services
func (c *client) Subscribe(services []string) error {
	req := SubscribeRequest{
		Type:     MessageSubscribe,
		Services: services,
	}

	data, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("%w: %w", errors.ErrFailedToMarshalMessage, err)
	}

	data = append(data, '\n')
	if _, err := c.conn.Write(data); err != nil {
		return fmt.Errorf("%w: %w", errors.ErrFailedToWriteSocket, err)
	}

	return nil
}

// Stream reads log messages and dispatches them to the handler
func (c *client) Stream(ctx context.Context, handler Handler) error {
	done := make(chan struct{})
	defer close(done)

	go func() {
		select {
		case <-ctx.Done():
			c.conn.Close()
		case <-done:
		}
	}()

	reader := bufio.NewReader(c.conn)

	for {
		line, err := reader.ReadBytes('\n')
		if err != nil && (ctx.Err() != nil || err == io.EOF) {
			return nil
		}

		if err != nil {
			return fmt.Errorf("%w: %w", errors.ErrFailedToReadSocket, err)
		}

		var envelope MessageEnvelope
		if err := json.Unmarshal(line, &envelope); err != nil {
			continue
		}

		//nolint:exhaustive // only handling known message types
		switch envelope.Type {
		case MessageStatus:
			var status StatusMessage
			if err := json.Unmarshal(line, &status); err != nil {
				continue
			}

			handler.HandleStatus(status)
		case MessageLog:
			var msg LogMessage
			if err := json.Unmarshal(line, &msg); err != nil {
				continue
			}

			handler.HandleLog(msg)
		}
	}
}

// Close closes the connection
func (c *client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}

	return nil
}
