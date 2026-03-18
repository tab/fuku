package logs

import (
	"context"
	"sync/atomic"
	"time"

	"fuku/internal/config/logger"
)

// Hub manages client connections and broadcasts log messages
type Hub interface {
	Register(conn *ClientConn)
	Unregister(conn *ClientConn)
	Broadcast(service, message string)
	Run(ctx context.Context)
}

// ClientConn represents a connected client
type ClientConn struct {
	ID       string
	Services map[string]bool // subscribed services (empty = all)
	SendChan chan LogMessage
}

// NewClientConn creates a new client connection with the specified buffer size
func NewClientConn(id string, bufferSize int) *ClientConn {
	return &ClientConn{
		ID:       id,
		Services: make(map[string]bool),
		SendChan: make(chan LogMessage, bufferSize),
	}
}

// SetSubscription sets the services this client is subscribed to
func (c *ClientConn) SetSubscription(services []string) {
	c.Services = make(map[string]bool)
	for _, svc := range services {
		c.Services[svc] = true
	}
}

// ShouldReceive returns true if client should receive logs for the given service
func (c *ClientConn) ShouldReceive(service string) bool {
	if len(c.Services) == 0 {
		return true
	}

	return c.Services[service]
}

// ringBuffer is a fixed-size circular buffer for log message history
type ringBuffer struct {
	items []LogMessage
	head  int
	count int
}

// newRingBuffer creates a ring buffer with the given capacity
func newRingBuffer(capacity int) *ringBuffer {
	return &ringBuffer{
		items: make([]LogMessage, capacity),
	}
}

// push adds a message, overwriting the oldest if full
func (r *ringBuffer) push(msg LogMessage) {
	r.items[r.head] = msg

	r.head = (r.head + 1) % len(r.items)
	if r.count < len(r.items) {
		r.count++
	}
}

// forEach iterates from oldest to newest
func (r *ringBuffer) forEach(fn func(LogMessage)) {
	if r.count == 0 {
		return
	}

	start := (r.head - r.count + len(r.items)) % len(r.items)
	for i := range r.count {
		fn(r.items[(start+i)%len(r.items)])
	}
}

// hub implements the Hub interface
type hub struct {
	clients    map[*ClientConn]bool
	register   chan *ClientConn
	unregister chan *ClientConn
	broadcast  chan LogMessage
	done       chan struct{}
	history    *ringBuffer
	log        logger.Logger
	dropped    atomic.Int64
}

// NewHub creates a new Hub instance with the specified buffer and history sizes
func NewHub(bufferSize int, historySize int, log logger.Logger) Hub {
	return &hub{
		clients:    make(map[*ClientConn]bool),
		register:   make(chan *ClientConn),
		unregister: make(chan *ClientConn),
		broadcast:  make(chan LogMessage, bufferSize),
		done:       make(chan struct{}),
		history:    newRingBuffer(historySize),
		log:        log,
	}
}

// Register adds a client to the hub
func (h *hub) Register(conn *ClientConn) {
	select {
	case h.register <- conn:
	case <-h.done:
	}
}

// Unregister removes a client from the hub
func (h *hub) Unregister(conn *ClientConn) {
	select {
	case h.unregister <- conn:
	case <-h.done:
	}
}

// Broadcast sends a log message to all subscribed clients
func (h *hub) Broadcast(service, message string) {
	msg := LogMessage{
		Type:    MessageLog,
		Service: service,
		Message: message,
	}

	select {
	case h.broadcast <- msg:
	default:
		h.dropped.Add(1)
	}
}

// Run starts the hub's main loop
func (h *hub) Run(ctx context.Context) {
	defer close(h.done)

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			for client := range h.clients {
				close(client.SendChan)
				delete(h.clients, client)
			}

			return
		case <-ticker.C:
			if dropped := h.dropped.Swap(0); dropped > 0 {
				h.log.Warn().Msgf("Dropped %d log messages (buffer full)", dropped)
			}
		case client := <-h.register:
			h.clients[client] = true

			replayed := 0

			h.history.forEach(func(msg LogMessage) {
				if !client.ShouldReceive(msg.Service) {
					return
				}

				select {
				case client.SendChan <- msg:
					replayed++
				default:
					h.dropped.Add(1)
				}
			})

			h.log.Debug().Msgf("Client %s registered, replayed %d messages", client.ID, replayed)
		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				close(client.SendChan)
				delete(h.clients, client)
			}
		case msg := <-h.broadcast:
			h.history.push(msg)

			for client := range h.clients {
				if client.ShouldReceive(msg.Service) {
					select {
					case client.SendChan <- msg:
					default:
						h.dropped.Add(1)
					}
				}
			}
		}
	}
}
