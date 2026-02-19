package session

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"fuku/internal/app/bus"
	"fuku/internal/app/errors"
	"fuku/internal/config/logger"
)

func Test_NewListener(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSession := NewMockSession(ctrl)
	mockBus := bus.NoOp()
	mockLog := logger.NewMockLogger(ctrl)
	componentLog := logger.NewMockLogger(ctrl)
	mockLog.EXPECT().WithComponent("SESSION").Return(componentLog)

	l := NewListener(mockSession, mockBus, mockLog)

	assert.NotNil(t, l)
	instance, ok := l.(*listener)
	assert.True(t, ok)
	assert.Equal(t, mockSession, instance.session)
	assert.Equal(t, mockBus, instance.bus)
	assert.Equal(t, componentLog, instance.log)
}

func Test_Listener_CleanupStaleSession(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSession := NewMockSession(ctrl)
	mockLog := logger.NewMockLogger(ctrl)
	mockLog.EXPECT().Info().Return(nil).AnyTimes()
	mockLog.EXPECT().Debug().Return(nil).AnyTimes()

	state := &State{
		Profile: "test",
		Entries: []Entry{
			{Service: "api", PID: 999999, StartedAt: time.Now()},
		},
	}

	mockSession.EXPECT().Load().Return(state, nil)
	mockSession.EXPECT().Delete().Return(nil)

	l := &listener{
		session: mockSession,
		bus:     bus.NoOp(),
		log:     mockLog,
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	l.Start(ctx)
}

func Test_Listener_CleanupStaleSession_NoSession(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSession := NewMockSession(ctrl)
	mockLog := logger.NewMockLogger(ctrl)

	mockSession.EXPECT().Load().Return(nil, errors.ErrSessionNotFound)

	l := &listener{
		session: mockSession,
		bus:     bus.NoOp(),
		log:     mockLog,
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	l.Start(ctx)
}

func Test_Listener_ProfileResolved(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSession := NewMockSession(ctrl)
	mockLog := logger.NewMockLogger(ctrl)

	now := time.Now()

	mockSession.EXPECT().Save(gomock.Any()).DoAndReturn(func(state *State) error {
		assert.Equal(t, "core", state.Profile)
		assert.Equal(t, now, state.StartedAt)

		return nil
	})

	l := &listener{
		session: mockSession,
		bus:     bus.NoOp(),
		log:     mockLog,
	}

	l.handleEvent(bus.Message{
		Type:      bus.EventProfileResolved,
		Timestamp: now,
		Data:      bus.ProfileResolved{Profile: "core"},
	})
}

func Test_Listener_ProfileResolved_SaveError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSession := NewMockSession(ctrl)
	mockLog := logger.NewMockLogger(ctrl)
	mockLog.EXPECT().Warn().Return(nil)

	mockSession.EXPECT().Save(gomock.Any()).Return(errors.ErrSessionFileWrite)

	l := &listener{
		session: mockSession,
		bus:     bus.NoOp(),
		log:     mockLog,
	}

	l.handleEvent(bus.Message{
		Type: bus.EventProfileResolved,
		Data: bus.ProfileResolved{Profile: "core"},
	})
}

func Test_Listener_ServiceStarting(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSession := NewMockSession(ctrl)
	mockLog := logger.NewMockLogger(ctrl)

	now := time.Now()

	mockSession.EXPECT().Add(gomock.Any()).DoAndReturn(func(entry Entry) error {
		assert.Equal(t, "api", entry.Service)
		assert.Equal(t, 1234, entry.PID)
		assert.Equal(t, now, entry.StartedAt)

		return nil
	})

	l := &listener{
		session: mockSession,
		bus:     bus.NoOp(),
		log:     mockLog,
	}

	l.handleEvent(bus.Message{
		Type:      bus.EventServiceStarting,
		Timestamp: now,
		Data: bus.ServiceStarting{
			ServiceEvent: bus.ServiceEvent{Service: "api"},
			PID:          1234,
		},
	})
}

func Test_Listener_ServiceStarting_AddError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSession := NewMockSession(ctrl)
	mockLog := logger.NewMockLogger(ctrl)
	mockLog.EXPECT().Warn().Return(nil)

	mockSession.EXPECT().Add(gomock.Any()).Return(errors.ErrSessionFileWrite)

	l := &listener{
		session: mockSession,
		bus:     bus.NoOp(),
		log:     mockLog,
	}

	l.handleEvent(bus.Message{
		Type: bus.EventServiceStarting,
		Data: bus.ServiceStarting{
			ServiceEvent: bus.ServiceEvent{Service: "api"},
			PID:          1234,
		},
	})
}

func Test_Listener_ServiceStopped(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSession := NewMockSession(ctrl)
	mockLog := logger.NewMockLogger(ctrl)

	mockSession.EXPECT().Remove("api").Return(nil)

	l := &listener{
		session: mockSession,
		bus:     bus.NoOp(),
		log:     mockLog,
	}

	l.handleEvent(bus.Message{
		Type: bus.EventServiceStopped,
		Data: bus.ServiceStopped{
			ServiceEvent: bus.ServiceEvent{Service: "api"},
		},
	})
}

func Test_Listener_ServiceStopped_RemoveError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSession := NewMockSession(ctrl)
	mockLog := logger.NewMockLogger(ctrl)
	mockLog.EXPECT().Warn().Return(nil)

	mockSession.EXPECT().Remove("api").Return(errors.ErrSessionFileWrite)

	l := &listener{
		session: mockSession,
		bus:     bus.NoOp(),
		log:     mockLog,
	}

	l.handleEvent(bus.Message{
		Type: bus.EventServiceStopped,
		Data: bus.ServiceStopped{
			ServiceEvent: bus.ServiceEvent{Service: "api"},
		},
	})
}

func Test_Listener_PhaseStopped(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSession := NewMockSession(ctrl)
	mockLog := logger.NewMockLogger(ctrl)

	mockSession.EXPECT().Delete().Return(nil)

	l := &listener{
		session: mockSession,
		bus:     bus.NoOp(),
		log:     mockLog,
	}

	l.handleEvent(bus.Message{
		Type: bus.EventPhaseChanged,
		Data: bus.PhaseChanged{Phase: bus.PhaseStopped},
	})
}

func Test_Listener_PhaseStopped_DeleteError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSession := NewMockSession(ctrl)
	mockLog := logger.NewMockLogger(ctrl)
	mockLog.EXPECT().Warn().Return(nil)

	mockSession.EXPECT().Delete().Return(errors.ErrSessionFileWrite)

	l := &listener{
		session: mockSession,
		bus:     bus.NoOp(),
		log:     mockLog,
	}

	l.handleEvent(bus.Message{
		Type: bus.EventPhaseChanged,
		Data: bus.PhaseChanged{Phase: bus.PhaseStopped},
	})
}

func Test_Listener_IgnoresIrrelevantEvents(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSession := NewMockSession(ctrl)
	mockLog := logger.NewMockLogger(ctrl)

	l := &listener{
		session: mockSession,
		bus:     bus.NoOp(),
		log:     mockLog,
	}

	l.handleEvent(bus.Message{
		Type: bus.EventServiceReady,
		Data: bus.ServiceReady{ServiceEvent: bus.ServiceEvent{Service: "api"}},
	})

	l.handleEvent(bus.Message{
		Type: bus.EventPhaseChanged,
		Data: bus.PhaseChanged{Phase: bus.PhaseRunning},
	})

	l.handleEvent(bus.Message{
		Type: bus.EventPhaseChanged,
		Data: bus.PhaseChanged{Phase: bus.PhaseStartup},
	})

	l.handleEvent(bus.Message{
		Type: bus.EventWatchTriggered,
		Data: bus.WatchTriggered{Service: "api"},
	})
}
