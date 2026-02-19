package session

import (
	"context"
	"time"

	"fuku/internal/app/bus"
	"fuku/internal/config/logger"
)

// Listener subscribes to bus events and manages session state reactively
type Listener interface {
	Start(ctx context.Context)
}

type listener struct {
	session Session
	bus     bus.Bus
	log     logger.Logger
}

// NewListener creates a new session event listener
func NewListener(session Session, b bus.Bus, log logger.Logger) Listener {
	return &listener{
		session: session,
		bus:     b,
		log:     log.WithComponent("SESSION"),
	}
}

// Start begins listening for bus events and managing session state
func (l *listener) Start(ctx context.Context) {
	l.cleanupStaleSession()

	msgCh := l.bus.Subscribe(ctx)

	go func() {
		for msg := range msgCh {
			l.handleEvent(msg)
		}
	}()
}

func (l *listener) handleEvent(msg bus.Message) {
	switch msg.Type {
	case bus.EventProfileResolved:
		if data, ok := msg.Data.(bus.ProfileResolved); ok {
			l.onProfileResolved(data.Profile, msg.Timestamp)
		}
	case bus.EventServiceStarting:
		if data, ok := msg.Data.(bus.ServiceStarting); ok {
			l.onServiceStarting(data.Service, data.PID, msg.Timestamp)
		}
	case bus.EventServiceStopped:
		if data, ok := msg.Data.(bus.ServiceStopped); ok {
			l.onServiceStopped(data.Service)
		}
	case bus.EventPhaseChanged:
		if data, ok := msg.Data.(bus.PhaseChanged); ok && data.Phase == bus.PhaseStopped {
			l.onPhaseStopped()
		}
	}
}

func (l *listener) onProfileResolved(profile string, timestamp time.Time) {
	if err := l.session.Save(&State{
		Profile:   profile,
		StartedAt: timestamp,
	}); err != nil {
		l.log.Warn().Err(err).Msg("Failed to save session state")
	}
}

func (l *listener) onServiceStarting(service string, pid int, timestamp time.Time) {
	if err := l.session.Add(Entry{
		Service:   service,
		PID:       pid,
		StartedAt: timestamp,
	}); err != nil {
		l.log.Warn().Err(err).Msgf("Failed to track session for service '%s'", service)
	}
}

func (l *listener) onServiceStopped(service string) {
	if err := l.session.Remove(service); err != nil {
		l.log.Warn().Err(err).Msgf("Failed to remove session entry for service '%s'", service)
	}
}

func (l *listener) onPhaseStopped() {
	if err := l.session.Delete(); err != nil {
		l.log.Warn().Err(err).Msg("Failed to delete session file")
	}
}

func (l *listener) cleanupStaleSession() {
	state, err := l.session.Load()
	if err != nil {
		return
	}

	killed := KillOrphans(state, l.log)
	if killed > 0 {
		l.log.Info().Msgf("Cleaned up %d orphaned process(es) from previous session", killed)
	}

	if err := l.session.Delete(); err != nil {
		l.log.Warn().Err(err).Msg("Failed to delete stale session file")
	}
}
