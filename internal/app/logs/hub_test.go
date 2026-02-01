package logs

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"fuku/internal/config"
)

func Test_NewHub(t *testing.T) {
	bufferSize := 100

	h := NewHub(bufferSize)
	assert.NotNil(t, h)

	impl, ok := h.(*hub)
	assert.True(t, ok)
	assert.Equal(t, bufferSize, impl.bufferSize)
	assert.NotNil(t, impl.clients)
	assert.NotNil(t, impl.register)
	assert.NotNil(t, impl.unregister)
	assert.NotNil(t, impl.broadcast)
	assert.NotNil(t, impl.done)
}

func Test_NewClientConn(t *testing.T) {
	bufferSize := 100

	c := NewClientConn("test-client", bufferSize)
	assert.NotNil(t, c)
	assert.Equal(t, "test-client", c.ID)
	assert.NotNil(t, c.Services)
	assert.Empty(t, c.Services)
	assert.NotNil(t, c.SendChan)
	assert.Equal(t, bufferSize, cap(c.SendChan))
}

func Test_SetSubscription(t *testing.T) {
	cfg := config.DefaultConfig()

	tests := []struct {
		name            string
		existingService string
		input           []string
		expectedLen     int
		checkServices   map[string]bool
	}{
		{
			name:          "Sets services",
			input:         []string{"api", "db"},
			expectedLen:   2,
			checkServices: map[string]bool{"api": true, "db": true},
		},
		{
			name:            "Empty services",
			existingService: "old",
			input:           nil,
			expectedLen:     0,
			checkServices:   map[string]bool{},
		},
		{
			name:            "Replaces existing",
			existingService: "old",
			input:           []string{"new"},
			expectedLen:     1,
			checkServices:   map[string]bool{"new": true, "old": false},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewClientConn("test", cfg.Logs.Buffer)
			if tt.existingService != "" {
				c.Services[tt.existingService] = true
			}

			c.SetSubscription(tt.input)
			assert.Len(t, c.Services, tt.expectedLen)

			for svc, expected := range tt.checkServices {
				assert.Equal(t, expected, c.Services[svc])
			}
		})
	}
}

func Test_ShouldReceive(t *testing.T) {
	cfg := config.DefaultConfig()

	tests := []struct {
		name         string
		subscription []string
		service      string
		expected     bool
	}{
		{
			name:         "Empty subscription receives all",
			subscription: nil,
			service:      "api",
			expected:     true,
		},
		{
			name:         "Empty subscription receives any",
			subscription: nil,
			service:      "anything",
			expected:     true,
		},
		{
			name:         "Subscribed service",
			subscription: []string{"api", "db"},
			service:      "api",
			expected:     true,
		},
		{
			name:         "Another subscribed service",
			subscription: []string{"api", "db"},
			service:      "db",
			expected:     true,
		},
		{
			name:         "Not subscribed service",
			subscription: []string{"api", "db"},
			service:      "cache",
			expected:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewClientConn("test", cfg.Logs.Buffer)
			if tt.subscription != nil {
				c.SetSubscription(tt.subscription)
			}

			result := c.ShouldReceive(tt.service)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_Register(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	defer cancel()

	cfg := config.DefaultConfig()
	h := NewHub(cfg.Logs.Buffer)

	go h.Run(ctx)

	client := NewClientConn("test", cfg.Logs.Buffer)

	h.Register(client)

	impl := h.(*hub)

	assert.Eventually(t, func() bool {
		impl.mu.RLock()
		defer impl.mu.RUnlock()

		return impl.clients[client]
	}, 100*time.Millisecond, 5*time.Millisecond)
}

func Test_Register_AfterDone(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	cfg := config.DefaultConfig()
	h := NewHub(cfg.Logs.Buffer)

	done := make(chan struct{})

	go func() {
		h.Run(ctx)
		close(done)
	}()

	cancel()

	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Hub did not stop")
	}

	client := NewClientConn("test", cfg.Logs.Buffer)

	h.Register(client)
}

func Test_Unregister(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	defer cancel()

	cfg := config.DefaultConfig()
	h := NewHub(cfg.Logs.Buffer)

	go h.Run(ctx)

	client := NewClientConn("test", cfg.Logs.Buffer)
	h.Register(client)

	impl := h.(*hub)

	assert.Eventually(t, func() bool {
		impl.mu.RLock()
		defer impl.mu.RUnlock()

		return impl.clients[client]
	}, 100*time.Millisecond, 5*time.Millisecond)

	h.Unregister(client)

	assert.Eventually(t, func() bool {
		impl.mu.RLock()
		defer impl.mu.RUnlock()

		return !impl.clients[client]
	}, 100*time.Millisecond, 5*time.Millisecond)
}

func Test_Unregister_NonExistent(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	defer cancel()

	cfg := config.DefaultConfig()
	h := NewHub(cfg.Logs.Buffer)

	go h.Run(ctx)

	client := NewClientConn("test", cfg.Logs.Buffer)

	h.Unregister(client)
}

func Test_Unregister_AfterDone(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	cfg := config.DefaultConfig()
	h := NewHub(cfg.Logs.Buffer)

	done := make(chan struct{})

	go func() {
		h.Run(ctx)
		close(done)
	}()

	cancel()

	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Hub did not stop")
	}

	client := NewClientConn("test", cfg.Logs.Buffer)

	h.Unregister(client)
}

func Test_Broadcast_ToSubscribedClients(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	defer cancel()

	cfg := config.DefaultConfig()
	h := NewHub(cfg.Logs.Buffer)

	go h.Run(ctx)

	client1 := NewClientConn("client1", cfg.Logs.Buffer)
	client1.SetSubscription([]string{"api"})

	client2 := NewClientConn("client2", cfg.Logs.Buffer)
	client2.SetSubscription([]string{"db"})

	h.Register(client1)
	h.Register(client2)

	impl := h.(*hub)

	assert.Eventually(t, func() bool {
		impl.mu.RLock()
		defer impl.mu.RUnlock()

		return impl.clients[client1] && impl.clients[client2]
	}, 100*time.Millisecond, 5*time.Millisecond)

	h.Broadcast("api", "test message")

	select {
	case msg := <-client1.SendChan:
		assert.Equal(t, MessageLog, msg.Type)
		assert.Equal(t, "api", msg.Service)
		assert.Equal(t, "test message", msg.Message)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("client1 should receive message")
	}

	select {
	case <-client2.SendChan:
		t.Fatal("client2 should not receive message")
	default:
	}
}

func Test_Broadcast_ToAllWhenNoFilter(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	defer cancel()

	cfg := config.DefaultConfig()
	h := NewHub(cfg.Logs.Buffer)

	go h.Run(ctx)

	client := NewClientConn("client", cfg.Logs.Buffer)
	h.Register(client)

	impl := h.(*hub)

	assert.Eventually(t, func() bool {
		impl.mu.RLock()
		defer impl.mu.RUnlock()

		return impl.clients[client]
	}, 100*time.Millisecond, 5*time.Millisecond)

	h.Broadcast("any-service", "test message")

	select {
	case msg := <-client.SendChan:
		assert.Equal(t, "any-service", msg.Service)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("client should receive message")
	}
}

func Test_Broadcast_DropsWhenBufferFull(t *testing.T) {
	cfg := config.DefaultConfig()
	h := NewHub(cfg.Logs.Buffer).(*hub)

	for i := 0; i < cfg.Logs.Buffer+10; i++ {
		h.Broadcast("api", "message")
	}
}

func Test_Run_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	cfg := config.DefaultConfig()
	h := NewHub(cfg.Logs.Buffer)

	done := make(chan struct{})

	go func() {
		h.Run(ctx)
		close(done)
	}()

	client := NewClientConn("test", cfg.Logs.Buffer)
	h.Register(client)

	impl := h.(*hub)

	assert.Eventually(t, func() bool {
		impl.mu.RLock()
		defer impl.mu.RUnlock()

		return impl.clients[client]
	}, 100*time.Millisecond, 5*time.Millisecond)

	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Hub did not stop")
	}

	_, ok := <-client.SendChan
	assert.False(t, ok, "SendChan should be closed")
}
