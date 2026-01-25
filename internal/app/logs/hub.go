package logs

import (
	"context"
	"sync"
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

// hub implements the Hub interface
type hub struct {
	bufferSize int
	clients    map[*ClientConn]bool
	register   chan *ClientConn
	unregister chan *ClientConn
	broadcast  chan LogMessage
	done       chan struct{}
	mu         sync.RWMutex
}

// NewHub creates a new Hub instance with the specified buffer size
func NewHub(bufferSize int) Hub {
	return &hub{
		bufferSize: bufferSize,
		clients:    make(map[*ClientConn]bool),
		register:   make(chan *ClientConn),
		unregister: make(chan *ClientConn),
		broadcast:  make(chan LogMessage, bufferSize),
		done:       make(chan struct{}),
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
	}
}

// Run starts the hub's main loop
func (h *hub) Run(ctx context.Context) {
	defer close(h.done)

	for {
		select {
		case <-ctx.Done():
			h.mu.Lock()

			for client := range h.clients {
				close(client.SendChan)
				delete(h.clients, client)
			}

			h.mu.Unlock()

			return
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
		case client := <-h.unregister:
			h.mu.Lock()

			if _, ok := h.clients[client]; ok {
				close(client.SendChan)
				delete(h.clients, client)
			}

			h.mu.Unlock()
		case msg := <-h.broadcast:
			h.mu.RLock()

			for client := range h.clients {
				if client.ShouldReceive(msg.Service) {
					select {
					case client.SendChan <- msg:
					default:
					}
				}
			}

			h.mu.RUnlock()
		}
	}
}
