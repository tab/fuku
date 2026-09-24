package tui

import (
	"errors"
	"log/slog"
	"os"
	"testing"
	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"fuku/internal/adapters/resources"
	"fuku/internal/app/services"
	"fuku/internal/contracts"
	"fuku/internal/model"
)

func Test_HandleStopKey(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockControl := NewMockControl(ctrl)

	api := model.Service{ID: "id-api", Name: "api"}
	starting := services.Admission{Service: api, Action: contracts.ActionStart, Status: model.StatusStarting}
	stopping := services.Admission{Service: api, Action: contracts.ActionStop, Status: model.StatusStopping}
	busy := contracts.ErrServiceBusy

	tests := []struct {
		name     string
		before   func() Model
		expected tea.Msg
	}{
		{
			name: "a stopped service is started",
			before: func() Model {
				mockControl.EXPECT().Start("id-api").Return(starting, nil)

				m := Model{loader: &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}, control: mockControl}
				m.state.serviceIDs = []string{"id-api"}
				m.snapshot = &model.Snapshot{Services: map[string]*model.Service{"id-api": {ID: "id-api", Name: "api", Status: model.StatusStopped}}}

				return m
			},
			expected: admissionMsg{name: "api", verb: "starting", admission: starting},
		},
		{
			name: "a failed service is started",
			before: func() Model {
				mockControl.EXPECT().Start("id-api").Return(starting, nil)

				m := Model{loader: &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}, control: mockControl}
				m.state.serviceIDs = []string{"id-api"}
				m.snapshot = &model.Snapshot{Services: map[string]*model.Service{"id-api": {ID: "id-api", Name: "api", Status: model.StatusFailed}}}

				return m
			},
			expected: admissionMsg{name: "api", verb: "starting", admission: starting},
		},
		{
			name: "a running service is stopped",
			before: func() Model {
				mockControl.EXPECT().Stop("id-api").Return(stopping, nil)

				m := Model{loader: &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}, control: mockControl}
				m.state.serviceIDs = []string{"id-api"}
				m.snapshot = &model.Snapshot{Services: map[string]*model.Service{"id-api": {ID: "id-api", Name: "api", Status: model.StatusRunning}}}

				return m
			},
			expected: admissionMsg{name: "api", verb: "stopping", admission: stopping},
		},
		{
			name: "a rejected stop answers with the error",
			before: func() Model {
				mockControl.EXPECT().Stop("id-api").Return(services.Admission{}, busy)

				m := Model{loader: &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}, control: mockControl}
				m.state.serviceIDs = []string{"id-api"}
				m.snapshot = &model.Snapshot{Services: map[string]*model.Service{"id-api": {ID: "id-api", Name: "api", Status: model.StatusRunning}}}

				return m
			},
			expected: admissionMsg{name: "api", verb: "stopping", err: busy},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tt.before()

			result, cmd := m.handleStopKey()

			assert.Equal(t, tt.expected, cmd())
			assert.False(t, result.loader.Active)
		})
	}
}

func Test_HandleStopKey_StartingServiceIsNotOffered(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockControl := NewMockControl(ctrl)

	loader := &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}

	m := Model{loader: loader, control: mockControl}
	m.state.serviceIDs = []string{"id-api"}
	m.snapshot = &model.Snapshot{Services: map[string]*model.Service{"id-api": {ID: "id-api", Name: "api", Status: model.StatusStarting}}}

	_, cmd := m.handleStopKey()

	assert.Nil(t, cmd)
}

func Test_HandleRestartKey(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockControl := NewMockControl(ctrl)

	api := model.Service{ID: "id-api", Name: "api"}
	restarting := services.Admission{Service: api, Action: contracts.ActionRestart, Status: model.StatusRestarting}
	notAllowed := contracts.ActionNotAllowedError{Action: contracts.ActionRestart}
	readyAt := time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		before   func() Model
		expected tea.Msg
	}{
		{
			name: "an admitted restart answers with the admission",
			before: func() Model {
				mockControl.EXPECT().Restart("id-api").Return(restarting, nil)

				m := Model{loader: &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}, control: mockControl}
				m.state.serviceIDs = []string{"id-api"}
				m.snapshot = &model.Snapshot{Services: map[string]*model.Service{"id-api": {ID: "id-api", Name: "api", Status: model.StatusRunning, LifecycleAt: readyAt}}}

				return m
			},
			expected: admissionMsg{name: "api", verb: "restarting", seenAt: readyAt, admission: restarting},
		},
		{
			name: "a rejected restart answers with the error",
			before: func() Model {
				mockControl.EXPECT().Restart("id-api").Return(services.Admission{}, notAllowed)

				m := Model{loader: &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}, control: mockControl}
				m.state.serviceIDs = []string{"id-api"}
				m.snapshot = &model.Snapshot{Services: map[string]*model.Service{"id-api": {ID: "id-api", Name: "api", Status: model.StatusStopped}}}

				return m
			},
			expected: admissionMsg{name: "api", verb: "restarting", err: notAllowed},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tt.before()

			result, cmd := m.handleRestartKey()

			assert.Equal(t, tt.expected, cmd())
			assert.False(t, result.loader.Active)
		})
	}
}

func Test_HandleQuitKey(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockControl := NewMockControl(ctrl)

	tests := []struct {
		name     string
		before   func() Model
		expected tea.Msg
	}{
		{
			name: "an accepted stop all answers without an error",
			before: func() Model {
				mockControl.EXPECT().StopAll().Return(nil)

				return Model{loader: &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}, control: mockControl}
			},
			expected: stopAllMsg{},
		},
		{
			name: "a rejected stop all answers with the error",
			before: func() Model {
				mockControl.EXPECT().StopAll().Return(contracts.ErrNotAccepting)

				return Model{loader: &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}, control: mockControl}
			},
			expected: stopAllMsg{err: contracts.ErrNotAccepting},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tt.before()

			result, cmd := m.handleQuitKey()

			assert.Equal(t, tt.expected, cmd())
			assert.False(t, result.state.shuttingDown)
			assert.False(t, result.loader.Active)
		})
	}
}

func Test_HandleAdmission(t *testing.T) {
	log := slog.New(slog.DiscardHandler)

	api := model.Service{ID: "id-api", Name: "api"}
	starting := services.Admission{Service: api, Action: contracts.ActionStart, Status: model.StatusStarting}
	restarting := services.Admission{Service: api, Action: contracts.ActionRestart, Status: model.StatusRestarting}
	pressedAt := time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	eventAt := pressedAt.Add(time.Second)

	tests := []struct {
		name         string
		before       func() Model
		msg          admissionMsg
		expectLoader string
	}{
		{
			name: "an admission before any event starts the service loader",
			before: func() Model {
				m := Model{loader: &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}, log: log}
				m.snapshot = &model.Snapshot{Services: map[string]*model.Service{"id-api": {ID: "id-api", Name: "api", Status: model.StatusStopped, LifecycleAt: pressedAt}}}

				return m
			},
			msg:          admissionMsg{name: "api", verb: "starting", seenAt: pressedAt, admission: starting},
			expectLoader: "starting api…",
		},
		{
			name: "an admission after the service settled starts nothing",
			before: func() Model {
				m := Model{loader: &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}, log: log}
				m.snapshot = &model.Snapshot{Services: map[string]*model.Service{"id-api": {ID: "id-api", Name: "api", Status: model.StatusRunning, LifecycleAt: eventAt}}}

				return m
			},
			msg: admissionMsg{name: "api", verb: "starting", seenAt: pressedAt, admission: starting},
		},
		{
			name: "an admission after a restarting event leaves the loader the event started",
			before: func() Model {
				m := Model{loader: &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}, log: log}
				m.snapshot = &model.Snapshot{Services: map[string]*model.Service{"id-api": {ID: "id-api", Name: "api", Status: model.StatusRestarting, LifecycleAt: eventAt}}}
				m.loader.Start("id-api", "restarting api…")

				return m
			},
			msg:          admissionMsg{name: "api", verb: "restarting", seenAt: pressedAt, admission: restarting},
			expectLoader: "restarting api…",
		},
		{
			name: "a rejected action leaves the loader idle",
			before: func() Model {
				m := Model{loader: &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}, log: log}
				m.snapshot = &model.Snapshot{Services: map[string]*model.Service{"id-api": {ID: "id-api", Name: "api", Status: model.StatusRunning, LifecycleAt: pressedAt}}}

				return m
			},
			msg: admissionMsg{name: "api", verb: "stopping", seenAt: pressedAt, err: contracts.ErrServiceBusy},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tt.before()

			result, _ := m.handleAdmission(tt.msg)

			assert.Equal(t, tt.expectLoader, result.loader.Message())
		})
	}
}

func Test_HandleStopAll(t *testing.T) {
	log := slog.New(slog.DiscardHandler)

	tests := []struct {
		name         string
		before       func() Model
		msg          stopAllMsg
		shuttingDown bool
	}{
		{
			name: "an accepted stop all starts the shutdown",
			before: func() Model {
				return Model{loader: &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}, log: log}
			},
			msg:          stopAllMsg{},
			shuttingDown: true,
		},
		{
			name: "a rejected stop all keeps the view interactive",
			before: func() Model {
				return Model{loader: &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}, log: log}
			},
			msg: stopAllMsg{err: contracts.ErrNotAccepting},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tt.before()

			result, cmd := m.handleStopAll(tt.msg)

			assert.Equal(t, tt.shuttingDown, result.state.shuttingDown)
			assert.Equal(t, tt.shuttingDown, result.loader.Has(loaderKeyShutdown))
			assert.Nil(t, cmd)
		})
	}
}

func Test_SampleAppStatsCmd(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockMonitor := NewMockMonitor(ctrl)

	m := Model{ctx: t.Context(), monitor: mockMonitor}

	errStats := errors.New("stats unavailable")

	tests := []struct {
		name     string
		before   func()
		expected tea.Msg
	}{
		{
			name: "a sample carries the usage of the fuku process",
			before: func() {
				mockMonitor.EXPECT().GetStats(gomock.Any(), os.Getpid()).Return(resources.Stats{CPU: 1.5, MEM: 64}, nil)
			},
			expected: appStatsMsg{cpu: 1.5, mem: 64},
		},
		{
			name: "a failed sample reports no usage",
			before: func() {
				mockMonitor.EXPECT().GetStats(gomock.Any(), os.Getpid()).Return(resources.Stats{}, errStats)
			},
			expected: appStatsMsg{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.before()

			result := m.sampleAppStatsCmd()()

			assert.Equal(t, tt.expected, result)
		})
	}
}
