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

// SubscribeOptions describes what a client asks the relay server to send
type SubscribeOptions struct {
	Services []string
	ReplayOptions
}

// Client connects to a running fuku instance and streams logs
type Client interface {
	Connect(socketPath string) error
	Subscribe(options SubscribeOptions) error
	Stream(ctx context.Context, handler Handler) error
	Close() error
}

// client implements the Client interface
type client struct {
	conn      net.Conn
	requested SubscribeOptions
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

// Subscribe sends a subscription request and remembers the requested bounded-read options
func (c *client) Subscribe(options SubscribeOptions) error {
	req := SubscribeRequest{
		Type:          MessageSubscribe,
		Services:      options.Services,
		ReplayOptions: options.ReplayOptions,
	}

	data, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("%w: %w", errors.ErrFailedToMarshalMessage, err)
	}

	data = append(data, '\n')
	if _, err := c.conn.Write(data); err != nil {
		return fmt.Errorf("%w: %w", errors.ErrFailedToWriteSocket, err)
	}

	c.requested = options

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
	awaitingAck := c.requested.bounded()

	for {
		line, err := reader.ReadBytes('\n')

		if err != nil && ctx.Err() != nil {
			return nil
		}

		if errors.Is(err, io.EOF) && awaitingAck {
			return notAcknowledged()
		}

		if errors.Is(err, io.EOF) {
			return nil
		}

		if err != nil {
			return fmt.Errorf("%w: %w", errors.ErrFailedToReadSocket, err)
		}

		if awaitingAck {
			status, ok := c.acknowledgement(line)
			if !ok {
				return notAcknowledged()
			}

			awaitingAck = false

			handler.HandleStatus(status)

			continue
		}

		dispatch(line, handler)
	}
}

// Close closes the connection
func (c *client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}

	return nil
}

// notAcknowledged returns the compatibility error for a server that did not confirm the request
func notAcknowledged() error {
	return fmt.Errorf("%w, restart the running profile with the current fuku version", errors.ErrBoundedReadNotSupported)
}

// acknowledgement decodes a frame that must be a status echoing the exact requested bounded-read options
func (c *client) acknowledgement(line []byte) (StatusMessage, bool) {
	status, ok := decodeStatus(line)
	if !ok {
		return StatusMessage{}, false
	}

	return status, c.requested.equal(status.ReplayOptions)
}

// dispatch routes one relay frame to the handler and ignores frames it cannot decode
func dispatch(line []byte, handler Handler) {
	var envelope MessageEnvelope
	if err := json.Unmarshal(line, &envelope); err != nil {
		return
	}

	//nolint:exhaustive // only handling known message types
	switch envelope.Type {
	case MessageStatus:
		status, ok := decodeStatus(line)
		if !ok {
			return
		}

		handler.HandleStatus(status)
	case MessageLog:
		var msg LogMessage
		if err := json.Unmarshal(line, &msg); err != nil {
			return
		}

		handler.HandleLog(msg)
	}
}

// decodeStatus decodes a status frame and reports false for any other frame
func decodeStatus(line []byte) (StatusMessage, bool) {
	var status StatusMessage
	if err := json.Unmarshal(line, &status); err != nil {
		return StatusMessage{}, false
	}

	return status, status.Type == MessageStatus
}
