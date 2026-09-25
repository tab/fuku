package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"fuku/internal/contracts"
	"fuku/internal/model"
)

func Test_Control_Actions(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTracker := NewMockTracker(ctrl)
	mockPublisher := NewMockPublisher(ctrl)
	mockProcess := NewMockProcess(ctrl)

	guard := NewGuard(mockTracker)
	control := NewControl(guard, mockPublisher)

	svc := model.Service{ID: "test-id-api", Name: "api"}
	tiers := []model.Tier{{Name: "platform", Services: []*model.Service{&svc}}}
	alive := make(chan struct{})

	tests := []struct {
		name     string
		before   func()
		act      func(id string) (Admission, error)
		expected Admission
		err      error
	}{
		{
			name: "start publishes the start command for an admitted service",
			before: func() {
				guard.open()
				guard.resolve(tiers)
				guard.dispatch(svc.ID)
				guard.release(svc.ID)

				mockTracker.EXPECT().Get(svc.ID).Return(nil, false)
				mockPublisher.EXPECT().Publish(contracts.Message{Type: contracts.CommandStartService, Data: svc}).Return(nil)
			},
			act:      control.Start,
			expected: Admission{Service: svc, Action: contracts.ActionStart, Status: model.StatusStarting},
		},
		{
			name: "stop publishes the stop command for an admitted service",
			before: func() {
				guard.open()
				guard.resolve(tiers)
				guard.dispatch(svc.ID)
				guard.release(svc.ID)

				mockTracker.EXPECT().Get(svc.ID).Return(mockProcess, true)
				mockProcess.EXPECT().Done().Return(alive)
				mockPublisher.EXPECT().Publish(contracts.Message{Type: contracts.CommandStopService, Data: svc}).Return(nil)
			},
			act:      control.Stop,
			expected: Admission{Service: svc, Action: contracts.ActionStop, Status: model.StatusStopping},
		},
		{
			name: "restart publishes the restart command for an admitted service",
			before: func() {
				guard.open()
				guard.resolve(tiers)
				guard.dispatch(svc.ID)
				guard.release(svc.ID)

				mockTracker.EXPECT().Get(svc.ID).Return(mockProcess, true)
				mockProcess.EXPECT().Done().Return(alive)
				mockPublisher.EXPECT().Publish(contracts.Message{Type: contracts.CommandRestartService, Data: svc}).Return(nil)
			},
			act:      control.Restart,
			expected: Admission{Service: svc, Action: contracts.ActionRestart, Status: model.StatusRestarting},
		},
		{
			name: "toggle publishes the stop command for a service with a live child",
			before: func() {
				guard.open()
				guard.resolve(tiers)
				guard.dispatch(svc.ID)
				guard.release(svc.ID)

				mockTracker.EXPECT().Get(svc.ID).Return(mockProcess, true).Times(2)
				mockProcess.EXPECT().Done().Return(alive).Times(2)
				mockPublisher.EXPECT().Publish(contracts.Message{Type: contracts.CommandStopService, Data: svc}).Return(nil)
			},
			act:      control.Toggle,
			expected: Admission{Service: svc, Action: contracts.ActionStop, Status: model.StatusStopping},
		},
		{
			name: "toggle publishes the start command for a service without a live child",
			before: func() {
				guard.open()
				guard.resolve(tiers)
				guard.dispatch(svc.ID)
				guard.release(svc.ID)

				mockTracker.EXPECT().Get(svc.ID).Return(nil, false).Times(2)
				mockPublisher.EXPECT().Publish(contracts.Message{Type: contracts.CommandStartService, Data: svc}).Return(nil)
			},
			act:      control.Toggle,
			expected: Admission{Service: svc, Action: contracts.ActionStart, Status: model.StatusStarting},
		},
		{
			name: "toggle of a busy service publishes nothing",
			before: func() {
				guard.open()
				guard.resolve(tiers)
				guard.dispatch(svc.ID)

				mockTracker.EXPECT().Get(svc.ID).Return(mockProcess, true)
				mockProcess.EXPECT().Done().Return(alive)
			},
			act: control.Toggle,
			err: contracts.ErrServiceBusy,
		},
		{
			name: "a rejected action publishes nothing",
			before: func() {
				guard.open()
				guard.resolve(tiers)
				guard.setPhase(model.PhaseStopping)
			},
			act: control.Restart,
			err: contracts.ErrNotAccepting,
		},
		{
			name: "a rejected publish returns the bus error",
			before: func() {
				guard.open()
				guard.resolve(tiers)
				guard.dispatch(svc.ID)
				guard.release(svc.ID)

				mockTracker.EXPECT().Get(svc.ID).Return(nil, false)
				mockPublisher.EXPECT().Publish(contracts.Message{Type: contracts.CommandStartService, Data: svc}).Return(contracts.ErrBusOverloaded)
			},
			act: control.Start,
			err: contracts.ErrBusOverloaded,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.before()

			admission, err := tt.act(svc.ID)

			require.ErrorIs(t, err, tt.err)
			assert.Equal(t, tt.expected, admission)
		})
	}
}

func Test_Control_Start_ReleasesOnPublishFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTracker := NewMockTracker(ctrl)
	mockPublisher := NewMockPublisher(ctrl)

	guard := NewGuard(mockTracker)
	control := NewControl(guard, mockPublisher)

	svc := model.Service{ID: "test-id-api", Name: "api"}

	guard.open()
	guard.resolve([]model.Tier{{Name: "platform", Services: []*model.Service{&svc}}})
	guard.dispatch(svc.ID)
	guard.release(svc.ID)

	mockTracker.EXPECT().Get(svc.ID).Return(nil, false)
	mockPublisher.EXPECT().Publish(gomock.Any()).Return(contracts.ErrBusOverloaded)

	_, err := control.Start(svc.ID)

	require.ErrorIs(t, err, contracts.ErrBusOverloaded)
	assert.True(t, guard.reserve(svc.ID))
}

func Test_Control_StopAll(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTracker := NewMockTracker(ctrl)
	mockPublisher := NewMockPublisher(ctrl)

	guard := NewGuard(mockTracker)
	control := NewControl(guard, mockPublisher)

	tiers := []model.Tier{{Name: "platform", Services: []*model.Service{{ID: "test-id-api", Name: "api"}}}}

	tests := []struct {
		name     string
		before   func()
		expected error
	}{
		{
			name: "is refused before a run",
			before: func() {
				guard.setPhase("")
			},
			expected: contracts.ErrNotAccepting,
		},
		{
			name: "publishes the command during startup before the profile is resolved",
			before: func() {
				guard.open()

				mockPublisher.EXPECT().Publish(contracts.Message{Type: contracts.CommandStopAll}).Return(nil)
			},
		},
		{
			name: "publishes the command during startup",
			before: func() {
				guard.open()
				guard.resolve(tiers)

				mockPublisher.EXPECT().Publish(contracts.Message{Type: contracts.CommandStopAll}).Return(nil)
			},
		},
		{
			name: "publishes the command during running",
			before: func() {
				guard.open()
				guard.resolve(tiers)
				guard.setPhase(model.PhaseRunning)

				mockPublisher.EXPECT().Publish(contracts.Message{Type: contracts.CommandStopAll}).Return(nil)
			},
		},
		{
			name: "returns the publish error",
			before: func() {
				guard.open()
				guard.resolve(tiers)
				guard.setPhase(model.PhaseRunning)

				mockPublisher.EXPECT().Publish(contracts.Message{Type: contracts.CommandStopAll}).Return(contracts.ErrBusClosed)
			},
			expected: contracts.ErrBusClosed,
		},
		{
			name: "is a no-op once stopping",
			before: func() {
				guard.open()
				guard.resolve(tiers)
				guard.setPhase(model.PhaseStopping)
			},
		},
		{
			name: "is a no-op once stopped",
			before: func() {
				guard.open()
				guard.resolve(tiers)
				guard.setPhase(model.PhaseStopped)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.before()

			err := control.StopAll()

			require.ErrorIs(t, err, tt.expected)
		})
	}
}

func Test_Control_StopAll_ClosesAdmissionBeforeTheHandlerRuns(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTracker := NewMockTracker(ctrl)
	mockPublisher := NewMockPublisher(ctrl)

	svc := model.Service{ID: "test-id-api", Name: "api"}
	guard := NewGuard(mockTracker)
	control := NewControl(guard, mockPublisher)

	guard.open()
	guard.resolve([]model.Tier{{Name: "platform", Services: []*model.Service{&svc}}})
	guard.dispatch(svc.ID)
	guard.release(svc.ID)
	guard.setPhase(model.PhaseRunning)

	mockPublisher.EXPECT().Publish(contracts.Message{Type: contracts.CommandStopAll}).Return(nil)

	first := control.StopAll()
	_, action := control.Restart(svc.ID)
	second := control.StopAll()

	require.NoError(t, first)
	require.ErrorIs(t, action, contracts.ErrNotAccepting)
	require.NoError(t, second)
	assert.Equal(t, model.PhaseStopping, guard.phase)
}
