package runtime

import (
	"context"
	"sync"
)

// CommandType represents the type of command
type CommandType string

// Command types for service control
const (
	CommandStopService    CommandType = "stop_service"
	CommandRestartService CommandType = "restart_service"
	CommandStopAll        CommandType = "stop_all"
)

// Command represents a runtime command
type Command struct {
	Type CommandType
	Data interface{}
}

// StopServiceData contains service stop command details
type StopServiceData struct {
	Service string
}

// RestartServiceData contains service restart command details
type RestartServiceData struct {
	Service string
}

// CommandBus defines the interface for command publishing and subscription
type CommandBus interface {
	Subscribe(ctx context.Context) <-chan Command
	Publish(cmd Command)
	Close()
}

// commandBus implements the CommandBus interface
type commandBus struct {
	subscribers []chan Command
	mu          sync.RWMutex
	bufferSize  int
	closed      bool
}

// NewCommandBus creates a new command bus with the specified buffer size
func NewCommandBus(bufferSize int) CommandBus {
	return &commandBus{
		subscribers: make([]chan Command, 0),
		bufferSize:  bufferSize,
	}
}

// Subscribe creates a new subscription channel for commands
func (cb *commandBus) Subscribe(ctx context.Context) <-chan Command {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	ch := make(chan Command, cb.bufferSize)
	cb.subscribers = append(cb.subscribers, ch)

	go func() {
		<-ctx.Done()
		cb.unsubscribe(ch)
	}()

	return ch
}

// Publish sends a command to all subscribers
func (cb *commandBus) Publish(cmd Command) {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	if cb.closed {
		return
	}

	for _, ch := range cb.subscribers {
		select {
		case ch <- cmd:
		default:
		}
	}
}

// Close closes all subscriber channels
func (cb *commandBus) Close() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if cb.closed {
		return
	}

	cb.closed = true

	for _, ch := range cb.subscribers {
		close(ch)
	}

	cb.subscribers = nil
}

// unsubscribe removes a channel from subscribers and closes it
func (cb *commandBus) unsubscribe(ch chan Command) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	for i, sub := range cb.subscribers {
		if sub == ch {
			cb.subscribers = append(cb.subscribers[:i], cb.subscribers[i+1:]...)

			close(ch)

			break
		}
	}
}

// NoOpCommandBus is a no-operation command bus for when UI is disabled
type noOpCommandBus struct{}

// NewNoOpCommandBus creates a no-op command bus
func NewNoOpCommandBus() CommandBus {
	return &noOpCommandBus{}
}

// Subscribe returns a channel that closes when context is cancelled
func (ncb *noOpCommandBus) Subscribe(ctx context.Context) <-chan Command {
	ch := make(chan Command)

	go func() {
		<-ctx.Done()
		close(ch)
	}()

	return ch
}

// Publish is a no-op
func (ncb *noOpCommandBus) Publish(cmd Command) {}

// Close is a no-op
func (ncb *noOpCommandBus) Close() {}
