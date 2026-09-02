package relay

import (
	"context"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fuku/internal/config"
	"fuku/internal/config/logger"
)

func testLogger() logger.Logger {
	cfg := config.DefaultConfig()

	return logger.NewLoggerWithOutput(cfg, io.Discard)
}

func Test_NewHub(t *testing.T) {
	h := NewHub(10, 50, testLogger())

	assert.NotNil(t, h)
}

func Test_NewClientConn(t *testing.T) {
	conn := NewClientConn("client-1", 10)

	assert.Equal(t, "client-1", conn.ID)
	assert.NotNil(t, conn.Services)
	assert.Empty(t, conn.Services)
	assert.NotNil(t, conn.SendChan)
	assert.Equal(t, 10, cap(conn.SendChan))
}

func Test_ClientConn_SetSubscription(t *testing.T) {
	tests := []struct {
		name     string
		services []string
		expected map[string]bool
	}{
		{
			name:     "multiple services",
			services: []string{"api", "web"},
			expected: map[string]bool{"api": true, "web": true},
		},
		{
			name:     "empty services",
			services: []string{},
			expected: map[string]bool{},
		},
		{
			name:     "single service",
			services: []string{"api"},
			expected: map[string]bool{"api": true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conn := NewClientConn("client-1", 10)
			conn.SetSubscription(tt.services)
			assert.Equal(t, tt.expected, conn.Services)
		})
	}
}

func Test_ClientConn_SetSubscription_Replaces(t *testing.T) {
	conn := NewClientConn("client-1", 10)

	conn.SetSubscription([]string{"api", "web"})
	assert.Len(t, conn.Services, 2)

	conn.SetSubscription([]string{"db"})
	assert.Len(t, conn.Services, 1)
	assert.True(t, conn.Services["db"])
	assert.False(t, conn.Services["api"])
}

func Test_ClientConn_ShouldReceive(t *testing.T) {
	tests := []struct {
		name     string
		services []string
		check    string
		expected bool
	}{
		{
			name:     "empty subscription receives all",
			services: []string{},
			check:    "api",
			expected: true,
		},
		{
			name:     "subscribed service",
			services: []string{"api", "web"},
			check:    "api",
			expected: true,
		},
		{
			name:     "unsubscribed service",
			services: []string{"api", "web"},
			check:    "db",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conn := NewClientConn("client-1", 10)
			conn.SetSubscription(tt.services)
			assert.Equal(t, tt.expected, conn.ShouldReceive(tt.check))
		})
	}
}

func Test_Hub_Register(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	h := NewHub(10, 50, testLogger())
	go h.Run(ctx)

	conn := NewClientConn("client-1", 10)
	h.Register(conn)

	h.Broadcast("api", "hello")

	select {
	case msg := <-conn.SendChan:
		assert.Equal(t, MessageLog, msg.Type)
		assert.Equal(t, "api", msg.Service)
		assert.Equal(t, "hello", msg.Message)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Expected message on registered client")
	}
}

func Test_Hub_Unregister(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	h := NewHub(10, 50, testLogger())
	go h.Run(ctx)

	conn := NewClientConn("client-1", 10)
	h.Register(conn)

	//nolint:forbidigo // allow hub goroutine to process
	time.Sleep(10 * time.Millisecond)

	h.Unregister(conn)

	select {
	case _, ok := <-conn.SendChan:
		assert.False(t, ok, "Channel should be closed after unregister")
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Channel was not closed after unregister")
	}
}

func Test_Hub_Broadcast_ToSubscribedClient(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	h := NewHub(10, 50, testLogger())
	go h.Run(ctx)

	conn := NewClientConn("client-1", 10)
	conn.SetSubscription([]string{"api"})
	h.Register(conn)

	//nolint:forbidigo // allow hub goroutine to process
	time.Sleep(10 * time.Millisecond)

	h.Broadcast("api", "subscribed message")
	h.Broadcast("web", "filtered message")

	select {
	case msg := <-conn.SendChan:
		assert.Equal(t, "api", msg.Service)
		assert.Equal(t, "subscribed message", msg.Message)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Expected message for subscribed service")
	}

	select {
	case msg := <-conn.SendChan:
		t.Fatalf("Should not receive message for unsubscribed service, got: %v", msg)
	case <-time.After(50 * time.Millisecond):
	}
}

func Test_Hub_Broadcast_ToAllClients(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	h := NewHub(10, 50, testLogger())
	go h.Run(ctx)

	conn := NewClientConn("client-1", 10)
	h.Register(conn)

	//nolint:forbidigo // allow hub goroutine to process
	time.Sleep(10 * time.Millisecond)

	h.Broadcast("api", "message for all")

	select {
	case msg := <-conn.SendChan:
		assert.Equal(t, "api", msg.Service)
		assert.Equal(t, "message for all", msg.Message)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Expected message on unfiltered client")
	}
}

func Test_Hub_Broadcast_BufferFull(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	h := NewHub(1, 50, testLogger())
	go h.Run(ctx)

	conn := NewClientConn("client-1", 1)
	h.Register(conn)

	//nolint:forbidigo // allow hub goroutine to process
	time.Sleep(10 * time.Millisecond)

	for i := range 5 {
		h.Broadcast("api", fmt.Sprintf("msg-%d", i))
	}

	//nolint:forbidigo // allow hub goroutine to process
	time.Sleep(50 * time.Millisecond)

	received := 0

	for {
		select {
		case <-conn.SendChan:
			received++
		default:
			assert.GreaterOrEqual(t, received, 1)
			return
		}
	}
}

func Test_Hub_Run_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())

	h := NewHub(10, 50, testLogger())

	done := make(chan struct{})

	go func() {
		h.Run(ctx)
		close(done)
	}()

	conn := NewClientConn("client-1", 10)
	h.Register(conn)

	//nolint:forbidigo // allow hub goroutine to process
	time.Sleep(10 * time.Millisecond)

	cancel()

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("Hub did not stop after context cancellation")
	}

	select {
	case _, ok := <-conn.SendChan:
		assert.False(t, ok, "Client channels should be closed on shutdown")
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Client channel was not closed on shutdown")
	}
}

func Test_ringBuffer_Push_PartialFill(t *testing.T) {
	rb := newRingBuffer(5)

	rb.push(LogMessage{Service: "a", Message: "1"})
	rb.push(LogMessage{Service: "b", Message: "2"})

	assert.Equal(t, 2, rb.count)

	var messages []LogMessage

	rb.forEach(func(msg LogMessage) {
		messages = append(messages, msg)
	})

	require.Len(t, messages, 2)
	assert.Equal(t, "a", messages[0].Service)
	assert.Equal(t, "b", messages[1].Service)
}

func Test_ringBuffer_Push_Wraparound(t *testing.T) {
	rb := newRingBuffer(3)

	rb.push(LogMessage{Service: "a", Message: "1"})
	rb.push(LogMessage{Service: "b", Message: "2"})
	rb.push(LogMessage{Service: "c", Message: "3"})
	rb.push(LogMessage{Service: "d", Message: "4"})

	assert.Equal(t, 3, rb.count)

	var messages []LogMessage

	rb.forEach(func(msg LogMessage) {
		messages = append(messages, msg)
	})

	require.Len(t, messages, 3)
	assert.Equal(t, "b", messages[0].Service)
	assert.Equal(t, "c", messages[1].Service)
	assert.Equal(t, "d", messages[2].Service)
}

func Test_ringBuffer_ForEach_Empty(t *testing.T) {
	rb := newRingBuffer(5)

	var messages []LogMessage

	rb.forEach(func(msg LogMessage) {
		messages = append(messages, msg)
	})

	assert.Empty(t, messages)
}

func Test_Hub_HistoryReplay_OnRegister(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	h := NewHub(10, 50, testLogger())
	go h.Run(ctx)

	//nolint:forbidigo // allow hub goroutine to process
	time.Sleep(10 * time.Millisecond)

	h.Broadcast("api", "history-1")
	h.Broadcast("web", "history-2")
	h.Broadcast("api", "history-3")

	//nolint:forbidigo // allow hub goroutine to process
	time.Sleep(10 * time.Millisecond)

	conn := NewClientConn("client-1", 50)
	h.Register(conn)

	//nolint:forbidigo // allow hub goroutine to process
	time.Sleep(10 * time.Millisecond)

	var messages []LogMessage

	for {
		select {
		case msg := <-conn.SendChan:
			messages = append(messages, msg)
		case <-time.After(50 * time.Millisecond):
			require.Len(t, messages, 3)
			assert.Equal(t, "history-1", messages[0].Message)
			assert.Equal(t, "history-2", messages[1].Message)
			assert.Equal(t, "history-3", messages[2].Message)

			return
		}
	}
}

func Test_Hub_Run_TickerLogsDroppedMessages(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())

	h := NewHub(1, 50, testLogger()).(*hub)

	done := make(chan struct{})

	go func() {
		h.Run(ctx)
		close(done)
	}()

	conn := NewClientConn("client-1", 1)
	h.Register(conn)

	//nolint:forbidigo // allow hub goroutine to process
	time.Sleep(10 * time.Millisecond)

	for range 10 {
		h.Broadcast("api", "msg")
	}

	//nolint:forbidigo // wait for ticker to fire and reset dropped counter
	time.Sleep(6 * time.Second)

	assert.Equal(t, int64(0), h.dropped.Load())

	cancel()
	<-done
}

func Test_Hub_Register_ReplayDropsWhenClientBufferFull(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	h := NewHub(10, 50, testLogger())
	go h.Run(ctx)

	//nolint:forbidigo // allow hub goroutine to process
	time.Sleep(10 * time.Millisecond)

	for i := range 20 {
		h.Broadcast("api", fmt.Sprintf("history-%d", i))
	}

	//nolint:forbidigo // allow hub goroutine to process
	time.Sleep(10 * time.Millisecond)

	conn := NewClientConn("tiny-client", 5)
	h.Register(conn)

	//nolint:forbidigo // allow hub goroutine to process
	time.Sleep(10 * time.Millisecond)

	received := 0

	for {
		select {
		case <-conn.SendChan:
			received++
		default:
			assert.Equal(t, 5, received)

			return
		}
	}
}

func Test_Hub_HistoryReplay_WithSubscriptionFilter(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	h := NewHub(10, 50, testLogger())
	go h.Run(ctx)

	//nolint:forbidigo // allow hub goroutine to process
	time.Sleep(10 * time.Millisecond)

	h.Broadcast("api", "api-msg-1")
	h.Broadcast("web", "web-msg-1")
	h.Broadcast("api", "api-msg-2")
	h.Broadcast("db", "db-msg-1")

	//nolint:forbidigo // allow hub goroutine to process
	time.Sleep(10 * time.Millisecond)

	conn := NewClientConn("client-1", 50)
	conn.SetSubscription([]string{"api"})
	h.Register(conn)

	//nolint:forbidigo // allow hub goroutine to process
	time.Sleep(10 * time.Millisecond)

	var messages []LogMessage

	for {
		select {
		case msg := <-conn.SendChan:
			messages = append(messages, msg)
		case <-time.After(50 * time.Millisecond):
			require.Len(t, messages, 2)
			assert.Equal(t, "api-msg-1", messages[0].Message)
			assert.Equal(t, "api-msg-2", messages[1].Message)

			return
		}
	}
}

// seedHub starts a hub and broadcasts the given service lines, waiting until they are buffered
func seedHub(ctx context.Context, t *testing.T, lines [][2]string) Hub {
	t.Helper()

	h := NewHub(100, 50, testLogger())

	go h.Run(ctx)

	for _, line := range lines {
		h.Broadcast(line[0], line[1])
	}

	require.Eventually(t, func() bool {
		return len(h.History(HistoryQuery{})) == len(lines)
	}, time.Second, 5*time.Millisecond)

	return h
}

func Test_Hub_History(t *testing.T) {
	lines := [][2]string{
		{"api", "first"},
		{"web", "second"},
		{"api", "third"},
		{"api", "fourth"},
	}

	tests := []struct {
		name     string
		query    HistoryQuery
		expected []string
	}{
		{
			name:     "returns every buffered line oldest first",
			query:    HistoryQuery{},
			expected: []string{"first", "second", "third", "fourth"},
		},
		{
			name:     "filters by service",
			query:    HistoryQuery{Services: []string{"api"}},
			expected: []string{"first", "third", "fourth"},
		},
		{
			name:     "keeps only the newest tail entries",
			query:    HistoryQuery{Tail: 2},
			expected: []string{"third", "fourth"},
		},
		{
			name:     "applies the tail after the service filter",
			query:    HistoryQuery{Services: []string{"api"}, Tail: 2},
			expected: []string{"third", "fourth"},
		},
		{
			name:     "reports nothing for an unknown service",
			query:    HistoryQuery{Services: []string{"missing"}},
			expected: nil,
		},
	}

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	h := seedHub(ctx, t, lines)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			messages := h.History(tt.query)

			actual := make([]string, 0, len(messages))
			for _, msg := range messages {
				actual = append(actual, msg.Message)
			}

			assert.Equal(t, tt.expected, nilWhenEmpty(actual))
		})
	}
}

// nilWhenEmpty normalizes an empty slice to nil so table expectations stay readable
func nilWhenEmpty(values []string) []string {
	if len(values) == 0 {
		return nil
	}

	return values
}

func Test_Hub_History_SkipsEntriesBeforeSince(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	h := seedHub(ctx, t, [][2]string{{"api", "old"}})

	cutoff := time.Now()

	h.Broadcast("api", "new")

	require.Eventually(t, func() bool {
		return len(h.History(HistoryQuery{})) == 2
	}, time.Second, 5*time.Millisecond)

	messages := h.History(HistoryQuery{Since: cutoff})

	require.Len(t, messages, 1)
	assert.Equal(t, "new", messages[0].Message)
}

func Test_Hub_History_StampsBroadcastTime(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	before := time.Now()
	h := seedHub(ctx, t, [][2]string{{"api", "line"}})

	messages := h.History(HistoryQuery{})

	require.Len(t, messages, 1)
	assert.False(t, messages[0].Timestamp.Before(before))
}

func Test_Hub_History_AnswersWithoutRunningHub(t *testing.T) {
	h := NewHub(100, 50, testLogger())

	answered := make(chan []LogMessage, 1)

	go func() {
		answered <- h.History(HistoryQuery{})
	}()

	select {
	case messages := <-answered:
		assert.Empty(t, messages)
	case <-time.After(time.Second):
		t.Fatal("History blocked while the hub loop was not running")
	}
}

func Test_Hub_Register_ClosesNoFollowClientAfterReplay(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	h := seedHub(ctx, t, [][2]string{{"api", "first"}, {"api", "second"}, {"web", "other"}})

	client := NewClientConn("client-1", 10)
	client.SetSubscription([]string{"api"})
	client.Tail = 1
	client.NoFollow = true

	h.Register(client)

	var received []string

	for msg := range client.SendChan {
		received = append(received, msg.Message)
	}

	assert.Equal(t, []string{"second"}, received)
}

func Test_Hub_Register_KeepsFollowingClientOpen(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	h := seedHub(ctx, t, [][2]string{{"api", "buffered"}})

	client := NewClientConn("client-1", 10)

	h.Register(client)

	assert.Equal(t, "buffered", (<-client.SendChan).Message)

	h.Broadcast("api", "live")

	assert.Equal(t, "live", (<-client.SendChan).Message)
}
