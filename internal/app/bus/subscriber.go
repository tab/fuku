package bus

import "sync"

// subscriber wraps a channel with a FIFO overflow queue for critical messages
type subscriber struct {
	ch       chan Message
	overflow []Message
	signal   chan struct{}
	quit     chan struct{}
	done     chan struct{}
	mu       sync.Mutex
	closed   bool
}

func newSubscriber(bufferSize int) *subscriber {
	s := &subscriber{
		ch:     make(chan Message, bufferSize),
		signal: make(chan struct{}, 1),
		quit:   make(chan struct{}),
		done:   make(chan struct{}),
	}

	go s.pump()

	return s
}

// send attempts a non-blocking send; if full and critical, queues for FIFO drain
func (s *subscriber) send(msg Message) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()

		return
	}

	switch {
	case len(s.overflow) > 0 && msg.Critical:
		s.overflow = append(s.overflow, msg)
		s.mu.Unlock()
		s.notify()

		return
	case len(s.overflow) > 0:
		s.mu.Unlock()

		return
	}
	s.mu.Unlock()

	select {
	case s.ch <- msg:
		return
	default:
	}

	if !msg.Critical {
		return
	}

	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()

		return
	}

	s.overflow = append(s.overflow, msg)
	s.mu.Unlock()
	s.notify()
}

// notify signals the pump goroutine that overflow has messages to drain
func (s *subscriber) notify() {
	select {
	case s.signal <- struct{}{}:
	default:
	}
}

// pump drains overflow messages into the channel in FIFO order
func (s *subscriber) pump() {
	defer close(s.done)

	for {
		select {
		case <-s.signal:
		case <-s.quit:
			return
		}

		for {
			s.mu.Lock()
			if len(s.overflow) == 0 {
				s.mu.Unlock()

				break
			}

			msg := s.overflow[0]
			s.mu.Unlock()

			select {
			case s.ch <- msg:
				s.mu.Lock()
				s.overflow = s.overflow[1:]
				s.mu.Unlock()
			case <-s.quit:
				return
			}
		}
	}
}

// close stops the pump goroutine and closes the channel
func (s *subscriber) close() {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()

		return
	}

	s.closed = true
	s.mu.Unlock()

	close(s.quit)
	<-s.done
	close(s.ch)
}
