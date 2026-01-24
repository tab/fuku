package logs

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"

	"fuku/internal/app/errors"
	"fuku/internal/config"
)

// Client connects to a running fuku instance and streams logs
type Client interface {
	Connect(socketPath string) error
	Subscribe(services []string) error
	Stream(ctx context.Context, output io.Writer) error
	Close() error
}

type client struct {
	conn      net.Conn
	formatter *LogFormatter
}

// NewClient creates a new tail client with the given formatter
func NewClient(formatter *LogFormatter) Client {
	return &client{
		formatter: formatter,
	}
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

// Stream reads log messages and writes them to output
func (c *client) Stream(ctx context.Context, output io.Writer) error {
	reader := bufio.NewReader(c.conn)

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			line, err := reader.ReadBytes('\n')
			if err != nil {
				if err == io.EOF {
					return nil
				}

				return fmt.Errorf("%w: %w", errors.ErrFailedToReadSocket, err)
			}

			var msg LogMessage
			if err := json.Unmarshal(line, &msg); err != nil {
				continue
			}

			if msg.Type == MessageLog {
				c.formatter.WriteFormatted(output, msg.Service, msg.Message)
			}
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

// FindSocket finds the socket for a running fuku instance in the given directory
func FindSocket(socketDir, profile string) (string, error) {
	if profile != "" {
		socketPath := filepath.Join(socketDir, config.SocketPrefix+profile+config.SocketSuffix)
		if _, err := os.Stat(socketPath); err == nil {
			return socketPath, nil
		}

		return "", fmt.Errorf("%w: '%s'", errors.ErrInstanceNotFound, profile)
	}

	pattern := filepath.Join(socketDir, config.SocketPrefix+"*"+config.SocketSuffix)

	matches, err := filepath.Glob(pattern)
	if err != nil {
		return "", fmt.Errorf("%w: %w", errors.ErrSocketSearchFailed, err)
	}

	if len(matches) == 0 {
		return "", errors.ErrNoInstanceRunning
	}

	if len(matches) > 1 {
		profiles := make([]string, len(matches))
		for i, m := range matches {
			base := filepath.Base(m)
			profiles[i] = strings.TrimSuffix(strings.TrimPrefix(base, config.SocketPrefix), config.SocketSuffix)
		}

		return "", fmt.Errorf("%w, specify with --profile: %v", errors.ErrMultipleInstancesRunning, profiles)
	}

	return matches[0], nil
}
