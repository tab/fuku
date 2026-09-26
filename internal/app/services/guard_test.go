package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"fuku/internal/contracts"
	"fuku/internal/model"
)

func Test_Guard_admit(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTracker := NewMockTracker(ctrl)
	mockProcess := NewMockProcess(ctrl)

	svc := model.Service{ID: "test-id-api", Name: "api"}
	tiers := []model.Tier{{Name: "platform", Services: []*model.Service{&svc}}}
	alive := make(chan struct{})
	exited := make(chan struct{})

	close(exited)

	tests := []struct {
		name     string
		before   func() *Guard
		id       string
		action   contracts.Action
		expected Admission
		err      error
	}{
		{
			name: "rejects every action before a run",
			before: func() *Guard {
				return NewGuard(mockTracker)
			},
			id:     svc.ID,
			action: contracts.ActionRestart,
			err:    contracts.ErrNotAccepting,
		},
		{
			name: "rejects every action before the profile is resolved",
			before: func() *Guard {
				guard := NewGuard(mockTracker)
				guard.open()

				return guard
			},
			id:     svc.ID,
			action: contracts.ActionRestart,
			err:    contracts.ErrNotAccepting,
		},
		{
			name: "rejects every action once stopping",
			before: func() *Guard {
				guard := NewGuard(mockTracker)
				guard.open()
				guard.resolve(tiers)
				guard.dispatch(svc.ID)
				guard.release(svc.ID)
				guard.setPhase(model.PhaseStopping)

				return guard
			},
			id:     svc.ID,
			action: contracts.ActionRestart,
			err:    contracts.ErrNotAccepting,
		},
		{
			name: "rejects a service outside the resolved profile",
			before: func() *Guard {
				guard := NewGuard(mockTracker)
				guard.open()
				guard.resolve(tiers)

				return guard
			},
			id:     "test-id-unknown",
			action: contracts.ActionStart,
			err:    contracts.ErrServiceNotFound,
		},
		{
			name: "rejects an action on a pending service",
			before: func() *Guard {
				guard := NewGuard(mockTracker)
				guard.open()
				guard.resolve(tiers)

				return guard
			},
			id:     svc.ID,
			action: contracts.ActionRestart,
			err:    contracts.ErrActionNotAllowed,
		},
		{
			name: "rejects an action while a token is held",
			before: func() *Guard {
				guard := NewGuard(mockTracker)
				guard.open()
				guard.resolve(tiers)
				guard.dispatch(svc.ID)

				return guard
			},
			id:     svc.ID,
			action: contracts.ActionStop,
			err:    contracts.ErrServiceBusy,
		},
		{
			name: "rejects a start of a live service",
			before: func() *Guard {
				guard := NewGuard(mockTracker)
				guard.open()
				guard.resolve(tiers)
				guard.dispatch(svc.ID)
				guard.release(svc.ID)

				mockTracker.EXPECT().Get(svc.ID).Return(mockProcess, true)
				mockProcess.EXPECT().Done().Return(alive)

				return guard
			},
			id:     svc.ID,
			action: contracts.ActionStart,
			err:    contracts.ErrActionNotAllowed,
		},
		{
			name: "admits a start of a service without a child",
			before: func() *Guard {
				guard := NewGuard(mockTracker)
				guard.open()
				guard.resolve(tiers)
				guard.dispatch(svc.ID)
				guard.release(svc.ID)

				mockTracker.EXPECT().Get(svc.ID).Return(nil, false)

				return guard
			},
			id:       svc.ID,
			action:   contracts.ActionStart,
			expected: Admission{Service: svc, Action: contracts.ActionStart, Status: model.StatusStarting},
		},
		{
			name: "admits a start of a service whose child has exited",
			before: func() *Guard {
				guard := NewGuard(mockTracker)
				guard.open()
				guard.resolve(tiers)
				guard.dispatch(svc.ID)
				guard.release(svc.ID)

				mockTracker.EXPECT().Get(svc.ID).Return(mockProcess, true)
				mockProcess.EXPECT().Done().Return(exited)

				return guard
			},
			id:       svc.ID,
			action:   contracts.ActionStart,
			expected: Admission{Service: svc, Action: contracts.ActionStart, Status: model.StatusStarting},
		},
		{
			name: "rejects a stop of a service without a child",
			before: func() *Guard {
				guard := NewGuard(mockTracker)
				guard.open()
				guard.resolve(tiers)
				guard.dispatch(svc.ID)
				guard.release(svc.ID)

				mockTracker.EXPECT().Get(svc.ID).Return(nil, false)

				return guard
			},
			id:     svc.ID,
			action: contracts.ActionStop,
			err:    contracts.ErrActionNotAllowed,
		},
		{
			name: "admits a stop of a live service during running",
			before: func() *Guard {
				guard := NewGuard(mockTracker)
				guard.open()
				guard.resolve(tiers)
				guard.dispatch(svc.ID)
				guard.release(svc.ID)
				guard.setPhase(model.PhaseRunning)

				mockTracker.EXPECT().Get(svc.ID).Return(mockProcess, true)
				mockProcess.EXPECT().Done().Return(alive)

				return guard
			},
			id:       svc.ID,
			action:   contracts.ActionStop,
			expected: Admission{Service: svc, Action: contracts.ActionStop, Status: model.StatusStopping},
		},
		{
			name: "admits a restart of a stopped service during startup",
			before: func() *Guard {
				guard := NewGuard(mockTracker)
				guard.open()
				guard.resolve(tiers)
				guard.dispatch(svc.ID)
				guard.release(svc.ID)

				mockTracker.EXPECT().Get(svc.ID).Return(nil, false)

				return guard
			},
			id:       svc.ID,
			action:   contracts.ActionRestart,
			expected: Admission{Service: svc, Action: contracts.ActionRestart, Status: model.StatusRestarting},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			guard := tt.before()

			admission, err := guard.admit(tt.id, tt.action)

			require.ErrorIs(t, err, tt.err)
			assert.Equal(t, tt.expected, admission)
		})
	}
}

func Test_Guard_admit_HoldsTheToken(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTracker := NewMockTracker(ctrl)

	svc := model.Service{ID: "test-id-api", Name: "api"}
	guard := NewGuard(mockTracker)

	guard.open()
	guard.resolve([]model.Tier{{Name: "platform", Services: []*model.Service{&svc}}})
	guard.dispatch(svc.ID)
	guard.release(svc.ID)

	mockTracker.EXPECT().Get(svc.ID).Return(nil, false)

	_, err := guard.admit(svc.ID, contracts.ActionStart)
	_, again := guard.admit(svc.ID, contracts.ActionRestart)

	require.NoError(t, err)
	require.ErrorIs(t, again, contracts.ErrServiceBusy)
	assert.False(t, guard.reserve(svc.ID))
}

func Test_Guard_reserve(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTracker := NewMockTracker(ctrl)
	mockProcess := NewMockProcess(ctrl)

	svc := model.Service{ID: "test-id-api", Name: "api"}
	tiers := []model.Tier{{Name: "platform", Services: []*model.Service{&svc}}}
	alive := make(chan struct{})

	tests := []struct {
		name     string
		before   func() *Guard
		id       string
		expected bool
	}{
		{
			name: "takes the free token of a dispatched service",
			before: func() *Guard {
				guard := NewGuard(mockTracker)
				guard.open()
				guard.resolve(tiers)
				guard.dispatch(svc.ID)
				guard.release(svc.ID)

				return guard
			},
			id:       svc.ID,
			expected: true,
		},
		{
			name: "refuses a held token",
			before: func() *Guard {
				guard := NewGuard(mockTracker)
				guard.open()
				guard.resolve(tiers)
				guard.dispatch(svc.ID)

				return guard
			},
			id: svc.ID,
		},
		{
			name: "refuses a pending service",
			before: func() *Guard {
				guard := NewGuard(mockTracker)
				guard.open()
				guard.resolve(tiers)

				return guard
			},
			id: svc.ID,
		},
		{
			name: "refuses an unknown service",
			before: func() *Guard {
				guard := NewGuard(mockTracker)
				guard.open()
				guard.resolve(tiers)

				return guard
			},
			id: "test-id-unknown",
		},
		{
			name: "refuses every service once stopping",
			before: func() *Guard {
				guard := NewGuard(mockTracker)
				guard.open()
				guard.resolve(tiers)
				guard.dispatch(svc.ID)
				guard.release(svc.ID)
				guard.setPhase(model.PhaseStopping)

				return guard
			},
			id: svc.ID,
		},
		{
			name: "refuses a service stopped on purpose, so a late file change never restarts it",
			before: func() *Guard {
				guard := NewGuard(mockTracker)
				guard.open()
				guard.resolve(tiers)
				guard.dispatch(svc.ID)
				guard.release(svc.ID)

				mockTracker.EXPECT().Get(svc.ID).Return(mockProcess, true)
				mockProcess.EXPECT().Done().Return(alive)

				_, err := guard.admit(svc.ID, contracts.ActionStop)
				require.NoError(t, err)
				guard.release(svc.ID)

				return guard
			},
			id: svc.ID,
		},
		{
			name: "takes the token of a service started again after a stop",
			before: func() *Guard {
				guard := NewGuard(mockTracker)
				guard.open()
				guard.resolve(tiers)
				guard.dispatch(svc.ID)
				guard.release(svc.ID)

				mockTracker.EXPECT().Get(svc.ID).Return(mockProcess, true)
				mockProcess.EXPECT().Done().Return(alive)

				_, err := guard.admit(svc.ID, contracts.ActionStop)
				require.NoError(t, err)
				guard.release(svc.ID)

				mockTracker.EXPECT().Get(svc.ID).Return(nil, false)

				_, err = guard.admit(svc.ID, contracts.ActionStart)
				require.NoError(t, err)
				guard.release(svc.ID)

				return guard
			},
			id:       svc.ID,
			expected: true,
		},
		{
			name: "takes the token of a failed service without a child, so a file change can recover it",
			before: func() *Guard {
				guard := NewGuard(mockTracker)
				guard.open()
				guard.resolve(tiers)
				guard.dispatch(svc.ID)
				guard.release(svc.ID)
				guard.setPhase(model.PhaseRunning)

				return guard
			},
			id:       svc.ID,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			guard := tt.before()

			reserved := guard.reserve(tt.id)

			assert.Equal(t, tt.expected, reserved)
		})
	}
}

func Test_Guard_release(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTracker := NewMockTracker(ctrl)

	svc := model.Service{ID: "test-id-api", Name: "api"}
	tiers := []model.Tier{{Name: "platform", Services: []*model.Service{&svc}}}

	tests := []struct {
		name   string
		before func() *Guard
	}{
		{
			name: "frees a dispatched service",
			before: func() *Guard {
				guard := NewGuard(mockTracker)
				guard.open()
				guard.resolve(tiers)
				guard.dispatch(svc.ID)

				return guard
			},
		},
		{
			name: "frees an admitted service",
			before: func() *Guard {
				guard := NewGuard(mockTracker)
				guard.open()
				guard.resolve(tiers)
				guard.dispatch(svc.ID)
				guard.release(svc.ID)

				mockTracker.EXPECT().Get(svc.ID).Return(nil, false)

				_, err := guard.admit(svc.ID, contracts.ActionStart)
				require.NoError(t, err)

				return guard
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			guard := tt.before()

			guard.release(svc.ID)

			assert.True(t, guard.reserve(svc.ID))
		})
	}
}

func Test_Guard_halt(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTracker := NewMockTracker(ctrl)

	tests := []struct {
		name     string
		before   func() *Guard
		expected model.Phase
		phase    model.Phase
	}{
		{
			name: "leaves a guard without a run alone",
			before: func() *Guard {
				return NewGuard(mockTracker)
			},
		},
		{
			name: "closes a run in startup before the profile is resolved",
			before: func() *Guard {
				guard := NewGuard(mockTracker)
				guard.open()

				return guard
			},
			expected: model.PhaseStartup,
			phase:    model.PhaseStopping,
		},
		{
			name: "closes a running run",
			before: func() *Guard {
				guard := NewGuard(mockTracker)
				guard.open()
				guard.setPhase(model.PhaseRunning)

				return guard
			},
			expected: model.PhaseRunning,
			phase:    model.PhaseStopping,
		},
		{
			name: "reports a run already stopping",
			before: func() *Guard {
				guard := NewGuard(mockTracker)
				guard.open()
				guard.setPhase(model.PhaseStopping)

				return guard
			},
			expected: model.PhaseStopping,
			phase:    model.PhaseStopping,
		},
		{
			name: "reports a stopped run",
			before: func() *Guard {
				guard := NewGuard(mockTracker)
				guard.open()
				guard.setPhase(model.PhaseStopped)

				return guard
			},
			expected: model.PhaseStopped,
			phase:    model.PhaseStopped,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			guard := tt.before()

			found := guard.halt()

			assert.Equal(t, tt.expected, found)
			assert.Equal(t, tt.phase, guard.phase)
		})
	}
}

func Test_Guard_settle(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTracker := NewMockTracker(ctrl)

	tests := []struct {
		name     string
		before   func() *Guard
		expected model.Phase
	}{
		{
			name: "moves a run in startup to running",
			before: func() *Guard {
				guard := NewGuard(mockTracker)
				guard.open()

				return guard
			},
			expected: model.PhaseRunning,
		},
		{
			name: "keeps a run halted during startup stopping",
			before: func() *Guard {
				guard := NewGuard(mockTracker)
				guard.open()
				guard.halt()

				return guard
			},
			expected: model.PhaseStopping,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			guard := tt.before()

			guard.settle()

			assert.Equal(t, tt.expected, guard.phase)
		})
	}
}
