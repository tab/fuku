package logs

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"fuku/internal/config"
	"fuku/internal/config/logger"
)

func testLogger() logger.Logger {
	cfg := config.DefaultConfig()
	return logger.NewLoggerWithOutput(cfg, io.Discard)
}

func Test_NewHub(t *testing.T) {
	bufferSize := 100
	historySize := 500
	log := testLogger()

	h := NewHub(bufferSize, historySize, log)
	assert.NotNil(t, h)

	impl, ok := h.(*hub)
	assert.True(t, ok)
	assert.NotNil(t, impl.clients)
	assert.NotNil(t, impl.register)
	assert.NotNil(t, impl.unregister)
	assert.NotNil(t, impl.broadcast)
	assert.NotNil(t, impl.done)
	assert.NotNil(t, impl.history)
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
	cfg := config.DefaultConfig()
	log := testLogger()
	h := NewHub(cfg.Logs.Buffer, cfg.Logs.History, log)

	go h.Run(t.Context())

	client := NewClientConn("test", cfg.Logs.Buffer)
	h.Register(client)

	assert.Eventually(t, func() bool {
		h.Broadcast("test", "ping")

		select {
		case <-client.SendChan:
			return true
		default:
			return false
		}
	}, 100*time.Millisecond, 5*time.Millisecond)
}

func Test_Register_AfterDone(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	cfg := config.DefaultConfig()
	log := testLogger()
	h := NewHub(cfg.Logs.Buffer, cfg.Logs.History, log)

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
	cfg := config.DefaultConfig()
	log := testLogger()
	h := NewHub(cfg.Logs.Buffer, cfg.Logs.History, log)

	go h.Run(t.Context())

	client := NewClientConn("test", cfg.Logs.Buffer)
	h.Register(client)

	assert.Eventually(t, func() bool {
		h.Broadcast("test", "ping")

		select {
		case <-client.SendChan:
			return true
		default:
			return false
		}
	}, 100*time.Millisecond, 5*time.Millisecond)

	h.Unregister(client)

	assert.Eventually(t, func() bool {
		select {
		case _, ok := <-client.SendChan:
			return !ok
		default:
			return false
		}
	}, 100*time.Millisecond, 5*time.Millisecond)
}

func Test_Unregister_NonExistent(t *testing.T) {
	cfg := config.DefaultConfig()
	log := testLogger()
	h := NewHub(cfg.Logs.Buffer, cfg.Logs.History, log)

	go h.Run(t.Context())

	client := NewClientConn("test", cfg.Logs.Buffer)

	h.Unregister(client)
}

func Test_Unregister_AfterDone(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	cfg := config.DefaultConfig()
	log := testLogger()
	h := NewHub(cfg.Logs.Buffer, cfg.Logs.History, log)

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
	cfg := config.DefaultConfig()
	log := testLogger()
	h := NewHub(cfg.Logs.Buffer, cfg.Logs.History, log)

	go h.Run(t.Context())

	client1 := NewClientConn("client1", cfg.Logs.Buffer)
	client1.SetSubscription([]string{"api"})

	client2 := NewClientConn("client2", cfg.Logs.Buffer)
	client2.SetSubscription([]string{"db"})

	h.Register(client1)
	h.Register(client2)

	assert.Eventually(t, func() bool {
		h.Broadcast("api", "test message")

		select {
		case msg := <-client1.SendChan:
			assert.Equal(t, MessageLog, msg.Type)
			assert.Equal(t, "api", msg.Service)
			assert.Equal(t, "test message", msg.Message)

			return true
		default:
			return false
		}
	}, 100*time.Millisecond, 5*time.Millisecond)

	select {
	case <-client2.SendChan:
		t.Fatal("client2 should not receive message")
	default:
	}
}

func Test_Broadcast_ToAllWhenNoFilter(t *testing.T) {
	cfg := config.DefaultConfig()
	log := testLogger()
	h := NewHub(cfg.Logs.Buffer, cfg.Logs.History, log)

	go h.Run(t.Context())

	client := NewClientConn("client", cfg.Logs.Buffer)
	h.Register(client)

	assert.Eventually(t, func() bool {
		h.Broadcast("any-service", "test message")

		select {
		case msg := <-client.SendChan:
			assert.Equal(t, "any-service", msg.Service)
			assert.Equal(t, "test message", msg.Message)

			return true
		default:
			return false
		}
	}, 100*time.Millisecond, 5*time.Millisecond)
}

func Test_Broadcast_DropsWhenBufferFull(t *testing.T) {
	cfg := config.DefaultConfig()
	log := testLogger()
	h := NewHub(cfg.Logs.Buffer, cfg.Logs.History, log).(*hub)

	for range cfg.Logs.Buffer + 10 {
		h.Broadcast("api", "message")
	}

	assert.Equal(t, int64(10), h.dropped.Load())
}

func Test_Run_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	cfg := config.DefaultConfig()
	log := testLogger()
	h := NewHub(cfg.Logs.Buffer, cfg.Logs.History, log)

	done := make(chan struct{})

	go func() {
		h.Run(ctx)
		close(done)
	}()

	client := NewClientConn("test", cfg.Logs.Buffer)
	h.Register(client)

	assert.Eventually(t, func() bool {
		h.Broadcast("test", "ping")

		select {
		case <-client.SendChan:
			return true
		default:
			return false
		}
	}, 100*time.Millisecond, 5*time.Millisecond)

	cancel()

	assert.Eventually(t, func() bool {
		select {
		case _, ok := <-client.SendChan:
			return !ok
		default:
			return false
		}
	}, time.Second, 5*time.Millisecond)
}

func Test_Broadcast_StoresInHistory(t *testing.T) {
	log := testLogger()
	ctx, cancel := context.WithCancel(t.Context())
	h := NewHub(100, 10, log).(*hub)

	done := make(chan struct{})

	go func() {
		h.Run(ctx)
		close(done)
	}()

	sentinel := NewClientConn("sentinel", 100)
	h.Register(sentinel)

	h.Broadcast("api", "msg1")
	h.Broadcast("db", "msg2")
	h.Broadcast("api", "msg3")

	received := 0

	assert.Eventually(t, func() bool {
		select {
		case <-sentinel.SendChan:
			received++

			return received == 3
		default:
			return false
		}
	}, 100*time.Millisecond, 5*time.Millisecond)

	cancel()
	<-done

	var messages []LogMessage

	h.history.forEach(func(msg LogMessage) {
		messages = append(messages, msg)
	})

	assert.Len(t, messages, 3)
	assert.Equal(t, "msg1", messages[0].Message)
	assert.Equal(t, "api", messages[0].Service)
	assert.Equal(t, "msg2", messages[1].Message)
	assert.Equal(t, "msg3", messages[2].Message)
}

func Test_Register_ReplaysHistory(t *testing.T) {
	log := testLogger()
	h := NewHub(100, 100, log)

	go h.Run(t.Context())

	sentinel := NewClientConn("sentinel", 100)
	h.Register(sentinel)

	h.Broadcast("api", "historical-1")
	h.Broadcast("api", "historical-2")

	received := 0

	assert.Eventually(t, func() bool {
		select {
		case <-sentinel.SendChan:
			received++

			return received == 2
		default:
			return false
		}
	}, 100*time.Millisecond, 5*time.Millisecond)

	client := NewClientConn("test", 200)
	h.Register(client)

	var messages []LogMessage

	assert.Eventually(t, func() bool {
		for {
			select {
			case msg := <-client.SendChan:
				messages = append(messages, msg)
			default:
				return len(messages) >= 2
			}
		}
	}, 200*time.Millisecond, 10*time.Millisecond)

	assert.Len(t, messages, 2)
	assert.Equal(t, MessageLog, messages[0].Type)
	assert.Equal(t, "historical-1", messages[0].Message)
	assert.Equal(t, MessageLog, messages[1].Type)
	assert.Equal(t, "historical-2", messages[1].Message)
}

func Test_Register_ReplaysFilteredBySubscription(t *testing.T) {
	log := testLogger()
	h := NewHub(100, 100, log)

	go h.Run(t.Context())

	sentinel := NewClientConn("sentinel", 100)
	h.Register(sentinel)

	h.Broadcast("api", "api-msg")
	h.Broadcast("db", "db-msg")
	h.Broadcast("api", "api-msg-2")

	received := 0

	assert.Eventually(t, func() bool {
		select {
		case <-sentinel.SendChan:
			received++

			return received == 3
		default:
			return false
		}
	}, 100*time.Millisecond, 5*time.Millisecond)

	client := NewClientConn("test", 200)
	client.SetSubscription([]string{"api"})
	h.Register(client)

	var messages []LogMessage

	assert.Eventually(t, func() bool {
		for {
			select {
			case msg := <-client.SendChan:
				messages = append(messages, msg)
			default:
				return len(messages) >= 2
			}
		}
	}, 200*time.Millisecond, 10*time.Millisecond)

	assert.Len(t, messages, 2)
	assert.Equal(t, "api-msg", messages[0].Message)
	assert.Equal(t, "api-msg-2", messages[1].Message)
}

func Test_Register_NoReplayEndWhenNoHistory(t *testing.T) {
	log := testLogger()
	h := NewHub(100, 100, log)

	go h.Run(t.Context())

	client := NewClientConn("test", 200)
	h.Register(client)

	h.Broadcast("api", "live-msg")

	assert.Eventually(t, func() bool {
		select {
		case msg := <-client.SendChan:
			assert.Equal(t, MessageLog, msg.Type)
			assert.Equal(t, "live-msg", msg.Message)

			return true
		default:
			return false
		}
	}, 100*time.Millisecond, 5*time.Millisecond)
}

func Test_Register_NoReplayEndWhenNoMatchingHistory(t *testing.T) {
	log := testLogger()
	h := NewHub(100, 100, log)

	go h.Run(t.Context())

	sentinel := NewClientConn("sentinel", 100)
	h.Register(sentinel)

	h.Broadcast("db", "db-only-msg")

	assert.Eventually(t, func() bool {
		select {
		case <-sentinel.SendChan:
			return true
		default:
			return false
		}
	}, 100*time.Millisecond, 5*time.Millisecond)

	client := NewClientConn("test", 200)
	client.SetSubscription([]string{"api"})
	h.Register(client)

	h.Broadcast("api", "live-msg")

	assert.Eventually(t, func() bool {
		select {
		case msg := <-client.SendChan:
			assert.Equal(t, MessageLog, msg.Type)
			assert.Equal(t, "live-msg", msg.Message)

			return true
		default:
			return false
		}
	}, 100*time.Millisecond, 5*time.Millisecond)
}

func Test_RingBuffer_Push(t *testing.T) {
	rb := newRingBuffer(3)

	rb.push(LogMessage{Service: "a", Message: "1"})
	assert.Equal(t, 1, rb.count)

	rb.push(LogMessage{Service: "b", Message: "2"})
	assert.Equal(t, 2, rb.count)

	rb.push(LogMessage{Service: "c", Message: "3"})
	assert.Equal(t, 3, rb.count)

	rb.push(LogMessage{Service: "d", Message: "4"})
	assert.Equal(t, 3, rb.count)
}

func Test_RingBuffer_ForEach_OldestFirst(t *testing.T) {
	rb := newRingBuffer(3)

	rb.push(LogMessage{Message: "1"})
	rb.push(LogMessage{Message: "2"})
	rb.push(LogMessage{Message: "3"})

	var messages []string

	rb.forEach(func(msg LogMessage) {
		messages = append(messages, msg.Message)
	})

	assert.Equal(t, []string{"1", "2", "3"}, messages)
}

func Test_RingBuffer_ForEach_Wraparound(t *testing.T) {
	rb := newRingBuffer(3)

	rb.push(LogMessage{Message: "1"})
	rb.push(LogMessage{Message: "2"})
	rb.push(LogMessage{Message: "3"})
	rb.push(LogMessage{Message: "4"})
	rb.push(LogMessage{Message: "5"})

	var messages []string

	rb.forEach(func(msg LogMessage) {
		messages = append(messages, msg.Message)
	})

	assert.Equal(t, []string{"3", "4", "5"}, messages)
}

func Test_RingBuffer_ForEach_Empty(t *testing.T) {
	rb := newRingBuffer(3)

	called := false

	rb.forEach(func(msg LogMessage) {
		called = true
	})

	assert.False(t, called)
}

func Test_RingBuffer_ForEach_PartialFill(t *testing.T) {
	rb := newRingBuffer(5)

	rb.push(LogMessage{Message: "1"})
	rb.push(LogMessage{Message: "2"})

	var messages []string

	rb.forEach(func(msg LogMessage) {
		messages = append(messages, msg.Message)
	})

	assert.Equal(t, []string{"1", "2"}, messages)
}
