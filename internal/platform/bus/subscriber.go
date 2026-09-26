package bus

import "fuku/internal/contracts"

// subscriber is a named subscription with a bounded queue and drop counters
type subscriber struct {
	name     string
	required bool
	types    map[contracts.MessageType]struct{}
	ch       chan contracts.Message
	drops    map[contracts.MessageType]uint64
	stop     func() bool
}

func newSubscriber(opts contracts.SubscribeOptions, queueDepth int) *subscriber {
	s := &subscriber{
		name:     opts.Name,
		required: opts.Required,
		ch:       make(chan contracts.Message, queueDepth),
		drops:    make(map[contracts.MessageType]uint64),
	}

	if opts.Types != nil {
		s.types = make(map[contracts.MessageType]struct{}, len(opts.Types))
		for _, t := range opts.Types {
			s.types[t] = struct{}{}
		}
	}

	return s
}

// Messages returns the queue, which closes when the subscription ends
func (s *subscriber) Messages() <-chan contracts.Message {
	return s.ch
}

// matches reports whether the subscription receives the type
func (s *subscriber) matches(msgType contracts.MessageType) bool {
	if s.types == nil {
		return true
	}

	_, ok := s.types[msgType]

	return ok
}

// full reports whether an open queue has no free slot
func (s *subscriber) full() bool {
	return len(s.ch) == cap(s.ch)
}

// send queues the message without waiting and counts a drop when the queue is full
func (s *subscriber) send(msg contracts.Message) {
	select {
	case s.ch <- msg:
	default:
		s.drops[msg.Type]++
	}
}

// close ends the subscription and returns its drop counters
func (s *subscriber) close() map[contracts.MessageType]uint64 {
	close(s.ch)

	return s.drops
}
