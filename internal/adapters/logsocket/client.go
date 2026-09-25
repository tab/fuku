package logsocket

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"

	"fuku/internal/adapters/instance"
	"fuku/internal/app/logs"
	"fuku/internal/contracts"
	"fuku/internal/model"
)

// Client connects to the log socket of the project and streams its frames as typed notifications
type Client struct {
	fingerprint string
	conn        net.Conn
	requested   model.ReplayOptions
}

// NewClient creates the log client of the project the identity names
func NewClient(identity model.Instance) *Client {
	return &Client{fingerprint: identity.Fingerprint}
}

// Connect finds the socket of the running instance and dials it (ErrNoInstanceRunning when there is none)
func (c *Client) Connect() error {
	socketPath, err := findSocket(instance.SocketDir, c.fingerprint)
	if err != nil {
		return err
	}

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		return fmt.Errorf("failed to connect to socket: %w", err)
	}

	c.conn = conn

	return nil
}

// Subscribe sends a subscription request and remembers the requested bounded-read options
func (c *Client) Subscribe(services []string, replay model.ReplayOptions) error {
	req := SubscribeRequest{
		Type:          MessageSubscribe,
		Services:      services,
		ReplayOptions: replay,
	}

	data, _ := json.Marshal(req)
	data = append(data, '\n')

	if _, err := c.conn.Write(data); err != nil {
		return fmt.Errorf("failed to write to socket: %w", err)
	}

	c.requested = replay

	return nil
}

// Stream reads frames and dispatches them to the handler until the connection ends or ctx is cancelled
func (c *Client) Stream(ctx context.Context, handler logs.Handler) error {
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
	awaitingAck := c.requested.Bounded()

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
			return fmt.Errorf("failed to read from socket: %w", err)
		}

		switch {
		case awaitingAck:
			awaitingAck = false
			err = c.acknowledge(line, handler)
		default:
			err = dispatch(line, handler)
		}

		if err != nil {
			return err
		}
	}
}

// Close closes the connection
func (c *Client) Close() error {
	return c.conn.Close()
}

// notAcknowledged returns the compatibility error for a server that did not confirm the request
func notAcknowledged() error {
	return fmt.Errorf("%w, restart the running profile with the current fuku version", contracts.ErrBoundedReadNotSupported)
}

// acknowledge requires a status frame echoing the exact requested bounded-read options and hands it to the handler
func (c *Client) acknowledge(line []byte, handler logs.Handler) error {
	status, ok := decodeStatus(line)
	if !ok || !c.requested.Equal(status.ReplayOptions) {
		return notAcknowledged()
	}

	return handler.HandleStatus(notification(status))
}

// dispatch routes one frame to the handler, ignores frames it cannot decode and returns the handler's status error
func dispatch(line []byte, handler logs.Handler) error {
	//nolint:exhaustive // only handling known message types
	switch messageType(line) {
	case MessageStatus:
		status, ok := decodeStatus(line)
		if !ok {
			return nil
		}

		return handler.HandleStatus(notification(status))
	case MessageLog:
		msg, ok := decodeLog(line)
		if !ok {
			return nil
		}

		handler.HandleLog(model.LogLine{Service: msg.Service, Message: msg.Message})
	}

	return nil
}

// notification maps a status frame to the notification the session receives
func notification(status StatusMessage) contracts.LogStatus {
	return contracts.LogStatus{
		Version:  status.Version,
		Profile:  status.Profile,
		Services: status.Services,
	}
}

// messageType reads the type of a frame and returns an empty type for a frame that is not JSON
func messageType(line []byte) MessageType {
	var envelope MessageEnvelope
	if err := json.Unmarshal(line, &envelope); err != nil {
		return ""
	}

	return envelope.Type
}

// decodeStatus decodes a status frame and reports false for any other frame
func decodeStatus(line []byte) (StatusMessage, bool) {
	var status StatusMessage
	if err := json.Unmarshal(line, &status); err != nil {
		return StatusMessage{}, false
	}

	return status, status.Type == MessageStatus
}

// decodeLog decodes a log frame and reports false for a frame it cannot decode
func decodeLog(line []byte) (LogMessage, bool) {
	var msg LogMessage
	if err := json.Unmarshal(line, &msg); err != nil {
		return LogMessage{}, false
	}

	return msg, true
}
