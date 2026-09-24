package registry

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"fuku/internal/contracts"
	"fuku/internal/model"
)

func Test_Store_update(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSubscriber := NewMockSubscriber(ctrl)
	mockPublisher := NewMockPublisher(ctrl)

	s := NewStore(mockSubscriber, mockPublisher)

	changed := contracts.Message{Type: contracts.EventSnapshotChanged, Data: contracts.SnapshotChanged{}}
	rename := func(snapshot *model.Snapshot) bool {
		snapshot.Profile = "renamed"

		return true
	}
	keep := func(*model.Snapshot) bool {
		return false
	}

	tests := []struct {
		name     string
		before   func()
		fn       func(*model.Snapshot) bool
		expected string
	}{
		{
			name: "a change is applied and announced",
			before: func() {
				mockPublisher.EXPECT().Publish(changed).Return(nil)
			},
			fn:       rename,
			expected: "renamed",
		},
		{
			name:     "no change announces nothing",
			before:   func() {},
			fn:       keep,
			expected: "renamed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.before()

			s.update(tt.fn)

			assert.Equal(t, tt.expected, s.snapshot.Profile)
			assert.True(t, ctrl.Satisfied())
		})
	}
}

func Test_Store_update_AnnouncesOnceTheLockIsReleased(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSubscriber := NewMockSubscriber(ctrl)
	mockPublisher := NewMockPublisher(ctrl)

	s := NewStore(mockSubscriber, mockPublisher)

	var readable string

	readProfile := func(snapshot *model.Snapshot) {
		readable = snapshot.Profile
	}
	capture := func(contracts.Message) error {
		s.Read(readProfile)

		return nil
	}
	rename := func(snapshot *model.Snapshot) bool {
		snapshot.Profile = "default"

		return true
	}

	mockPublisher.EXPECT().Publish(gomock.Any()).DoAndReturn(capture)

	s.update(rename)

	assert.Equal(t, "default", readable, "the notification must not go out before its change is readable")
}

func Test_Store_ProfileResolved(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSubscriber := NewMockSubscriber(ctrl)
	mockPublisher := NewMockPublisher(ctrl)
	mockPublisher.EXPECT().Publish(gomock.Any()).Return(nil)

	s := NewStore(mockSubscriber, mockPublisher)

	db := &model.Service{ID: "test-id-db", Name: "db", Tier: "foundation"}
	cache := &model.Service{ID: "test-id-cache", Name: "cache", Tier: "foundation"}
	api := &model.Service{ID: "test-id-api", Name: "api", Tier: "application"}
	web := &model.Service{ID: "test-id-web", Name: "web", Tier: "application"}

	resolved := contracts.Message{
		Type: contracts.EventProfileResolved,
		Data: contracts.ProfileResolved{
			Profile: "default",
			Tiers: []model.Tier{
				{ID: "test-tier-foundation", Name: "foundation", Services: []*model.Service{db, cache}},
				{ID: "test-tier-application", Name: "application", Services: []*model.Service{api, web}},
			},
		},
	}

	s.handle(resolved)

	assert.Equal(t, "default", s.snapshot.Profile)
	assert.True(t, s.snapshot.Resolved)
	assert.Equal(t, []*model.Tier{
		{ID: "test-tier-foundation", Name: "foundation", Services: []*model.Service{
			{ID: "test-id-db", Name: "db", Tier: "foundation", Status: model.StatusPending},
			{ID: "test-id-cache", Name: "cache", Tier: "foundation", Status: model.StatusPending},
		}},
		{ID: "test-tier-application", Name: "application", Services: []*model.Service{
			{ID: "test-id-api", Name: "api", Tier: "application", Status: model.StatusPending},
			{ID: "test-id-web", Name: "web", Tier: "application", Status: model.StatusPending},
		}},
	}, s.snapshot.Tiers)
	require.Len(t, s.snapshot.Services, 4)
	assert.Same(t, s.snapshot.Tiers[0].Services[1], s.snapshot.Services["test-id-cache"], "a tier and the map share one service")
	assert.NotSame(t, cache, s.snapshot.Services["test-id-cache"], "the store owns its services")
	assert.Empty(t, cache.Status, "the payload is never written")
}

func Test_Store_PhaseTransitions(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSubscriber := NewMockSubscriber(ctrl)
	mockPublisher := NewMockPublisher(ctrl)
	mockPublisher.EXPECT().Publish(gomock.Any()).Return(nil).AnyTimes()

	s := NewStore(mockSubscriber, mockPublisher)

	startedAt := time.Now()

	tests := []struct {
		name              string
		msg               contracts.Message
		expectedPhase     model.Phase
		expectedStartedAt time.Time
	}{
		{
			name: "startup starts the uptime clock",
			msg: contracts.Message{
				Timestamp: startedAt,
				Type:      contracts.EventPhaseChanged,
				Data:      contracts.PhaseChanged{Phase: model.PhaseStartup},
			},
			expectedPhase:     model.PhaseStartup,
			expectedStartedAt: startedAt,
		},
		{
			name: "running",
			msg: contracts.Message{
				Type: contracts.EventPhaseChanged,
				Data: contracts.PhaseChanged{Phase: model.PhaseRunning},
			},
			expectedPhase:     model.PhaseRunning,
			expectedStartedAt: startedAt,
		},
		{
			name: "stopping",
			msg: contracts.Message{
				Type: contracts.EventPhaseChanged,
				Data: contracts.PhaseChanged{Phase: model.PhaseStopping},
			},
			expectedPhase:     model.PhaseStopping,
			expectedStartedAt: startedAt,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s.handle(tt.msg)

			assert.Equal(t, tt.expectedPhase, s.snapshot.Phase)
			assert.Equal(t, tt.expectedStartedAt, s.snapshot.StartedAt)
		})
	}
}

func Test_Store_TierReadiness(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSubscriber := NewMockSubscriber(ctrl)
	mockPublisher := NewMockPublisher(ctrl)
	mockPublisher.EXPECT().Publish(gomock.Any()).Return(nil)

	s := NewStore(mockSubscriber, mockPublisher)

	changed := contracts.Message{Type: contracts.EventSnapshotChanged, Data: contracts.SnapshotChanged{}}

	s.handle(contracts.Message{
		Type: contracts.EventProfileResolved,
		Data: contracts.ProfileResolved{
			Profile: "default",
			Tiers: []model.Tier{
				{Name: "foundation", Services: []*model.Service{{ID: "test-id-db", Name: "db"}}},
				{Name: "application", Services: []*model.Service{{ID: "test-id-api", Name: "api"}}},
			},
		},
	})

	tests := []struct {
		name     string
		before   func()
		msg      contracts.Message
		expected []bool
	}{
		{
			name:   "a tier starts not ready",
			before: func() {},
			msg: contracts.Message{
				Type: contracts.EventTierStarting,
				Data: contracts.TierStarting{Name: "foundation"},
			},
			expected: []bool{false, false},
		},
		{
			name: "a ready tier",
			before: func() {
				mockPublisher.EXPECT().Publish(changed).Return(nil)
			},
			msg: contracts.Message{
				Type: contracts.EventTierReady,
				Data: contracts.TierReady{Name: "foundation"},
			},
			expected: []bool{true, false},
		},
		{
			name:   "an unknown tier changes nothing",
			before: func() {},
			msg: contracts.Message{
				Type: contracts.EventTierReady,
				Data: contracts.TierReady{Name: "unknown"},
			},
			expected: []bool{true, false},
		},
		{
			name: "a restarted tier is not ready",
			before: func() {
				mockPublisher.EXPECT().Publish(changed).Return(nil)
			},
			msg: contracts.Message{
				Type: contracts.EventTierStarting,
				Data: contracts.TierStarting{Name: "foundation"},
			},
			expected: []bool{false, false},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.before()

			s.handle(tt.msg)

			assert.Equal(t, tt.expected, []bool{s.snapshot.Tiers[0].Ready, s.snapshot.Tiers[1].Ready})
			assert.True(t, ctrl.Satisfied())
		})
	}
}

func Test_Store_API(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSubscriber := NewMockSubscriber(ctrl)
	mockPublisher := NewMockPublisher(ctrl)

	s := NewStore(mockSubscriber, mockPublisher)

	changed := contracts.Message{Type: contracts.EventSnapshotChanged, Data: contracts.SnapshotChanged{}}

	tests := []struct {
		name     string
		before   func()
		msg      contracts.Message
		expected model.API
	}{
		{
			name:   "stopped before started is a no-op",
			before: func() {},
			msg: contracts.Message{
				Type: contracts.EventAPIStopped,
				Data: contracts.APIStopped{},
			},
			expected: model.API{},
		},
		{
			name: "started records the bound address",
			before: func() {
				mockPublisher.EXPECT().Publish(changed).Return(nil)
			},
			msg: contracts.Message{
				Type: contracts.EventAPIStarted,
				Data: contracts.APIStarted{Listen: "127.0.0.1:9876"},
			},
			expected: model.API{Listening: true, Address: "127.0.0.1:9876"},
		},
		{
			name:   "the same address again is a no-op",
			before: func() {},
			msg: contracts.Message{
				Type: contracts.EventAPIStarted,
				Data: contracts.APIStarted{Listen: "127.0.0.1:9876"},
			},
			expected: model.API{Listening: true, Address: "127.0.0.1:9876"},
		},
		{
			name: "stopped clears the listener",
			before: func() {
				mockPublisher.EXPECT().Publish(changed).Return(nil)
			},
			msg: contracts.Message{
				Type: contracts.EventAPIStopped,
				Data: contracts.APIStopped{},
			},
			expected: model.API{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.before()

			s.handle(tt.msg)

			assert.Equal(t, tt.expected, s.snapshot.API)
			assert.True(t, ctrl.Satisfied())
		})
	}
}

func Test_Store_ServiceLifecycle(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSubscriber := NewMockSubscriber(ctrl)
	mockPublisher := NewMockPublisher(ctrl)
	mockPublisher.EXPECT().Publish(gomock.Any()).Return(nil).AnyTimes()

	s := NewStore(mockSubscriber, mockPublisher)

	api := contracts.ServiceEvent{Service: model.Service{ID: "test-id-api", Name: "api"}, Tier: "foundation"}

	s.handle(contracts.Message{
		Type: contracts.EventProfileResolved,
		Data: contracts.ProfileResolved{
			Profile: "default",
			Tiers:   []model.Tier{{Name: "foundation", Services: []*model.Service{&api.Service}}},
		},
	})

	tests := []struct {
		name           string
		msg            contracts.Message
		expectedStatus model.Status
		expectedPID    int
	}{
		{
			name: "starting records the pid",
			msg: contracts.Message{
				Type: contracts.EventServiceStarting,
				Data: contracts.ServiceStarting{ServiceEvent: api, PID: 1234},
			},
			expectedStatus: model.StatusStarting,
			expectedPID:    1234,
		},
		{
			name: "ready",
			msg: contracts.Message{
				Type: contracts.EventServiceReady,
				Data: contracts.ServiceReady{ServiceEvent: api, PID: 1234},
			},
			expectedStatus: model.StatusRunning,
			expectedPID:    1234,
		},
		{
			name: "stopping",
			msg: contracts.Message{
				Type: contracts.EventServiceStopping,
				Data: contracts.ServiceStopping{ServiceEvent: api},
			},
			expectedStatus: model.StatusStopping,
			expectedPID:    1234,
		},
		{
			name: "stopped clears the pid",
			msg: contracts.Message{
				Type: contracts.EventServiceStopped,
				Data: contracts.ServiceStopped{ServiceEvent: api},
			},
			expectedStatus: model.StatusStopped,
			expectedPID:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s.handle(tt.msg)

			svc, found := s.snapshot.Services["test-id-api"]

			require.True(t, found)
			assert.Equal(t, tt.expectedStatus, svc.Status)
			assert.Equal(t, tt.expectedPID, svc.Process.PID)
		})
	}
}

func Test_Store_ServiceFailed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSubscriber := NewMockSubscriber(ctrl)
	mockPublisher := NewMockPublisher(ctrl)
	mockPublisher.EXPECT().Publish(gomock.Any()).Return(nil).AnyTimes()

	s := NewStore(mockSubscriber, mockPublisher)

	api := contracts.ServiceEvent{Service: model.Service{ID: "test-id-api", Name: "api"}, Tier: "foundation"}
	startedAt := time.Now()

	s.handle(contracts.Message{
		Type: contracts.EventProfileResolved,
		Data: contracts.ProfileResolved{
			Profile: "default",
			Tiers:   []model.Tier{{Name: "foundation", Services: []*model.Service{&api.Service}}},
		},
	})
	s.handle(contracts.Message{
		Type: contracts.EventServiceStarting,
		Data: contracts.ServiceStarting{ServiceEvent: api, PID: 5678, StartedAt: startedAt},
	})
	s.handle(contracts.Message{
		Type: contracts.EventServiceFailed,
		Data: contracts.ServiceFailed{ServiceEvent: api, Error: errors.New("readiness timeout")},
	})

	svc, found := s.snapshot.Services["test-id-api"]

	require.True(t, found)
	assert.Equal(t, model.StatusFailed, svc.Status)
	assert.Equal(t, "readiness timeout", svc.Error)
	assert.Equal(t, 0, svc.Process.PID)
	assert.True(t, svc.Process.StartedAt.IsZero())
	assert.Equal(t, startedAt, svc.AttemptedAt)
}

func Test_Store_Counts(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSubscriber := NewMockSubscriber(ctrl)
	mockPublisher := NewMockPublisher(ctrl)
	mockPublisher.EXPECT().Publish(gomock.Any()).Return(nil).AnyTimes()

	s := NewStore(mockSubscriber, mockPublisher)

	api := contracts.ServiceEvent{Service: model.Service{ID: "test-id-api", Name: "api"}, Tier: "foundation"}
	db := contracts.ServiceEvent{Service: model.Service{ID: "test-id-db", Name: "db"}, Tier: "foundation"}

	tests := []struct {
		name     string
		msg      contracts.Message
		expected model.Counts
	}{
		{
			name: "resolved services are pending",
			msg: contracts.Message{
				Type: contracts.EventProfileResolved,
				Data: contracts.ProfileResolved{
					Profile: "default",
					Tiers:   []model.Tier{{Name: "foundation", Services: []*model.Service{&api.Service, &db.Service}}},
				},
			},
			expected: model.Counts{Total: 2, Pending: 2},
		},
		{
			name: "starting service",
			msg: contracts.Message{
				Type: contracts.EventServiceStarting,
				Data: contracts.ServiceStarting{ServiceEvent: api},
			},
			expected: model.Counts{Total: 2, Pending: 1, Starting: 1},
		},
		{
			name: "ready service is running",
			msg: contracts.Message{
				Type: contracts.EventServiceReady,
				Data: contracts.ServiceReady{ServiceEvent: api},
			},
			expected: model.Counts{Total: 2, Pending: 1, Running: 1},
		},
		{
			name: "restarting leaves running",
			msg: contracts.Message{
				Type: contracts.EventServiceRestarting,
				Data: contracts.ServiceRestarting{ServiceEvent: api},
			},
			expected: model.Counts{Total: 2, Pending: 1, Restarting: 1},
		},
		{
			name: "stopping leaves restarting",
			msg: contracts.Message{
				Type: contracts.EventServiceStopping,
				Data: contracts.ServiceStopping{ServiceEvent: api},
			},
			expected: model.Counts{Total: 2, Pending: 1, Stopping: 1},
		},
		{
			name: "failed service",
			msg: contracts.Message{
				Type: contracts.EventServiceFailed,
				Data: contracts.ServiceFailed{ServiceEvent: db},
			},
			expected: model.Counts{Total: 2, Stopping: 1, Failed: 1},
		},
		{
			name: "stopped service",
			msg: contracts.Message{
				Type: contracts.EventServiceStopped,
				Data: contracts.ServiceStopped{ServiceEvent: api},
			},
			expected: model.Counts{Total: 2, Stopped: 1, Failed: 1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s.handle(tt.msg)

			assert.Equal(t, tt.expected, s.snapshot.Counts())
		})
	}
}

func Test_Store_Watching(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSubscriber := NewMockSubscriber(ctrl)
	mockPublisher := NewMockPublisher(ctrl)
	mockPublisher.EXPECT().Publish(gomock.Any()).Return(nil).AnyTimes()

	s := NewStore(mockSubscriber, mockPublisher)

	api := model.Service{ID: "test-id-api", Name: "api"}

	s.handle(contracts.Message{
		Type: contracts.EventProfileResolved,
		Data: contracts.ProfileResolved{
			Profile: "default",
			Tiers:   []model.Tier{{Name: "foundation", Services: []*model.Service{&api}}},
		},
	})

	tests := []struct {
		name     string
		msg      contracts.Message
		expected bool
	}{
		{
			name: "watch started",
			msg: contracts.Message{
				Type: contracts.EventWatchStarted,
				Data: contracts.WatchStarted{Service: api},
			},
			expected: true,
		},
		{
			name: "watch stopped",
			msg: contracts.Message{
				Type: contracts.EventWatchStopped,
				Data: contracts.WatchStopped{Service: api},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s.handle(tt.msg)

			svc, found := s.snapshot.Services["test-id-api"]

			require.True(t, found)
			assert.Equal(t, tt.expected, svc.Watching)
		})
	}
}

func Test_Store_ServiceRestarting(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSubscriber := NewMockSubscriber(ctrl)
	mockPublisher := NewMockPublisher(ctrl)
	mockPublisher.EXPECT().Publish(gomock.Any()).Return(nil).AnyTimes()

	s := NewStore(mockSubscriber, mockPublisher)

	api := contracts.ServiceEvent{Service: model.Service{ID: "test-id-api", Name: "api"}, Tier: "foundation"}
	startedAt := time.Now()

	s.handle(contracts.Message{
		Type: contracts.EventProfileResolved,
		Data: contracts.ProfileResolved{
			Profile: "default",
			Tiers:   []model.Tier{{Name: "foundation", Services: []*model.Service{&api.Service}}},
		},
	})
	s.handle(contracts.Message{
		Type: contracts.EventServiceStarting,
		Data: contracts.ServiceStarting{ServiceEvent: api, PID: 1234, StartedAt: startedAt},
	})
	s.handle(contracts.Message{
		Type: contracts.EventServiceReady,
		Data: contracts.ServiceReady{ServiceEvent: api, PID: 1234, StartedAt: startedAt},
	})
	s.handle(contracts.Message{
		Type: contracts.EventServiceResourcesSampled,
		Data: contracts.ServiceResourcesSampled{Services: []contracts.ServiceResourceSample{{ID: "test-id-api", PID: 1234, CPU: 2.5, Memory: 4096}}},
	})

	running := *s.snapshot.Services["test-id-api"]

	s.handle(contracts.Message{
		Type: contracts.EventServiceRestarting,
		Data: contracts.ServiceRestarting{ServiceEvent: api},
	})

	restarting := *s.snapshot.Services["test-id-api"]

	assert.Equal(t, model.StatusRunning, running.Status)
	assert.Equal(t, startedAt, running.Process.StartedAt)
	assert.Equal(t, startedAt, running.AttemptedAt)
	assert.InDelta(t, 2.5, running.Process.CPU, 0)
	assert.Equal(t, uint64(4096), running.Process.Memory)
	assert.Equal(t, model.StatusRestarting, restarting.Status)
	assert.Equal(t, 1234, restarting.Process.PID, "PID must be kept during restart")
	assert.True(t, restarting.Process.StartedAt.IsZero(), "StartedAt must be cleared during restart")
	assert.Equal(t, startedAt, restarting.AttemptedAt, "AttemptedAt must be preserved during restart")
	assert.InDelta(t, 0, restarting.Process.CPU, 0)
	assert.Equal(t, uint64(0), restarting.Process.Memory)
}

func Test_Store_ServiceReady_KeepsUsageOfTheSameProcess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSubscriber := NewMockSubscriber(ctrl)
	mockPublisher := NewMockPublisher(ctrl)
	mockPublisher.EXPECT().Publish(gomock.Any()).Return(nil).AnyTimes()

	s := NewStore(mockSubscriber, mockPublisher)

	api := contracts.ServiceEvent{Service: model.Service{ID: "test-id-api", Name: "api"}, Tier: "foundation"}
	startedAt := time.Now()

	s.handle(contracts.Message{
		Type: contracts.EventProfileResolved,
		Data: contracts.ProfileResolved{
			Profile: "default",
			Tiers:   []model.Tier{{Name: "foundation", Services: []*model.Service{&api.Service}}},
		},
	})
	s.handle(contracts.Message{
		Type: contracts.EventServiceStarting,
		Data: contracts.ServiceStarting{ServiceEvent: api, PID: 1234, StartedAt: startedAt},
	})
	s.handle(contracts.Message{
		Type: contracts.EventServiceResourcesSampled,
		Data: contracts.ServiceResourcesSampled{Services: []contracts.ServiceResourceSample{{ID: "test-id-api", PID: 1234, CPU: 2.5, Memory: 4096}}},
	})
	s.handle(contracts.Message{
		Type: contracts.EventServiceReady,
		Data: contracts.ServiceReady{ServiceEvent: api, PID: 1234, StartedAt: startedAt},
	})

	svc, found := s.snapshot.Services["test-id-api"]

	require.True(t, found)
	assert.Equal(t, model.StatusRunning, svc.Status)
	assert.InDelta(t, 2.5, svc.Process.CPU, 0)
	assert.Equal(t, uint64(4096), svc.Process.Memory)
}

func Test_Store_UnknownService(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSubscriber := NewMockSubscriber(ctrl)
	mockPublisher := NewMockPublisher(ctrl)
	mockPublisher.EXPECT().Publish(gomock.Any()).Return(nil)

	s := NewStore(mockSubscriber, mockPublisher)

	api := contracts.ServiceEvent{Service: model.Service{ID: "test-id-api", Name: "api"}, Tier: "foundation"}
	other := contracts.ServiceEvent{Service: model.Service{ID: "test-id-other", Name: "other"}, Tier: "foundation"}

	s.handle(contracts.Message{
		Type: contracts.EventProfileResolved,
		Data: contracts.ProfileResolved{
			Profile: "default",
			Tiers:   []model.Tier{{Name: "foundation", Services: []*model.Service{&api.Service}}},
		},
	})

	tests := []struct {
		name string
		msg  contracts.Message
	}{
		{
			name: "starting",
			msg: contracts.Message{
				Type: contracts.EventServiceStarting,
				Data: contracts.ServiceStarting{ServiceEvent: other, PID: 1234},
			},
		},
		{
			name: "stopping",
			msg: contracts.Message{
				Type: contracts.EventServiceStopping,
				Data: contracts.ServiceStopping{ServiceEvent: other},
			},
		},
		{
			name: "ready",
			msg: contracts.Message{
				Type: contracts.EventServiceReady,
				Data: contracts.ServiceReady{ServiceEvent: other, PID: 1234},
			},
		},
		{
			name: "failed",
			msg: contracts.Message{
				Type: contracts.EventServiceFailed,
				Data: contracts.ServiceFailed{ServiceEvent: other},
			},
		},
		{
			name: "stopped",
			msg: contracts.Message{
				Type: contracts.EventServiceStopped,
				Data: contracts.ServiceStopped{ServiceEvent: other},
			},
		},
		{
			name: "restarting",
			msg: contracts.Message{
				Type: contracts.EventServiceRestarting,
				Data: contracts.ServiceRestarting{ServiceEvent: other},
			},
		},
		{
			name: "watch started",
			msg: contracts.Message{
				Type: contracts.EventWatchStarted,
				Data: contracts.WatchStarted{Service: other.Service},
			},
		},
		{
			name: "resource sample",
			msg: contracts.Message{
				Type: contracts.EventServiceResourcesSampled,
				Data: contracts.ServiceResourcesSampled{Services: []contracts.ServiceResourceSample{{ID: "test-id-other", PID: 1234, CPU: 1}}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s.handle(tt.msg)

			assert.Len(t, s.snapshot.Services, 1)
			assert.Equal(t, model.StatusPending, s.snapshot.Services["test-id-api"].Status)
		})
	}
}

func Test_Store_apply_ForeignPayload(t *testing.T) {
	s := NewStore(nil, nil)

	snapshot := &model.Snapshot{}

	foreign := "not an event payload"

	tests := []struct {
		name string
		msg  contracts.Message
	}{
		{
			name: "profile resolved",
			msg:  contracts.Message{Type: contracts.EventProfileResolved, Data: foreign},
		},
		{
			name: "phase changed",
			msg:  contracts.Message{Type: contracts.EventPhaseChanged, Data: foreign},
		},
		{
			name: "tier starting",
			msg:  contracts.Message{Type: contracts.EventTierStarting, Data: foreign},
		},
		{
			name: "tier ready",
			msg:  contracts.Message{Type: contracts.EventTierReady, Data: foreign},
		},
		{
			name: "service starting",
			msg:  contracts.Message{Type: contracts.EventServiceStarting, Data: foreign},
		},
		{
			name: "service ready",
			msg:  contracts.Message{Type: contracts.EventServiceReady, Data: foreign},
		},
		{
			name: "service failed",
			msg:  contracts.Message{Type: contracts.EventServiceFailed, Data: foreign},
		},
		{
			name: "service stopping",
			msg:  contracts.Message{Type: contracts.EventServiceStopping, Data: foreign},
		},
		{
			name: "service stopped",
			msg:  contracts.Message{Type: contracts.EventServiceStopped, Data: foreign},
		},
		{
			name: "service restarting",
			msg:  contracts.Message{Type: contracts.EventServiceRestarting, Data: foreign},
		},
		{
			name: "watch started",
			msg:  contracts.Message{Type: contracts.EventWatchStarted, Data: foreign},
		},
		{
			name: "API started",
			msg:  contracts.Message{Type: contracts.EventAPIStarted, Data: foreign},
		},
		{
			name: "resources sampled",
			msg:  contracts.Message{Type: contracts.EventServiceResourcesSampled, Data: foreign},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			changed := s.apply(snapshot, tt.msg)

			assert.False(t, changed)
			assert.Equal(t, &model.Snapshot{}, snapshot)
		})
	}
}

func Test_Store_NewAttempt(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSubscriber := NewMockSubscriber(ctrl)
	mockPublisher := NewMockPublisher(ctrl)
	mockPublisher.EXPECT().Publish(gomock.Any()).Return(nil).AnyTimes()

	api := contracts.ServiceEvent{Service: model.Service{ID: "test-id-api", Name: "api", Tier: "foundation"}, Tier: "foundation"}
	first := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	second := first.Add(10 * time.Second)

	resolved := contracts.Message{
		Type: contracts.EventProfileResolved,
		Data: contracts.ProfileResolved{
			Profile: "default",
			Tiers:   []model.Tier{{Name: "foundation", Services: []*model.Service{&api.Service}}},
		},
	}

	tests := []struct {
		name     string
		before   func() *Store
		messages []contracts.Message
		expected model.Service
	}{
		{
			name: "ready of a new process drops the usage sampled for the old one",
			before: func() *Store {
				s := NewStore(mockSubscriber, mockPublisher)
				s.handle(resolved)

				return s
			},
			messages: []contracts.Message{
				{Type: contracts.EventServiceStarting, Data: contracts.ServiceStarting{ServiceEvent: api, PID: 1234, StartedAt: first}},
				{Type: contracts.EventServiceReady, Data: contracts.ServiceReady{ServiceEvent: api, PID: 1234, StartedAt: first}},
				{Type: contracts.EventServiceResourcesSampled, Data: contracts.ServiceResourcesSampled{Services: []contracts.ServiceResourceSample{{ID: "test-id-api", PID: 1234, CPU: 2.5, Memory: 4096}}}},
				{Type: contracts.EventServiceReady, Data: contracts.ServiceReady{ServiceEvent: api, PID: 5678, StartedAt: second}},
			},
			expected: model.Service{
				ID: "test-id-api", Name: "api", Tier: "foundation", Status: model.StatusRunning,
				Process: model.Process{PID: 5678, StartedAt: second}, AttemptedAt: second,
			},
		},
		{
			name: "starting after a failure clears the error and the usage",
			before: func() *Store {
				s := NewStore(mockSubscriber, mockPublisher)
				s.handle(resolved)

				return s
			},
			messages: []contracts.Message{
				{Type: contracts.EventServiceStarting, Data: contracts.ServiceStarting{ServiceEvent: api, PID: 1234, StartedAt: first}},
				{Type: contracts.EventServiceFailed, Data: contracts.ServiceFailed{ServiceEvent: api, Error: errors.New("readiness timeout")}},
				{Type: contracts.EventServiceStarting, Data: contracts.ServiceStarting{ServiceEvent: api, PID: 5678, StartedAt: second}},
			},
			expected: model.Service{
				ID: "test-id-api", Name: "api", Tier: "foundation", Status: model.StatusStarting,
				Process: model.Process{PID: 5678, StartedAt: second}, AttemptedAt: second,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := tt.before()

			for _, msg := range tt.messages {
				s.handle(msg)
			}

			svc, found := s.snapshot.Services["test-id-api"]

			require.True(t, found)
			assert.Equal(t, tt.expected, *svc)
		})
	}
}
