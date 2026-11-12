package registry

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"fuku/internal/contracts"
	"fuku/internal/model"
)

func Test_Store_Read(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSubscriber := NewMockSubscriber(ctrl)
	mockPublisher := NewMockPublisher(ctrl)

	s := NewStore(mockSubscriber, mockPublisher)

	resolved := contracts.Message{
		Type: contracts.EventProfileResolved,
		Data: contracts.ProfileResolved{
			Profile: "default",
			Tiers:   []model.Tier{{Name: "foundation", Services: []*model.Service{{ID: "test-id-api", Name: "api"}}}},
		},
	}

	var (
		profile  string
		services int
	)

	readFacts := func(snapshot *model.Snapshot) {
		profile = snapshot.Profile
		services = len(snapshot.Services)
	}

	tests := []struct {
		name     string
		before   func()
		expected string
		services int
	}{
		{
			name:     "the empty snapshot before the profile resolves",
			before:   func() {},
			expected: "",
			services: 0,
		},
		{
			name: "the resolved profile",
			before: func() {
				mockPublisher.EXPECT().Publish(gomock.Any()).Return(nil)

				s.handle(resolved)
			},
			expected: "default",
			services: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.before()

			s.Read(readFacts)

			assert.Equal(t, tt.expected, profile)
			assert.Equal(t, tt.services, services)
		})
	}
}

func Test_Store_Read_ConcurrentUpdates(t *testing.T) {
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
	s.handle(contracts.Message{
		Type: contracts.EventServiceStarting,
		Data: contracts.ServiceStarting{ServiceEvent: api, PID: 1234},
	})

	var wg sync.WaitGroup

	observed := make(chan float64, 50)

	sample := func(cpu float64) {
		defer wg.Done()

		s.handle(contracts.Message{
			Type: contracts.EventServiceResourcesSampled,
			Data: contracts.ServiceResourcesSampled{Services: []contracts.ServiceResourceSample{{ID: "test-id-api", PID: 1234, CPU: cpu}}},
		})
	}
	readCPU := func(snapshot *model.Snapshot) {
		observed <- snapshot.Services["test-id-api"].Process.CPU
	}
	read := func() {
		defer wg.Done()

		s.Read(readCPU)
	}

	for cpu := 3.0; cpu < 53; cpu++ {
		wg.Add(2)

		go sample(cpu)
		go read()
	}

	wg.Wait()
	close(observed)

	assert.Len(t, observed, 50)
	assert.GreaterOrEqual(t, s.snapshot.Services["test-id-api"].Process.CPU, 3.0)
	assert.Less(t, s.snapshot.Services["test-id-api"].Process.CPU, 53.0)
}

func Test_Store_WaitResolved(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSubscriber := NewMockSubscriber(ctrl)
	mockPublisher := NewMockPublisher(ctrl)

	s := NewStore(mockSubscriber, mockPublisher)

	resolved := contracts.Message{
		Type: contracts.EventProfileResolved,
		Data: contracts.ProfileResolved{
			Profile: "default",
			Tiers:   []model.Tier{{Name: "foundation", Services: []*model.Service{{ID: "test-id-api", Name: "api"}}}},
		},
	}

	tests := []struct {
		name     string
		before   func() context.Context
		expected bool
	}{
		{
			name: "returns once the context is cancelled",
			before: func() context.Context {
				ctx, cancel := context.WithCancel(t.Context())
				cancel()

				return ctx
			},
			expected: false,
		},
		{
			name: "returns once the profile is resolved",
			before: func() context.Context {
				mockPublisher.EXPECT().Publish(gomock.Any()).Return(nil)

				s.handle(resolved)

				return t.Context()
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := tt.before()

			s.WaitResolved(ctx)

			assert.Equal(t, tt.expected, s.snapshot.Resolved)
		})
	}
}

func Test_Store_WaitSettled(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSubscriber := NewMockSubscriber(ctrl)
	mockPublisher := NewMockPublisher(ctrl)
	mockPublisher.EXPECT().Publish(gomock.Any()).Return(nil).AnyTimes()

	since := time.Now()
	before := since.Add(-time.Second)
	after := since.Add(time.Second)

	api := &model.Service{ID: "test-id-api", Name: "api"}
	resolved := contracts.Message{
		Type: contracts.EventProfileResolved,
		Data: contracts.ProfileResolved{Profile: "default", Tiers: []model.Tier{{Name: "foundation", Services: []*model.Service{api}}}},
	}
	ready := func(at time.Time) contracts.Message {
		return contracts.Message{
			Type:      contracts.EventServiceReady,
			Timestamp: at,
			Data:      contracts.ServiceReady{ServiceEvent: contracts.ServiceEvent{Service: *api}, PID: 1234},
		}
	}
	stopped := contracts.Message{
		Type:      contracts.EventServiceStopped,
		Timestamp: after,
		Data:      contracts.ServiceStopped{ServiceEvent: contracts.ServiceEvent{Service: *api}},
	}
	failed := contracts.Message{
		Type:      contracts.EventServiceFailed,
		Timestamp: after,
		Data:      contracts.ServiceFailed{ServiceEvent: contracts.ServiceEvent{Service: *api}, Error: errors.New("boom")},
	}

	web := &model.Service{ID: "test-id-web", Name: "web", Watch: &model.Watch{}}
	watchedResolved := contracts.Message{
		Type: contracts.EventProfileResolved,
		Data: contracts.ProfileResolved{Profile: "default", Tiers: []model.Tier{{Name: "foundation", Services: []*model.Service{web}}}},
	}
	watchedReady := contracts.Message{
		Type:      contracts.EventServiceReady,
		Timestamp: after,
		Data:      contracts.ServiceReady{ServiceEvent: contracts.ServiceEvent{Service: *web}, PID: 1234},
	}

	cancelled, cancel := context.WithCancel(context.Background())
	cancel()

	tests := []struct {
		name   string
		before func(s *Store)
		id     string
		err    error
	}{
		{
			name:   "an unknown service",
			before: func(*Store) {},
			id:     api.ID,
			err:    contracts.ErrServiceNotFound,
		},
		{
			name:   "a pending service waits until the context is done",
			before: func(s *Store) { s.handle(resolved) },
			id:     api.ID,
			err:    context.Canceled,
		},
		{
			name: "a service settled before the action waits until the context is done",
			before: func(s *Store) {
				s.handle(resolved)
				s.handle(ready(before))
			},
			id:  api.ID,
			err: context.Canceled,
		},
		{
			name: "a running service returns at once",
			before: func(s *Store) {
				s.handle(resolved)
				s.handle(ready(after))
			},
			id: api.ID,
		},
		{
			name: "a watched running service waits until the watcher starts",
			before: func(s *Store) {
				s.handle(watchedResolved)
				s.handle(watchedReady)
			},
			id:  web.ID,
			err: context.Canceled,
		},
		{
			name: "a watched running service returns once the watcher starts",
			before: func(s *Store) {
				s.handle(watchedResolved)
				s.handle(watchedReady)
				s.handle(contracts.Message{Type: contracts.EventWatchStarted, Data: contracts.WatchStarted{Service: *web}})
			},
			id: web.ID,
		},
		{
			name: "a stopped service returns at once",
			before: func(s *Store) {
				s.handle(resolved)
				s.handle(stopped)
			},
			id: api.ID,
		},
		{
			name: "a failed service returns at once",
			before: func(s *Store) {
				s.handle(resolved)
				s.handle(failed)
			},
			id: api.ID,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewStore(mockSubscriber, mockPublisher)
			tt.before(s)

			err := s.WaitSettled(cancelled, tt.id, since)

			assert.ErrorIs(t, err, tt.err)
		})
	}
}

func Test_Store_WaitSettled_ReturnsOnChange(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPublisher := NewMockPublisher(ctrl)
	mockPublisher.EXPECT().Publish(gomock.Any()).Return(nil).AnyTimes()

	since := time.Now()
	api := &model.Service{ID: "test-id-api", Name: "api"}
	ready := func(at time.Time) contracts.Message {
		return contracts.Message{
			Type:      contracts.EventServiceReady,
			Timestamp: at,
			Data:      contracts.ServiceReady{ServiceEvent: contracts.ServiceEvent{Service: *api}, PID: 1234},
		}
	}

	s := NewStore(NewMockSubscriber(ctrl), mockPublisher)
	s.handle(contracts.Message{
		Type: contracts.EventProfileResolved,
		Data: contracts.ProfileResolved{Profile: "default", Tiers: []model.Tier{{Name: "foundation", Services: []*model.Service{api}}}},
	})
	s.handle(ready(since.Add(-time.Second)))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go func() {
		s.handle(ready(since.Add(time.Second)))
	}()

	assert.NoError(t, s.WaitSettled(ctx, api.ID, since))
}

func Test_Store_WaitSettled_IgnoresTheEarlierActionsOutcome(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPublisher := NewMockPublisher(ctrl)
	mockPublisher.EXPECT().Publish(gomock.Any()).Return(nil).AnyTimes()

	readBeforeAdmission := time.Now()
	earlierReady := readBeforeAdmission.Add(time.Millisecond)
	admittedAt := earlierReady.Add(time.Millisecond)

	api := &model.Service{ID: "test-id-api", Name: "api"}

	s := NewStore(NewMockSubscriber(ctrl), mockPublisher)
	s.handle(contracts.Message{
		Type: contracts.EventProfileResolved,
		Data: contracts.ProfileResolved{Profile: "default", Tiers: []model.Tier{{Name: "foundation", Services: []*model.Service{api}}}},
	})
	s.handle(contracts.Message{
		Type:      contracts.EventServiceReady,
		Timestamp: earlierReady,
		Data:      contracts.ServiceReady{ServiceEvent: contracts.ServiceEvent{Service: *api}, PID: 1234},
	})

	cancelled, cancel := context.WithCancel(context.Background())
	cancel()

	require.NoError(t, s.WaitSettled(cancelled, api.ID, readBeforeAdmission), "a clock read before admission takes the earlier outcome")
	assert.ErrorIs(t, s.WaitSettled(cancelled, api.ID, admittedAt), context.Canceled)
}
