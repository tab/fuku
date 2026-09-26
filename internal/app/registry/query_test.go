package registry

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
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
