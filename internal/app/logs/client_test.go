package logs

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"fuku/internal/app/errors"
	"fuku/internal/config"
	"fuku/internal/config/logger"
)

func createTestFormatter() *LogFormatter {
	cfg := config.DefaultConfig()
	cfg.Logging.Format = logger.ConsoleFormat

	return NewLogFormatter(cfg)
}

func Test_NewClient(t *testing.T) {
	formatter := createTestFormatter()

	c := NewClient(formatter)
	assert.NotNil(t, c)

	impl, ok := c.(*client)
	assert.True(t, ok)
	assert.NotNil(t, impl.formatter)
	assert.Nil(t, impl.conn)
}

func Test_Connect(t *testing.T) {
	socketPath := filepath.Join("/tmp", "fuku-test-connect.sock")

	listener, err := net.Listen("unix", socketPath)
	assert.NoError(t, err)

	defer listener.Close()
	defer os.Remove(socketPath)

	c := NewClient(createTestFormatter())

	err = c.Connect(socketPath)
	assert.NoError(t, err)

	defer c.Close()

	impl := c.(*client)
	assert.NotNil(t, impl.conn)
}

func Test_Connect_SocketNotFound(t *testing.T) {
	c := NewClient(createTestFormatter())

	err := c.Connect("/nonexistent/path/test.sock")
	assert.Error(t, err)
	assert.ErrorIs(t, err, errors.ErrFailedToConnectSocket)
}

func Test_Subscribe(t *testing.T) {
	tests := []struct {
		name          string
		services      []string
		closeServer   bool
		expectedError error
		checkRequest  func(t *testing.T, data []byte)
	}{
		{
			name:     "Sends subscribe request with services",
			services: []string{"api", "db"},
			checkRequest: func(t *testing.T, data []byte) {
				var req SubscribeRequest

				err := json.Unmarshal(bytes.TrimSuffix(data, []byte("\n")), &req)
				assert.NoError(t, err)
				assert.Equal(t, MessageSubscribe, req.Type)
				assert.Equal(t, []string{"api", "db"}, req.Services)
			},
		},
		{
			name:     "Sends subscribe request with empty services",
			services: nil,
			checkRequest: func(t *testing.T, data []byte) {
				var req SubscribeRequest

				err := json.Unmarshal(bytes.TrimSuffix(data, []byte("\n")), &req)
				assert.NoError(t, err)
				assert.Equal(t, MessageSubscribe, req.Type)
				assert.Nil(t, req.Services)
			},
		},
		{
			name:          "Fails when connection closed",
			services:      []string{"api"},
			closeServer:   true,
			expectedError: errors.ErrFailedToWriteSocket,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			serverConn, clientConn := net.Pipe()

			if tt.closeServer {
				serverConn.Close()
			} else {
				defer serverConn.Close()
			}

			defer clientConn.Close()

			c := NewClient(createTestFormatter()).(*client)
			c.conn = clientConn

			var receivedData []byte

			done := make(chan struct{})

			if !tt.closeServer {
				go func() {
					buf := make([]byte, 1024)
					n, _ := serverConn.Read(buf)
					receivedData = buf[:n]

					close(done)
				}()
			}

			err := c.Subscribe(tt.services)
			if tt.expectedError != nil {
				assert.Error(t, err)
				assert.ErrorIs(t, err, tt.expectedError)
			} else {
				assert.NoError(t, err)
				<-done
				tt.checkRequest(t, receivedData)
			}
		})
	}
}

func Test_Stream_ReceivesLogMessages(t *testing.T) {
	serverConn, clientConn := net.Pipe()

	defer serverConn.Close()
	defer clientConn.Close()

	c := NewClient(createTestFormatter()).(*client)
	c.conn = clientConn

	msg := LogMessage{Type: MessageLog, Service: "api", Message: "Hello World"}
	data, _ := json.Marshal(msg)
	data = append(data, '\n')

	go func() {
		serverConn.Write(data)
		serverConn.Close()
	}()

	var output bytes.Buffer

	err := c.Stream(context.Background(), &output)
	assert.NoError(t, err)
	assert.Contains(t, output.String(), "Hello World")
}

func Test_Stream_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	serverConn, clientConn := net.Pipe()

	defer clientConn.Close()

	c := NewClient(createTestFormatter()).(*client)
	c.conn = clientConn

	done := make(chan error)

	go func() {
		var output bytes.Buffer
		done <- c.Stream(ctx, &output)
	}()

	cancel()
	serverConn.Close()

	select {
	case err := <-done:
		assert.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("Stream did not exit after context cancellation")
	}
}

func Test_Stream_EOFReturnsNil(t *testing.T) {
	serverConn, clientConn := net.Pipe()

	c := NewClient(createTestFormatter()).(*client)
	c.conn = clientConn

	serverConn.Close()

	var output bytes.Buffer

	err := c.Stream(context.Background(), &output)
	assert.NoError(t, err)

	clientConn.Close()
}

func Test_Stream_SkipsInvalidJSON(t *testing.T) {
	serverConn, clientConn := net.Pipe()

	defer clientConn.Close()

	c := NewClient(createTestFormatter()).(*client)
	c.conn = clientConn

	go func() {
		serverConn.Write([]byte("invalid json\n"))

		msg := LogMessage{Type: MessageLog, Service: "api", Message: "Valid"}
		data, _ := json.Marshal(msg)
		data = append(data, '\n')
		serverConn.Write(data)
		serverConn.Close()
	}()

	var output bytes.Buffer

	err := c.Stream(context.Background(), &output)
	assert.NoError(t, err)
	assert.Contains(t, output.String(), "Valid")
}

func Test_Stream_SkipsNonLogMessages(t *testing.T) {
	serverConn, clientConn := net.Pipe()

	defer clientConn.Close()

	c := NewClient(createTestFormatter()).(*client)
	c.conn = clientConn

	go func() {
		msg := LogMessage{Type: MessageSubscribe, Service: "api", Message: "Subscribe"}
		data, _ := json.Marshal(msg)
		data = append(data, '\n')
		serverConn.Write(data)

		msg = LogMessage{Type: MessageLog, Service: "api", Message: "Log"}
		data, _ = json.Marshal(msg)
		data = append(data, '\n')
		serverConn.Write(data)
		serverConn.Close()
	}()

	var output bytes.Buffer

	err := c.Stream(context.Background(), &output)
	assert.NoError(t, err)
	assert.Contains(t, output.String(), "Log")
	assert.NotContains(t, output.String(), "Subscribe")
}

func Test_Close(t *testing.T) {
	tests := []struct {
		name      string
		setupConn bool
	}{
		{
			name:      "Closes connection",
			setupConn: true,
		},
		{
			name:      "Nil connection",
			setupConn: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewClient(createTestFormatter()).(*client)

			if tt.setupConn {
				serverConn, clientConn := net.Pipe()

				defer serverConn.Close()

				c.conn = clientConn
			}

			err := c.Close()
			assert.NoError(t, err)
		})
	}
}

func Test_FindSocket(t *testing.T) {
	tests := []struct {
		name          string
		profile       string
		before        func(tmpDir string) string
		expectedError error
	}{
		{
			name:    "Finds socket by profile",
			profile: "test",
			before: func(tmpDir string) string {
				socketPath := SocketPathForProfile(tmpDir, "test")
				f, _ := os.Create(socketPath)
				f.Close()

				return socketPath
			},
		},
		{
			name:          "Profile not found",
			profile:       "nonexistent",
			before:        func(tmpDir string) string { return "" },
			expectedError: errors.ErrInstanceNotFound,
		},
		{
			name:    "No profile - finds single socket",
			profile: "",
			before: func(tmpDir string) string {
				socketPath := SocketPathForProfile(tmpDir, "default")
				f, _ := os.Create(socketPath)
				f.Close()

				return socketPath
			},
		},
		{
			name:          "No profile - no sockets",
			profile:       "",
			before:        func(tmpDir string) string { return "" },
			expectedError: errors.ErrNoInstanceRunning,
		},
		{
			name:    "No profile - multiple sockets",
			profile: "",
			before: func(tmpDir string) string {
				for _, profile := range []string{"default", "dev"} {
					socketPath := SocketPathForProfile(tmpDir, profile)
					f, _ := os.Create(socketPath)
					f.Close()
				}

				return ""
			},
			expectedError: errors.ErrMultipleInstancesRunning,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			expectedPath := tt.before(tmpDir)

			found, err := FindSocket(tmpDir, tt.profile)

			if tt.expectedError != nil {
				assert.Error(t, err)
				assert.ErrorIs(t, err, tt.expectedError)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, expectedPath, found)
			}
		})
	}
}
