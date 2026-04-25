package services

import (
	"io"
	"testing"
	"time"

	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"fuku/internal/app/bus"
	"fuku/internal/app/registry"
	"fuku/internal/app/ui/components"
	"fuku/internal/config/logger"
)

func Test_HandleProfileResolved(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLog := logger.NewMockLogger(ctrl)
	noopLogger := zerolog.New(io.Discard)
	mockLog.EXPECT().Debug().Return(noopLogger.Debug()).AnyTimes()
	mockLog.EXPECT().Info().Return(noopLogger.Info()).AnyTimes()
	mockLog.EXPECT().Warn().Return(noopLogger.Warn()).AnyTimes()
	mockLog.EXPECT().Error().Return(noopLogger.Error()).AnyTimes()

	m := Model{log: mockLog, loader: NewLoader()}
	m.state.services = make(map[string]*ServiceState)
	m.state.restarting = make(map[string]bool)
	m.state.tiers = make([]Tier, 0)

	event := bus.Message{
		Type: bus.EventProfileResolved,
		Data: bus.ProfileResolved{
			Profile: "dev",
			Tiers: []bus.Tier{
				{Name: "tier1", Services: []bus.Service{{ID: "test-id-db", Name: "db"}}},
				{Name: "tier2", Services: []bus.Service{{ID: "test-id-api", Name: "api"}, {ID: "test-id-web", Name: "web"}}},
			},
		},
	}

	result := m.handleProfileResolved(event)

	assert.Len(t, result.state.tiers, 2)
	assert.Equal(t, "tier1", result.state.tiers[0].Name)
	assert.Equal(t, "tier2", result.state.tiers[1].Name)
	assert.Equal(t, []string{"test-id-db"}, result.state.tiers[0].Services)
	assert.Equal(t, []string{"test-id-api", "test-id-web"}, result.state.tiers[1].Services)
	assert.Len(t, result.state.services, 3)
	assert.NotNil(t, result.state.services["test-id-db"])
	assert.NotNil(t, result.state.services["test-id-api"])
	assert.NotNil(t, result.state.services["test-id-web"])
	assert.Equal(t, StatusStarting, result.state.services["test-id-db"].Status)
	assert.NotNil(t, result.state.services["test-id-db"].Blink)
	assert.NotNil(t, result.state.services["test-id-api"].Blink)
	assert.NotNil(t, result.state.services["test-id-web"].Blink)
}

func Test_HandleProfileResolved_InvalidData(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLog := logger.NewMockLogger(ctrl)
	noopLogger := zerolog.New(io.Discard)
	mockLog.EXPECT().Debug().Return(noopLogger.Debug()).AnyTimes()
	mockLog.EXPECT().Info().Return(noopLogger.Info()).AnyTimes()
	mockLog.EXPECT().Warn().Return(noopLogger.Warn()).AnyTimes()
	mockLog.EXPECT().Error().Return(noopLogger.Error()).AnyTimes()

	m := Model{log: mockLog, loader: NewLoader()}
	m.state.services = make(map[string]*ServiceState)
	m.state.restarting = make(map[string]bool)
	m.state.tiers = make([]Tier, 0)
	event := bus.Message{Type: bus.EventProfileResolved, Data: "invalid"}
	result := m.handleProfileResolved(event)
	assert.Empty(t, result.state.tiers)
}

func Test_HandleProfileResolved_ClearsStaleServices(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLog := logger.NewMockLogger(ctrl)
	noopLogger := zerolog.New(io.Discard)
	mockLog.EXPECT().Debug().Return(noopLogger.Debug()).AnyTimes()
	mockLog.EXPECT().Info().Return(noopLogger.Info()).AnyTimes()
	mockLog.EXPECT().Warn().Return(noopLogger.Warn()).AnyTimes()
	mockLog.EXPECT().Error().Return(noopLogger.Error()).AnyTimes()

	m := Model{log: mockLog, loader: NewLoader()}
	m.state.services = make(map[string]*ServiceState)
	m.state.restarting = make(map[string]bool)
	m.state.tiers = make([]Tier, 0)

	event1 := bus.Message{
		Type: bus.EventProfileResolved,
		Data: bus.ProfileResolved{
			Profile: "profile1",
			Tiers: []bus.Tier{
				{Name: "tier1", Services: []bus.Service{{ID: "test-id-db", Name: "db"}, {ID: "test-id-api", Name: "api"}, {ID: "test-id-web", Name: "web"}}},
			},
		},
	}

	result := m.handleProfileResolved(event1)
	assert.Len(t, result.state.services, 3, "First profile should have 3 services")
	assert.NotNil(t, result.state.services["test-id-db"])
	assert.NotNil(t, result.state.services["test-id-api"])
	assert.NotNil(t, result.state.services["test-id-web"])

	event2 := bus.Message{
		Type: bus.EventProfileResolved,
		Data: bus.ProfileResolved{
			Profile: "profile2",
			Tiers: []bus.Tier{
				{Name: "tier1", Services: []bus.Service{{ID: "test-id-storage", Name: "storage"}, {ID: "test-id-cache", Name: "cache"}}},
			},
		},
	}

	result = result.handleProfileResolved(event2)
	assert.Len(t, result.state.services, 2, "Second profile should have exactly 2 services, not 5")
	assert.NotNil(t, result.state.services["test-id-storage"])
	assert.NotNil(t, result.state.services["test-id-cache"])
	assert.Nil(t, result.state.services["test-id-db"], "Old service 'db' should be removed")
	assert.Nil(t, result.state.services["test-id-api"], "Old service 'api' should be removed")
	assert.Nil(t, result.state.services["test-id-web"], "Old service 'web' should be removed")
}

func Test_HandleProfileResolved_ResetsSelection(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLog := logger.NewMockLogger(ctrl)
	noopLogger := zerolog.New(io.Discard)
	mockLog.EXPECT().Debug().Return(noopLogger.Debug()).AnyTimes()
	mockLog.EXPECT().Info().Return(noopLogger.Info()).AnyTimes()
	mockLog.EXPECT().Warn().Return(noopLogger.Warn()).AnyTimes()
	mockLog.EXPECT().Error().Return(noopLogger.Error()).AnyTimes()

	m := Model{log: mockLog, loader: NewLoader()}
	m.state.services = make(map[string]*ServiceState)
	m.state.restarting = make(map[string]bool)
	m.state.tiers = make([]Tier, 0)
	m.state.selected = 5

	event := bus.Message{
		Type: bus.EventProfileResolved,
		Data: bus.ProfileResolved{
			Profile: "dev",
			Tiers:   []bus.Tier{{Name: "tier1", Services: []bus.Service{{ID: "test-id-db", Name: "db"}}}},
		},
	}

	result := m.handleProfileResolved(event)
	assert.Equal(t, 0, result.state.selected, "Selection should reset to 0 on profile reload")
}

func Test_HandleProfileResolved_ClearsFilterState(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLog := logger.NewMockLogger(ctrl)
	noopLogger := zerolog.New(io.Discard)
	mockLog.EXPECT().Debug().Return(noopLogger.Debug()).AnyTimes()
	mockLog.EXPECT().Info().Return(noopLogger.Info()).AnyTimes()
	mockLog.EXPECT().Warn().Return(noopLogger.Warn()).AnyTimes()
	mockLog.EXPECT().Error().Return(noopLogger.Error()).AnyTimes()

	m := Model{log: mockLog, loader: NewLoader()}
	m.state.services = make(map[string]*ServiceState)
	m.state.restarting = make(map[string]bool)
	m.state.tiers = make([]Tier, 0)
	m.state.filterQuery = "db"
	m.state.filterActive = true
	m.state.filteredIDs = []string{"id-db"}
	m.state.filteredTiers = []Tier{{Name: "tier1", Services: []string{"id-db"}}}

	event := bus.Message{
		Type: bus.EventProfileResolved,
		Data: bus.ProfileResolved{
			Profile: "dev",
			Tiers:   []bus.Tier{{Name: "tier1", Services: []bus.Service{{ID: "test-id-db", Name: "db"}}}},
		},
	}

	result := m.handleProfileResolved(event)
	assert.Empty(t, result.state.filterQuery)
	assert.False(t, result.state.filterActive)
	assert.Nil(t, result.state.filteredIDs)
	assert.Nil(t, result.state.filteredTiers)
}

func Test_HandleProfileResolved_PreservesReadyState(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLog := logger.NewMockLogger(ctrl)
	noopLogger := zerolog.New(io.Discard)
	mockLog.EXPECT().Debug().Return(noopLogger.Debug()).AnyTimes()
	mockLog.EXPECT().Info().Return(noopLogger.Info()).AnyTimes()
	mockLog.EXPECT().Warn().Return(noopLogger.Warn()).AnyTimes()
	mockLog.EXPECT().Error().Return(noopLogger.Error()).AnyTimes()

	m := Model{log: mockLog, loader: NewLoader()}
	m.state.services = make(map[string]*ServiceState)
	m.state.restarting = make(map[string]bool)
	m.state.tiers = make([]Tier, 0)
	m.state.ready = true

	event := bus.Message{
		Type: bus.EventProfileResolved,
		Data: bus.ProfileResolved{
			Profile: "dev",
			Tiers:   []bus.Tier{{Name: "tier1", Services: []bus.Service{{ID: "test-id-db", Name: "db"}}}},
		},
	}

	result := m.handleProfileResolved(event)
	assert.True(t, result.state.ready, "Ready state should be preserved on profile reload")
}

func Test_HandleProfileResolved_ClearsLoaderQueue(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	loader := NewLoader()
	loader.Start("old-service-1", "starting old-service-1")
	loader.Start("old-service-2", "starting old-service-2")

	mockLog := logger.NewMockLogger(ctrl)
	noopLogger := zerolog.New(io.Discard)
	mockLog.EXPECT().Debug().Return(noopLogger.Debug()).AnyTimes()
	mockLog.EXPECT().Info().Return(noopLogger.Info()).AnyTimes()
	mockLog.EXPECT().Warn().Return(noopLogger.Warn()).AnyTimes()
	mockLog.EXPECT().Error().Return(noopLogger.Error()).AnyTimes()

	m := Model{log: mockLog, loader: loader}
	m.state.services = make(map[string]*ServiceState)
	m.state.restarting = make(map[string]bool)
	m.state.tiers = make([]Tier, 0)

	assert.True(t, loader.Active, "Loader should be active before reload")
	assert.True(t, loader.Has("old-service-1"), "Loader should have old-service-1")
	assert.True(t, loader.Has("old-service-2"), "Loader should have old-service-2")

	event := bus.Message{
		Type: bus.EventProfileResolved,
		Data: bus.ProfileResolved{
			Profile: "new-profile",
			Tiers:   []bus.Tier{{Name: "tier1", Services: []bus.Service{{ID: "test-id-new-service", Name: "new-service"}}}},
		},
	}

	result := m.handleProfileResolved(event)
	assert.False(t, result.loader.Active, "Loader should be cleared after profile reload")
	assert.False(t, result.loader.Has("old-service-1"), "Loader should not have old-service-1")
	assert.False(t, result.loader.Has("old-service-2"), "Loader should not have old-service-2")
}

func Test_HandleTierStarting(t *testing.T) {
	m := Model{}
	m.state.tiers = []Tier{
		{Name: "tier1", Ready: true},
		{Name: "tier2", Ready: true},
	}
	m.state.tierIndex = map[string]int{"tier1": 0, "tier2": 1}

	event := bus.Message{Type: bus.EventTierStarting, Data: bus.TierStarting{Name: "tier1"}}
	result := m.handleTierStarting(event)

	assert.False(t, result.state.tiers[0].Ready)
	assert.True(t, result.state.tiers[1].Ready)
}

func Test_HandleTierStarting_InvalidData(t *testing.T) {
	m := Model{}
	m.state.tiers = []Tier{{Name: "tier1", Ready: true}}
	event := bus.Message{Type: bus.EventTierStarting, Data: "invalid"}
	result := m.handleTierStarting(event)
	assert.True(t, result.state.tiers[0].Ready)
}

func Test_HandleTierReady(t *testing.T) {
	m := Model{}
	m.state.tiers = []Tier{
		{Name: "tier1", Ready: false},
		{Name: "tier2", Ready: false},
	}
	m.state.tierIndex = map[string]int{"tier1": 0, "tier2": 1}

	event := bus.Message{Type: bus.EventTierReady, Data: bus.TierReady{Name: "tier2"}}
	result := m.handleTierReady(event)

	assert.False(t, result.state.tiers[0].Ready)
	assert.True(t, result.state.tiers[1].Ready)
}

func Test_HandleTierReady_InvalidData(t *testing.T) {
	m := Model{}
	m.state.tiers = []Tier{{Name: "tier1", Ready: false}}
	event := bus.Message{Type: bus.EventTierReady, Data: "invalid"}
	result := m.handleTierReady(event)
	assert.False(t, result.state.tiers[0].Ready)
}

func Test_HandleServiceStarting(t *testing.T) {
	loader := &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}
	service := &ServiceState{
		Name:   "api",
		Status: StatusStopped,
		Blink:  components.NewBlink(),
	}

	m := Model{loader: loader}
	m.state.services = map[string]*ServiceState{"test-id-api": service}
	m.state.restarting = make(map[string]bool)

	now := time.Now()
	startedAt := now.Add(-time.Second)
	event := bus.Message{
		Timestamp: now,
		Seq:       1,
		Type:      bus.EventServiceStarting,
		Data:      bus.ServiceStarting{ServiceEvent: bus.ServiceEvent{Service: bus.Service{ID: "test-id-api", Name: "api"}, Tier: "tier1"}, PID: 1234, StartedAt: startedAt},
	}

	result := m.handleServiceStarting(event)

	assert.Equal(t, StatusStarting, result.state.services["test-id-api"].Status)
	assert.Equal(t, "tier1", result.state.services["test-id-api"].Tier)
	assert.Equal(t, 1234, result.state.services["test-id-api"].PID)
	assert.Equal(t, startedAt, result.state.services["test-id-api"].StartTime)
	require.NoError(t, result.state.services["test-id-api"].Error)
	assert.True(t, result.loader.Has("test-id-api"))
	assert.True(t, result.loader.Active)
}

func Test_HandleServiceStarting_InvalidData(t *testing.T) {
	loader := &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}
	m := Model{loader: loader}
	m.state.services = make(map[string]*ServiceState)
	m.state.restarting = make(map[string]bool)
	event := bus.Message{Type: bus.EventServiceStarting, Data: "invalid"}
	result := m.handleServiceStarting(event)
	assert.False(t, result.loader.Active)
}

func Test_HandleServiceStarting_ClearsRestartingFlag(t *testing.T) {
	loader := &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}
	service := &ServiceState{
		Name:   "api",
		Status: StatusStopped,
		Blink:  components.NewBlink(),
	}

	m := Model{loader: loader}
	m.state.services = map[string]*ServiceState{"test-id-api": service}
	m.state.restarting = map[string]bool{"api": true}

	event := bus.Message{
		Timestamp: time.Now(),
		Seq:       1,
		Type:      bus.EventServiceStarting,
		Data:      bus.ServiceStarting{ServiceEvent: bus.ServiceEvent{Service: bus.Service{ID: "test-id-api", Name: "api"}, Tier: "tier1"}, PID: 5678},
	}

	result := m.handleServiceStarting(event)

	assert.False(t, result.state.restarting["test-id-api"])
}

func Test_HandleServiceStarting_DoesNotDuplicateLoader(t *testing.T) {
	loader := &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}
	loader.Start("test-id-api", "starting api…")

	service := &ServiceState{
		Name:   "api",
		Status: StatusStopped,
		Blink:  components.NewBlink(),
	}

	m := Model{loader: loader}
	m.state.services = map[string]*ServiceState{"test-id-api": service}
	m.state.restarting = make(map[string]bool)

	event := bus.Message{
		Timestamp: time.Now(),
		Seq:       1,
		Type:      bus.EventServiceStarting,
		Data:      bus.ServiceStarting{ServiceEvent: bus.ServiceEvent{Service: bus.Service{ID: "test-id-api", Name: "api"}, Tier: "tier1"}, PID: 1234},
	}

	result := m.handleServiceStarting(event)

	count := 0

	for _, item := range result.loader.queue {
		if item.Service == "test-id-api" {
			count++
		}
	}

	assert.Equal(t, 1, count, "Loader should not have duplicate entries for the same service")
}

func Test_HandleServiceReady(t *testing.T) {
	loader := &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}
	loader.Start("test-id-api", "starting api...")

	service := &ServiceState{
		Name:   "api",
		Status: StatusStarting,
		Blink:  components.NewBlink(),
	}

	m := Model{loader: loader}
	m.state.services = map[string]*ServiceState{"test-id-api": service}

	now := time.Now()
	started := now.Add(-2 * time.Second)
	event := bus.Message{
		Timestamp: now,
		Seq:       1,
		Type:      bus.EventServiceReady,
		Data: bus.ServiceReady{
			ServiceEvent: bus.ServiceEvent{Service: bus.Service{ID: "test-id-api", Name: "api"}, Tier: "platform"},
			PID:          5678,
			StartedAt:    started,
		},
	}

	result := m.handleServiceReady(event)

	svc := result.state.services["test-id-api"]
	assert.Equal(t, StatusRunning, svc.Status)
	assert.Equal(t, "platform", svc.Tier)
	assert.Equal(t, 5678, svc.PID)
	assert.Equal(t, started, svc.StartTime)
	assert.Equal(t, now, svc.ReadyTime)
	require.NoError(t, svc.Error)
	assert.False(t, result.loader.Active)
	assert.False(t, result.loader.Has("test-id-api"))
}

func Test_HandleServiceReady_InvalidData(t *testing.T) {
	loader := &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}
	loader.Start("test-id-api", "starting api...")
	m := Model{loader: loader}
	m.state.services = make(map[string]*ServiceState)
	event := bus.Message{Type: bus.EventServiceReady, Data: "invalid"}
	result := m.handleServiceReady(event)
	assert.True(t, result.loader.Active)
}

func Test_HandleServiceFailed(t *testing.T) {
	loader := &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}
	loader.Start("test-id-api", "starting api...")

	service := &ServiceState{
		Name:   "api",
		Status: StatusStarting,
		Blink:  components.NewBlink(),
	}

	m := Model{loader: loader}
	m.state.services = map[string]*ServiceState{"test-id-api": service}
	m.state.restarting = make(map[string]bool)

	testErr := assert.AnError
	event := bus.Message{
		Seq:  1,
		Type: bus.EventServiceFailed,
		Data: bus.ServiceFailed{ServiceEvent: bus.ServiceEvent{Service: bus.Service{ID: "test-id-api", Name: "api"}}, Error: testErr},
	}

	result := m.handleServiceFailed(event)

	assert.Equal(t, StatusFailed, result.state.services["test-id-api"].Status)
	assert.Equal(t, testErr, result.state.services["test-id-api"].Error)
	assert.False(t, result.loader.Active)
	assert.False(t, result.loader.Has("test-id-api"))
}

func Test_HandleServiceFailed_InvalidData(t *testing.T) {
	loader := &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}
	m := Model{loader: loader}
	m.state.services = make(map[string]*ServiceState)
	m.state.restarting = make(map[string]bool)
	event := bus.Message{Type: bus.EventServiceFailed, Data: "invalid"}
	result := m.handleServiceFailed(event)
	assert.Empty(t, result.state.services)
}

func Test_HandleServiceFailed_ClearsRestartingFlag(t *testing.T) {
	loader := &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}
	loader.Start("test-id-api", "restarting api...")

	service := &ServiceState{
		Name:   "api",
		Status: StatusStarting,
		Blink:  components.NewBlink(),
	}

	m := Model{loader: loader}
	m.state.services = map[string]*ServiceState{"test-id-api": service}
	m.state.restarting = map[string]bool{"api": true}

	event := bus.Message{
		Seq:  1,
		Type: bus.EventServiceFailed,
		Data: bus.ServiceFailed{ServiceEvent: bus.ServiceEvent{Service: bus.Service{ID: "test-id-api", Name: "api"}}, Error: assert.AnError},
	}

	result := m.handleServiceFailed(event)

	assert.False(t, result.state.restarting["test-id-api"])
}

func Test_HandleServiceStopping(t *testing.T) {
	loader := &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}
	service := &ServiceState{
		Name:   "api",
		Status: StatusRunning,
		Blink:  components.NewBlink(),
	}

	m := Model{loader: loader}
	m.state.services = map[string]*ServiceState{"test-id-api": service}

	event := bus.Message{
		Seq:  1,
		Type: bus.EventServiceStopping,
		Data: bus.ServiceStopping{ServiceEvent: bus.ServiceEvent{Service: bus.Service{ID: "test-id-api", Name: "api"}}},
	}

	result := m.handleServiceStopping(event)

	assert.Equal(t, StatusStopping, result.state.services["test-id-api"].Status)
	assert.True(t, result.loader.Active)
	assert.True(t, result.loader.Has("test-id-api"))
}

func Test_HandleServiceStopping_InvalidData(t *testing.T) {
	loader := &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}
	m := Model{loader: loader}
	m.state.services = make(map[string]*ServiceState)
	event := bus.Message{Type: bus.EventServiceStopping, Data: "invalid"}
	result := m.handleServiceStopping(event)
	assert.False(t, result.loader.Active)
}

func Test_HandleServiceStopping_DoesNotDuplicateLoader(t *testing.T) {
	loader := &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}
	loader.Start("test-id-api", "stopping api…")

	service := &ServiceState{
		Name:   "api",
		Status: StatusRunning,
		Blink:  components.NewBlink(),
	}

	m := Model{loader: loader}
	m.state.services = map[string]*ServiceState{"test-id-api": service}

	event := bus.Message{
		Seq:  1,
		Type: bus.EventServiceStopping,
		Data: bus.ServiceStopping{ServiceEvent: bus.ServiceEvent{Service: bus.Service{ID: "test-id-api", Name: "api"}}},
	}

	result := m.handleServiceStopping(event)

	count := 0

	for _, item := range result.loader.queue {
		if item.Service == "test-id-api" {
			count++
		}
	}

	assert.Equal(t, 1, count, "Loader should not have duplicate entries for the same service")
}

func Test_HandleServiceRestarting(t *testing.T) {
	loader := &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}
	service := &ServiceState{
		Name:   "api",
		Status: StatusRunning,
		Blink:  components.NewBlink(),
	}

	m := Model{loader: loader}
	m.state.services = map[string]*ServiceState{"test-id-api": service}
	m.state.restarting = make(map[string]bool)

	event := bus.Message{
		Seq:  1,
		Type: bus.EventServiceRestarting,
		Data: bus.ServiceRestarting{ServiceEvent: bus.ServiceEvent{Service: bus.Service{ID: "test-id-api", Name: "api"}}},
	}

	result := m.handleServiceRestarting(event)

	assert.Equal(t, StatusRestarting, result.state.services["test-id-api"].Status)
	assert.True(t, result.state.restarting["test-id-api"])
	assert.True(t, result.loader.Active)
	assert.True(t, result.loader.Has("test-id-api"))
}

func Test_HandleServiceRestarting_InvalidData(t *testing.T) {
	loader := &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}
	m := Model{loader: loader}
	m.state.services = make(map[string]*ServiceState)
	m.state.restarting = make(map[string]bool)
	event := bus.Message{Type: bus.EventServiceRestarting, Data: "invalid"}
	result := m.handleServiceRestarting(event)
	assert.False(t, result.loader.Active)
	assert.Empty(t, result.state.restarting)
}

func Test_HandleServiceStopped(t *testing.T) {
	tests := []struct {
		name          string
		restarting    bool
		loaderActive  bool
		loaderHasItem bool
	}{
		{
			name:          "Not restarting stops loader",
			restarting:    false,
			loaderActive:  false,
			loaderHasItem: false,
		},
		{
			name:          "Restarting keeps loader",
			restarting:    true,
			loaderActive:  true,
			loaderHasItem: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loader := &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}
			loader.Start("test-id-api", "stopping api...")

			service := &ServiceState{
				Name:   "api",
				Status: StatusRunning,
				PID:    1234,
				Blink:  components.NewBlink(),
			}

			m := Model{loader: loader}
			m.state.services = map[string]*ServiceState{"test-id-api": service}
			m.state.restarting = map[string]bool{"test-id-api": tt.restarting}

			event := bus.Message{
				Seq:  1,
				Type: bus.EventServiceStopped,
				Data: bus.ServiceStopped{ServiceEvent: bus.ServiceEvent{Service: bus.Service{ID: "test-id-api", Name: "api"}}},
			}

			result := m.handleServiceStopped(event)

			assert.Equal(t, StatusStopped, result.state.services["test-id-api"].Status)
			assert.Equal(t, tt.loaderActive, result.loader.Active)
			assert.Equal(t, tt.loaderHasItem, result.loader.Has("test-id-api"))
		})
	}
}

func Test_HandleServiceStopped_InvalidData(t *testing.T) {
	loader := &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}
	m := Model{loader: loader}
	m.state.services = make(map[string]*ServiceState)
	m.state.restarting = make(map[string]bool)
	event := bus.Message{Type: bus.EventServiceStopped, Data: "invalid"}
	result := m.handleServiceStopped(event)
	assert.Empty(t, result.state.services)
}

func Test_HandleServiceStopped_UnknownService(t *testing.T) {
	loader := &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}
	m := Model{loader: loader}
	m.state.services = make(map[string]*ServiceState)
	m.state.restarting = make(map[string]bool)

	event := bus.Message{
		Type: bus.EventServiceStopped,
		Data: bus.ServiceStopped{ServiceEvent: bus.ServiceEvent{Service: bus.Service{ID: "test-id-unknown", Name: "unknown"}}},
	}

	result := m.handleServiceStopped(event)
	assert.Empty(t, result.state.services)
}

func Test_HandleWatchStarted(t *testing.T) {
	service := &ServiceState{Name: "api", Status: StatusRunning, Watching: false}
	m := Model{}
	m.state.services = map[string]*ServiceState{"test-id-api": service}

	event := bus.Message{
		Seq:  1,
		Type: bus.EventWatchStarted,
		Data: bus.Service{ID: "test-id-api", Name: "api"},
	}

	result := m.handleWatchStarted(event)

	assert.True(t, result.state.services["test-id-api"].Watching)
}

func Test_HandleWatchStarted_InvalidData(t *testing.T) {
	service := &ServiceState{Name: "api", Status: StatusRunning, Watching: false}
	m := Model{}
	m.state.services = map[string]*ServiceState{"test-id-api": service}

	event := bus.Message{Type: bus.EventWatchStarted, Data: "invalid"}
	result := m.handleWatchStarted(event)

	assert.False(t, result.state.services["test-id-api"].Watching)
}

func Test_HandleWatchStarted_UnknownService(t *testing.T) {
	m := Model{}
	m.state.services = make(map[string]*ServiceState)

	event := bus.Message{
		Type: bus.EventWatchStarted,
		Data: bus.Service{Name: "unknown"},
	}

	result := m.handleWatchStarted(event)

	assert.Empty(t, result.state.services)
}

func Test_HandleWatchStopped(t *testing.T) {
	service := &ServiceState{Name: "api", Status: StatusStopped, Watching: true}
	m := Model{}
	m.state.services = map[string]*ServiceState{"test-id-api": service}

	event := bus.Message{
		Seq:  1,
		Type: bus.EventWatchStopped,
		Data: bus.Service{ID: "test-id-api", Name: "api"},
	}

	result := m.handleWatchStopped(event)

	assert.False(t, result.state.services["test-id-api"].Watching)
}

func Test_HandleWatchStopped_InvalidData(t *testing.T) {
	service := &ServiceState{Name: "api", Status: StatusStopped, Watching: true}
	m := Model{}
	m.state.services = map[string]*ServiceState{"test-id-api": service}

	event := bus.Message{Type: bus.EventWatchStopped, Data: "invalid"}
	result := m.handleWatchStopped(event)

	assert.True(t, result.state.services["test-id-api"].Watching)
}

func Test_HandleWatchStopped_UnknownService(t *testing.T) {
	m := Model{}
	m.state.services = make(map[string]*ServiceState)

	event := bus.Message{
		Type: bus.EventWatchStopped,
		Data: bus.Service{Name: "unknown"},
	}

	result := m.handleWatchStopped(event)

	assert.Empty(t, result.state.services)
}

func Test_HandlePhaseChanged_PhaseStopped(t *testing.T) {
	loader := &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}
	loader.Start(loaderKeyShutdown, "shutting down...")

	m := Model{loader: loader}
	m.state.shuttingDown = true

	event := bus.Message{Type: bus.EventPhaseChanged, Data: bus.PhaseChanged{Phase: bus.PhaseStopped}}
	result, cmd := m.handlePhaseChanged(event)

	assert.False(t, result.loader.Active)
	assert.NotNil(t, cmd)
}

func Test_HandlePhaseChanged_OtherPhase(t *testing.T) {
	loader := &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}
	msgChan := make(chan bus.Message, 1)

	m := Model{loader: loader, msgChan: msgChan}

	event := bus.Message{Type: bus.EventPhaseChanged, Data: bus.PhaseChanged{Phase: bus.PhaseRunning}}
	result, cmd := m.handlePhaseChanged(event)

	assert.Equal(t, bus.PhaseRunning, result.state.phase)
	assert.NotNil(t, cmd)
}

func Test_HandleKeyPress_ForceQuitWithCtrlC(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().Warn().Return(nil)

	mockController := NewMockController(ctrl)
	loader := &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}

	m := Model{loader: loader, controller: mockController, log: mockLogger}
	m.state.shuttingDown = false
	m.ui.servicesKeys = DefaultKeyMap()

	msg := tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}
	teaModel, cmd := m.handleKeyPress(msg)
	result := teaModel.(Model)

	assert.False(t, result.loader.Active)
	assert.NotNil(t, cmd)
}

func Test_HandleKeyPress_IgnoresQuitWhileShuttingDown(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockController := NewMockController(ctrl)
	loader := &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}

	m := Model{loader: loader, controller: mockController}
	m.state.shuttingDown = true
	m.ui.servicesKeys = DefaultKeyMap()

	msg := toKeyMsg("q")
	teaModel, cmd := m.handleKeyPress(msg)
	result := teaModel.(Model)

	assert.True(t, result.state.shuttingDown)
	assert.Nil(t, cmd)
}

func Test_HandleKeyPress_IgnoresOtherKeysWhileShuttingDown(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockController := NewMockController(ctrl)
	loader := &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}

	m := Model{loader: loader, controller: mockController}
	m.state.shuttingDown = true
	m.ui.servicesKeys = DefaultKeyMap()

	msg := toKeyMsg("j")
	teaModel, cmd := m.handleKeyPress(msg)
	result := teaModel.(Model)

	assert.True(t, result.state.shuttingDown)
	assert.Nil(t, cmd)
}

func Test_HandleKeyPress_QuitStartsGracefulShutdown(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockController := NewMockController(ctrl)
	mockController.EXPECT().StopAll()

	msgChan := make(chan bus.Message, 1)
	loader := &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}

	m := Model{loader: loader, controller: mockController, msgChan: msgChan}
	m.state.shuttingDown = false
	m.ui.servicesKeys = DefaultKeyMap()

	msg := toKeyMsg("q")
	teaModel, cmd := m.handleKeyPress(msg)
	result := teaModel.(Model)

	assert.True(t, result.state.shuttingDown)
	assert.True(t, result.loader.Active)
	assert.Equal(t, "shutting down all services\u2026", result.loader.Message())
	assert.NotNil(t, cmd)
}

func Test_HandleKeyPress_CtrlCForcesImmediateQuit(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().Warn().Return(nil)

	mockController := NewMockController(ctrl)
	loader := &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}

	m := Model{loader: loader, controller: mockController, log: mockLogger}
	m.state.shuttingDown = false
	m.ui.servicesKeys = DefaultKeyMap()

	msg := tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}
	_, cmd := m.handleKeyPress(msg)

	assert.NotNil(t, cmd)
}

func Test_Update_KeyMsg_CtrlCForceQuitsDuringShutdown(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().Warn().Return(nil)

	mockController := NewMockController(ctrl)
	loader := &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}

	m := Model{loader: loader, controller: mockController, log: mockLogger}
	m.state.shuttingDown = true
	m.ui.servicesKeys = DefaultKeyMap()

	msg := tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}
	_, cmd := m.Update(msg)

	assert.NotNil(t, cmd)
}

func Test_Update_KeyMsg_IgnoreQuitDuringShutdown(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockController := NewMockController(ctrl)
	loader := &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}

	m := Model{loader: loader, controller: mockController}
	m.state.shuttingDown = true
	m.ui.servicesKeys = DefaultKeyMap()

	msg := toKeyMsg("q")
	teaModel, cmd := m.Update(msg)
	result := teaModel.(Model)

	assert.True(t, result.state.shuttingDown)
	assert.Nil(t, cmd)
}

func Test_Update_EventMsg(t *testing.T) {
	loader := &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}
	msgChan := make(chan bus.Message, 1)

	m := Model{loader: loader, msgChan: msgChan}
	m.state.shuttingDown = false

	event := bus.Message{Type: bus.EventSignal, Data: bus.Signal{Name: "SIGINT"}}
	msg := msgMsg(event)
	teaModel, cmd := m.Update(msg)
	result := teaModel.(Model)

	assert.True(t, result.state.shuttingDown)
	assert.NotNil(t, cmd)
}

func Test_HandlePreflightStarted(t *testing.T) {
	loader := &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}
	m := Model{loader: loader}

	result := m.handlePreflightStarted()

	assert.True(t, result.loader.Active)
	assert.True(t, result.loader.Has(loaderKeyPreflight))
	assert.Equal(t, "preflight: scanning processes\u2026", result.loader.Message())
}

func Test_HandlePreflightKill(t *testing.T) {
	loader := &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}
	loader.Start(loaderKeyPreflight, "preflight: scanning processes…")

	m := Model{loader: loader}

	event := bus.Message{Type: bus.EventPreflightKill, Data: bus.PreflightKill{Service: "api", PID: 1234, Name: "node"}}
	result := m.handlePreflightKill(event)

	assert.True(t, result.loader.Active)
	assert.Equal(t, "preflight: stopping api\u2026", result.loader.Message())
}

func Test_HandlePreflightKill_InvalidData(t *testing.T) {
	loader := &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}
	loader.Start(loaderKeyPreflight, "preflight: scanning processes…")

	m := Model{loader: loader}

	event := bus.Message{Type: bus.EventPreflightKill, Data: "invalid"}
	result := m.handlePreflightKill(event)

	assert.True(t, result.loader.Active)
	assert.Equal(t, "preflight: scanning processes\u2026", result.loader.Message())
}

func Test_HandlePreflightComplete(t *testing.T) {
	loader := &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}
	loader.Start(loaderKeyPreflight, "preflight: stopping api...")

	m := Model{loader: loader}

	result := m.handlePreflightComplete()

	assert.False(t, result.loader.Active)
	assert.False(t, result.loader.Has(loaderKeyPreflight))
}

func Test_HandleSignal(t *testing.T) {
	loader := &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}
	m := Model{loader: loader}
	m.state.shuttingDown = false

	result := m.handleSignal()

	assert.True(t, result.state.shuttingDown)
	assert.True(t, result.loader.Active)
	assert.Equal(t, "shutting down all services\u2026", result.loader.Message())
}

func Test_HandleFilterKey(t *testing.T) {
	m := Model{}
	m.state.filterActive = false
	m.state.serviceIDs = []string{"id-api", "id-web", "id-db"}
	m.state.services = map[string]*ServiceState{
		"id-api": {ID: "id-api", Name: "api"},
		"id-web": {ID: "id-web", Name: "web"},
		"id-db":  {ID: "id-db", Name: "db"},
	}
	m.state.selected = 2
	m.ui.servicesKeys = DefaultKeyMap()

	teaModel, cmd := m.handleFilterKey()
	result := teaModel.(Model)

	assert.True(t, result.state.filterActive)
	assert.Equal(t, "id-db", result.state.preFilterSelectedID)
	assert.Nil(t, cmd)
}

func Test_HandleKeyPress_SlashEntersFilterMode(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockController := NewMockController(ctrl)
	loader := &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}

	m := Model{loader: loader, controller: mockController}
	m.state.shuttingDown = false
	m.ui.servicesKeys = DefaultKeyMap()

	msg := toKeyMsg("/")
	teaModel, cmd := m.handleKeyPress(msg)
	result := teaModel.(Model)

	assert.True(t, result.state.filterActive)
	assert.Nil(t, cmd)
}

func Test_HandleFilterInput(t *testing.T) {
	tests := []struct {
		name           string
		initialQuery   string
		initialActive  bool
		msg            tea.KeyPressMsg
		expectedQuery  string
		expectedActive bool
	}{
		{
			name:           "Typing appends to query",
			initialQuery:   "ap",
			initialActive:  true,
			msg:            toKeyMsg("i"),
			expectedQuery:  "api",
			expectedActive: true,
		},
		{
			name:           "Typing first character",
			initialQuery:   "",
			initialActive:  true,
			msg:            toKeyMsg("a"),
			expectedQuery:  "a",
			expectedActive: true,
		},
		{
			name:           "Backspace removes last character",
			initialQuery:   "api",
			initialActive:  true,
			msg:            tea.KeyPressMsg{Code: tea.KeyBackspace},
			expectedQuery:  "ap",
			expectedActive: true,
		},
		{
			name:           "Backspace on empty query is no-op",
			initialQuery:   "",
			initialActive:  true,
			msg:            tea.KeyPressMsg{Code: tea.KeyBackspace},
			expectedQuery:  "",
			expectedActive: true,
		},
		{
			name:           "Enter exits input mode keeping filter",
			initialQuery:   "api",
			initialActive:  true,
			msg:            tea.KeyPressMsg{Code: tea.KeyEnter},
			expectedQuery:  "api",
			expectedActive: false,
		},
		{
			name:           "Escape clears filter and exits input mode",
			initialQuery:   "api",
			initialActive:  true,
			msg:            tea.KeyPressMsg{Code: tea.KeyEscape},
			expectedQuery:  "",
			expectedActive: false,
		},
		{
			name:           "Arrow up does not modify query",
			initialQuery:   "api",
			initialActive:  true,
			msg:            tea.KeyPressMsg{Code: tea.KeyUp},
			expectedQuery:  "api",
			expectedActive: true,
		},
		{
			name:           "Arrow down does not modify query",
			initialQuery:   "api",
			initialActive:  true,
			msg:            tea.KeyPressMsg{Code: tea.KeyDown},
			expectedQuery:  "api",
			expectedActive: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{}
			m.state.filterQuery = tt.initialQuery
			m.state.filterActive = tt.initialActive
			m.state.serviceIDs = []string{"id-api", "id-web", "id-db"}
			m.state.services = map[string]*ServiceState{
				"id-api": {ID: "id-api", Name: "api"},
				"id-web": {ID: "id-web", Name: "web"},
				"id-db":  {ID: "id-db", Name: "db"},
			}
			m.state.tiers = []Tier{
				{Name: "tier1", Services: []string{"id-api", "id-web", "id-db"}},
			}

			teaModel, _ := m.handleFilterInput(tt.msg)
			result := teaModel.(Model)

			assert.Equal(t, tt.expectedQuery, result.state.filterQuery)
			assert.Equal(t, tt.expectedActive, result.state.filterActive)
		})
	}
}

func Test_HandleKeyPress_FilterActiveRoutesToFilterInput(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockController := NewMockController(ctrl)
	loader := &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}

	m := Model{loader: loader, controller: mockController}
	m.state.filterActive = true
	m.state.filterQuery = ""
	m.state.serviceIDs = []string{"id-api"}
	m.state.services = map[string]*ServiceState{
		"id-api": {ID: "id-api", Name: "api"},
	}
	m.state.tiers = []Tier{
		{Name: "tier1", Services: []string{"id-api"}},
	}
	m.ui.servicesKeys = DefaultKeyMap()

	msg := toKeyMsg("a")
	teaModel, _ := m.handleKeyPress(msg)
	result := teaModel.(Model)

	assert.Equal(t, "a", result.state.filterQuery)
	assert.True(t, result.state.filterActive)
}

func Test_ApplyFilter_SelectionAdjustment(t *testing.T) {
	tests := []struct {
		name             string
		serviceIDs       []string
		services         map[string]*ServiceState
		tiers            []Tier
		filterQuery      string
		initialSelected  int
		expectedSelected int
	}{
		{
			name:       "Keeps selection when current service still visible",
			serviceIDs: []string{"id-api", "id-web", "id-db"},
			services: map[string]*ServiceState{
				"id-api": {ID: "id-api", Name: "api-server"},
				"id-web": {ID: "id-web", Name: "web-app"},
				"id-db":  {ID: "id-db", Name: "db-primary"},
			},
			tiers: []Tier{
				{Name: "tier1", Services: []string{"id-api", "id-web", "id-db"}},
			},
			filterQuery:      "api",
			initialSelected:  0,
			expectedSelected: 0,
		},
		{
			name:       "Moves to first match when current service hidden",
			serviceIDs: []string{"id-api", "id-web", "id-db"},
			services: map[string]*ServiceState{
				"id-api": {ID: "id-api", Name: "api-server"},
				"id-web": {ID: "id-web", Name: "web-app"},
				"id-db":  {ID: "id-db", Name: "db-primary"},
			},
			tiers: []Tier{
				{Name: "tier1", Services: []string{"id-api", "id-web", "id-db"}},
			},
			filterQuery:      "web",
			initialSelected:  2,
			expectedSelected: 0,
		},
		{
			name:       "Preserves selection at non-zero filtered position",
			serviceIDs: []string{"id-api", "id-web", "id-db"},
			services: map[string]*ServiceState{
				"id-api": {ID: "id-api", Name: "api-server"},
				"id-web": {ID: "id-web", Name: "web-app"},
				"id-db":  {ID: "id-db", Name: "db-primary"},
			},
			tiers: []Tier{
				{Name: "tier1", Services: []string{"id-api", "id-web", "id-db"}},
			},
			filterQuery:      "p",
			initialSelected:  1,
			expectedSelected: 1,
		},
		{
			name:       "Resets to zero on no matches",
			serviceIDs: []string{"id-api", "id-web", "id-db"},
			services: map[string]*ServiceState{
				"id-api": {ID: "id-api", Name: "api-server"},
				"id-web": {ID: "id-web", Name: "web-app"},
				"id-db":  {ID: "id-db", Name: "db-primary"},
			},
			tiers: []Tier{
				{Name: "tier1", Services: []string{"id-api", "id-web", "id-db"}},
			},
			filterQuery:      "zzz",
			initialSelected:  2,
			expectedSelected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{}
			m.state.serviceIDs = tt.serviceIDs
			m.state.services = tt.services
			m.state.tiers = tt.tiers
			m.state.filterQuery = tt.filterQuery
			m.state.selected = tt.initialSelected

			m.applyFilter()

			assert.Equal(t, tt.expectedSelected, m.state.selected)
		})
	}
}

func Test_HandleFilterInput_ArrowDownNavigates(t *testing.T) {
	m := Model{}
	m.state.filterActive = true
	m.state.filterQuery = "b"
	m.state.serviceIDs = []string{"id-api", "id-web", "id-db"}
	m.state.services = map[string]*ServiceState{
		"id-api": {ID: "id-api", Name: "api"},
		"id-web": {ID: "id-web", Name: "web"},
		"id-db":  {ID: "id-db", Name: "db"},
	}
	m.state.tiers = []Tier{
		{Name: "tier1", Services: []string{"id-api", "id-web", "id-db"}},
	}
	m.state.filteredIDs = []string{"id-web", "id-db"}
	m.state.filteredTiers = []Tier{
		{Name: "tier1", Services: []string{"id-web", "id-db"}},
	}
	m.state.selected = 0

	msg := tea.KeyPressMsg{Code: tea.KeyDown}
	teaModel, _ := m.handleFilterInput(msg)
	result := teaModel.(Model)

	assert.Equal(t, 1, result.state.selected)
	assert.Equal(t, "b", result.state.filterQuery)
	assert.True(t, result.state.filterActive)
}

func Test_HandleFilterInput_ArrowUpNavigates(t *testing.T) {
	m := Model{}
	m.state.filterActive = true
	m.state.filterQuery = "b"
	m.state.serviceIDs = []string{"id-api", "id-web", "id-db"}
	m.state.services = map[string]*ServiceState{
		"id-api": {ID: "id-api", Name: "api"},
		"id-web": {ID: "id-web", Name: "web"},
		"id-db":  {ID: "id-db", Name: "db"},
	}
	m.state.tiers = []Tier{
		{Name: "tier1", Services: []string{"id-api", "id-web", "id-db"}},
	}
	m.state.filteredIDs = []string{"id-web", "id-db"}
	m.state.filteredTiers = []Tier{
		{Name: "tier1", Services: []string{"id-web", "id-db"}},
	}
	m.state.selected = 1

	msg := tea.KeyPressMsg{Code: tea.KeyUp}
	teaModel, _ := m.handleFilterInput(msg)
	result := teaModel.(Model)

	assert.Equal(t, 0, result.state.selected)
	assert.Equal(t, "b", result.state.filterQuery)
	assert.True(t, result.state.filterActive)
}

func Test_ApplyFilter_RestoresSelectionAfterTemporaryZeroMatches(t *testing.T) {
	m := Model{}
	m.state.serviceIDs = []string{"id-api", "id-web", "id-db"}
	m.state.services = map[string]*ServiceState{
		"id-api": {ID: "id-api", Name: "api"},
		"id-web": {ID: "id-web", Name: "web"},
		"id-db":  {ID: "id-db", Name: "db"},
	}
	m.state.tiers = []Tier{
		{Name: "tier1", Services: []string{"id-api", "id-web", "id-db"}},
	}

	m.state.filteredIDs = []string{"id-web", "id-db"}
	m.state.filteredTiers = []Tier{
		{Name: "tier1", Services: []string{"id-web", "id-db"}},
	}
	m.state.selected = 1
	m.state.filterQuery = "bx"
	m.applyFilter()

	assert.Empty(t, m.state.filteredIDs)
	assert.Equal(t, "id-db", m.state.lastFilteredSelectedID)

	m.state.filterQuery = "b"
	m.applyFilter()

	assert.Equal(t, []string{"id-web", "id-db"}, m.state.filteredIDs)
	assert.Equal(t, 1, m.state.selected)

	svc := m.state.services[m.state.filteredIDs[m.state.selected]]
	assert.Equal(t, "db", svc.Name)
}

func Test_ApplyFilter_TracksSelectionOnZeroMatches(t *testing.T) {
	m := Model{}
	m.state.serviceIDs = []string{"id-api", "id-web", "id-db"}
	m.state.services = map[string]*ServiceState{
		"id-api": {ID: "id-api", Name: "api"},
		"id-web": {ID: "id-web", Name: "web"},
		"id-db":  {ID: "id-db", Name: "db"},
	}
	m.state.tiers = []Tier{
		{Name: "tier1", Services: []string{"id-api", "id-web", "id-db"}},
	}
	m.state.filteredIDs = []string{"id-web", "id-db"}
	m.state.filteredTiers = []Tier{
		{Name: "tier1", Services: []string{"id-web", "id-db"}},
	}
	m.state.selected = 1
	m.state.filterQuery = "zzz"

	m.applyFilter()

	assert.Empty(t, m.state.filteredIDs)
	assert.Equal(t, "id-db", m.state.lastFilteredSelectedID)
}

func Test_HandleFilterInput_EscapeRestoresLastMatchNotPreFilter(t *testing.T) {
	m := Model{}
	m.state.filterActive = true
	m.state.filterQuery = "zzz"
	m.state.selected = 0
	m.state.filteredIDs = []string{}
	m.state.filteredTiers = []Tier{}
	m.state.preFilterSelectedID = "id-api"
	m.state.lastFilteredSelectedID = "id-web"
	m.state.serviceIDs = []string{"id-api", "id-web", "id-db"}
	m.state.services = map[string]*ServiceState{
		"id-api": {ID: "id-api", Name: "api"},
		"id-web": {ID: "id-web", Name: "web"},
		"id-db":  {ID: "id-db", Name: "db"},
	}
	m.state.tiers = []Tier{
		{Name: "tier1", Services: []string{"id-api", "id-web", "id-db"}},
	}

	msg := tea.KeyPressMsg{Code: tea.KeyEscape}
	teaModel, _ := m.handleFilterInput(msg)
	result := teaModel.(Model)

	assert.Equal(t, 1, result.state.selected)
	assert.Empty(t, result.state.lastFilteredSelectedID)
}

func Test_HandleFilterInput_EscapeClearsFilteredState(t *testing.T) {
	m := Model{}
	m.state.filterActive = true
	m.state.filterQuery = "api"
	m.state.selected = 0
	m.state.filteredIDs = []string{"id-api"}
	m.state.filteredTiers = []Tier{{Name: "tier1", Services: []string{"id-api"}}}
	m.state.serviceIDs = []string{"id-api", "id-web"}
	m.state.services = map[string]*ServiceState{
		"id-api": {ID: "id-api", Name: "api"},
		"id-web": {ID: "id-web", Name: "web"},
	}
	m.state.tiers = []Tier{
		{Name: "tier1", Services: []string{"id-api", "id-web"}},
	}

	msg := tea.KeyPressMsg{Code: tea.KeyEscape}
	teaModel, _ := m.handleFilterInput(msg)
	result := teaModel.(Model)

	assert.Empty(t, result.state.filterQuery)
	assert.False(t, result.state.filterActive)
	assert.Nil(t, result.state.filteredIDs)
	assert.Nil(t, result.state.filteredTiers)
	assert.Equal(t, 0, result.state.selected)
}

func Test_HandleFilterInput_EscapeRestoresSelectionPosition(t *testing.T) {
	m := Model{}
	m.state.filterActive = true
	m.state.filterQuery = "web"
	m.state.selected = 0
	m.state.filteredIDs = []string{"id-web"}
	m.state.filteredTiers = []Tier{{Name: "tier1", Services: []string{"id-web"}}}
	m.state.serviceIDs = []string{"id-api", "id-web", "id-db"}
	m.state.services = map[string]*ServiceState{
		"id-api": {ID: "id-api", Name: "api"},
		"id-web": {ID: "id-web", Name: "web"},
		"id-db":  {ID: "id-db", Name: "db"},
	}
	m.state.tiers = []Tier{
		{Name: "tier1", Services: []string{"id-api", "id-web", "id-db"}},
	}

	msg := tea.KeyPressMsg{Code: tea.KeyEscape}
	teaModel, _ := m.handleFilterInput(msg)
	result := teaModel.(Model)

	assert.Equal(t, 1, result.state.selected)
}

func Test_HandleFilterInput_EscapePreservesSelectionWithNoQuery(t *testing.T) {
	m := Model{}
	m.state.filterActive = true
	m.state.filterQuery = ""
	m.state.selected = 2
	m.state.preFilterSelectedID = "id-api"
	m.state.serviceIDs = []string{"id-api", "id-web", "id-db"}
	m.state.services = map[string]*ServiceState{
		"id-api": {ID: "id-api", Name: "api"},
		"id-web": {ID: "id-web", Name: "web"},
		"id-db":  {ID: "id-db", Name: "db"},
	}
	m.state.tiers = []Tier{
		{Name: "tier1", Services: []string{"id-api", "id-web", "id-db"}},
	}

	msg := tea.KeyPressMsg{Code: tea.KeyEscape}
	teaModel, _ := m.handleFilterInput(msg)
	result := teaModel.(Model)

	assert.Equal(t, 2, result.state.selected)
	assert.Empty(t, result.state.filterQuery)
	assert.False(t, result.state.filterActive)
}

func Test_HandleFilterInput_EscapeRestoresSelectionAfterZeroMatches(t *testing.T) {
	m := Model{}
	m.state.filterActive = true
	m.state.filterQuery = "zzz"
	m.state.selected = 0
	m.state.filteredIDs = []string{}
	m.state.filteredTiers = []Tier{}
	m.state.preFilterSelectedID = "id-db"
	m.state.serviceIDs = []string{"id-api", "id-web", "id-db"}
	m.state.services = map[string]*ServiceState{
		"id-api": {ID: "id-api", Name: "api"},
		"id-web": {ID: "id-web", Name: "web"},
		"id-db":  {ID: "id-db", Name: "db"},
	}
	m.state.tiers = []Tier{
		{Name: "tier1", Services: []string{"id-api", "id-web", "id-db"}},
	}

	msg := tea.KeyPressMsg{Code: tea.KeyEscape}
	teaModel, _ := m.handleFilterInput(msg)
	result := teaModel.(Model)

	assert.Equal(t, 2, result.state.selected)
	assert.Empty(t, result.state.preFilterSelectedID)
}

func Test_HandleKeyPress_EscClearsFilterAfterEnter(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockController := NewMockController(ctrl)
	loader := &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}

	m := Model{loader: loader, controller: mockController}
	m.state.filterActive = false
	m.state.filterQuery = "web"
	m.state.filteredIDs = []string{"id-web"}
	m.state.filteredTiers = []Tier{{Name: "tier1", Services: []string{"id-web"}}}
	m.state.serviceIDs = []string{"id-api", "id-web"}
	m.state.services = map[string]*ServiceState{
		"id-api": {ID: "id-api", Name: "api"},
		"id-web": {ID: "id-web", Name: "web"},
	}
	m.state.tiers = []Tier{
		{Name: "tier1", Services: []string{"id-api", "id-web"}},
	}
	m.ui.servicesKeys = DefaultKeyMap()

	msg := tea.KeyPressMsg{Code: tea.KeyEscape}
	teaModel, _ := m.handleKeyPress(msg)
	result := teaModel.(Model)

	assert.Empty(t, result.state.filterQuery)
	assert.False(t, result.state.filterActive)
	assert.Nil(t, result.state.filteredIDs)
	assert.Nil(t, result.state.filteredTiers)
}

func Test_HandleKeyPress_EscClearsSeparatorOnlyQuery(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockController := NewMockController(ctrl)
	loader := &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}

	m := Model{loader: loader, controller: mockController}
	m.state.filterActive = false
	m.state.filterQuery = "---"
	m.state.filteredIDs = []string{"id-api"}
	m.state.filteredTiers = []Tier{{Name: "tier1", Services: []string{"id-api"}}}
	m.state.serviceIDs = []string{"id-api", "id-web"}
	m.state.services = map[string]*ServiceState{
		"id-api": {ID: "id-api", Name: "api"},
		"id-web": {ID: "id-web", Name: "web"},
	}
	m.state.tiers = []Tier{
		{Name: "tier1", Services: []string{"id-api", "id-web"}},
	}
	m.ui.servicesKeys = DefaultKeyMap()

	msg := tea.KeyPressMsg{Code: tea.KeyEscape}
	teaModel, _ := m.handleKeyPress(msg)
	result := teaModel.(Model)

	assert.Empty(t, result.state.filterQuery)
	assert.False(t, result.state.filterActive)
	assert.Nil(t, result.state.filteredIDs)
	assert.Nil(t, result.state.filteredTiers)
}

func Test_HandleUpKey_WithFilter(t *testing.T) {
	m := Model{}
	m.state.serviceIDs = []string{"id-api", "id-web", "id-db"}
	m.state.services = map[string]*ServiceState{
		"id-api": {ID: "id-api", Name: "api"},
		"id-web": {ID: "id-web", Name: "web"},
		"id-db":  {ID: "id-db", Name: "db"},
	}
	m.state.tiers = []Tier{
		{Name: "tier1", Services: []string{"id-api", "id-web", "id-db"}},
	}
	m.state.filterQuery = "b"
	m.state.filteredIDs = []string{"id-web", "id-db"}
	m.state.filteredTiers = []Tier{
		{Name: "tier1", Services: []string{"id-web", "id-db"}},
	}
	m.state.selected = 1

	teaModel, _ := m.handleUpKey()
	result := teaModel.(Model)

	assert.Equal(t, 0, result.state.selected)
}

func Test_HandleUpKey_WithFilter_AtTop(t *testing.T) {
	m := Model{}
	m.state.serviceIDs = []string{"id-api", "id-web", "id-db"}
	m.state.services = map[string]*ServiceState{
		"id-api": {ID: "id-api", Name: "api"},
		"id-web": {ID: "id-web", Name: "web"},
		"id-db":  {ID: "id-db", Name: "db"},
	}
	m.state.filterQuery = "api"
	m.state.filteredIDs = []string{"id-api"}
	m.state.filteredTiers = []Tier{
		{Name: "tier1", Services: []string{"id-api"}},
	}
	m.state.selected = 0

	teaModel, _ := m.handleUpKey()
	result := teaModel.(Model)

	assert.Equal(t, 0, result.state.selected)
}

func Test_HandleDownKey_WithFilter(t *testing.T) {
	m := Model{}
	m.state.serviceIDs = []string{"id-api", "id-web", "id-db"}
	m.state.services = map[string]*ServiceState{
		"id-api": {ID: "id-api", Name: "api"},
		"id-web": {ID: "id-web", Name: "web"},
		"id-db":  {ID: "id-db", Name: "db"},
	}
	m.state.tiers = []Tier{
		{Name: "tier1", Services: []string{"id-api", "id-web", "id-db"}},
	}
	m.state.filterQuery = "b"
	m.state.filteredIDs = []string{"id-web", "id-db"}
	m.state.filteredTiers = []Tier{
		{Name: "tier1", Services: []string{"id-web", "id-db"}},
	}
	m.state.selected = 0

	teaModel, _ := m.handleDownKey()
	result := teaModel.(Model)

	assert.Equal(t, 1, result.state.selected)
}

func Test_HandleDownKey_WithFilter_AtBottom(t *testing.T) {
	m := Model{}
	m.state.serviceIDs = []string{"id-api", "id-web", "id-db"}
	m.state.services = map[string]*ServiceState{
		"id-api": {ID: "id-api", Name: "api"},
		"id-web": {ID: "id-web", Name: "web"},
		"id-db":  {ID: "id-db", Name: "db"},
	}
	m.state.filterQuery = "api"
	m.state.filteredIDs = []string{"id-api"}
	m.state.filteredTiers = []Tier{
		{Name: "tier1", Services: []string{"id-api"}},
	}
	m.state.selected = 0

	teaModel, _ := m.handleDownKey()
	result := teaModel.(Model)

	assert.Equal(t, 0, result.state.selected)
}

func Test_HandleDownKey_WithFilter_ZeroMatches(t *testing.T) {
	m := Model{}
	m.state.serviceIDs = []string{"id-api", "id-web"}
	m.state.services = map[string]*ServiceState{
		"id-api": {ID: "id-api", Name: "api"},
		"id-web": {ID: "id-web", Name: "web"},
	}
	m.state.filterQuery = "xyz"
	m.state.filteredIDs = []string{}
	m.state.filteredTiers = []Tier{}
	m.state.selected = 0

	teaModel, _ := m.handleDownKey()
	result := teaModel.(Model)

	assert.Equal(t, 0, result.state.selected)
}

func Test_HandleStopKey_WithFilter(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockController := NewMockController(ctrl)
	mockController.EXPECT().Stop(bus.Service{ID: "id-web", Name: "web"})

	loader := &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}

	m := Model{loader: loader, controller: mockController}
	m.state.serviceIDs = []string{"id-api", "id-web", "id-db"}
	m.state.services = map[string]*ServiceState{
		"id-api": {ID: "id-api", Name: "api", Status: StatusRunning},
		"id-web": {ID: "id-web", Name: "web", Status: StatusRunning},
		"id-db":  {ID: "id-db", Name: "db", Status: StatusRunning},
	}
	m.state.filterQuery = "web"
	m.state.filteredIDs = []string{"id-web"}
	m.state.filteredTiers = []Tier{
		{Name: "tier1", Services: []string{"id-web"}},
	}
	m.state.selected = 0

	teaModel, cmd := m.handleStopKey()
	result := teaModel.(Model)

	assert.True(t, result.loader.Has("id-web"))
	assert.NotNil(t, cmd)
}

func Test_HandleStopKey_ZeroMatches_IsNoop(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockController := NewMockController(ctrl)
	loader := &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}

	m := Model{loader: loader, controller: mockController}
	m.state.serviceIDs = []string{"id-api"}
	m.state.services = map[string]*ServiceState{
		"id-api": {ID: "id-api", Name: "api", Status: StatusRunning},
	}
	m.state.filterQuery = "xyz"
	m.state.filteredIDs = []string{}
	m.state.filteredTiers = []Tier{}
	m.state.selected = 0

	teaModel, cmd := m.handleStopKey()
	result := teaModel.(Model)

	assert.False(t, result.loader.Active)
	assert.Nil(t, cmd)
}

func Test_HandleRestartKey_WithFilter(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockController := NewMockController(ctrl)
	mockController.EXPECT().Restart(bus.Service{ID: "id-db", Name: "db"})

	loader := &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}

	m := Model{loader: loader, controller: mockController}
	m.state.serviceIDs = []string{"id-api", "id-web", "id-db"}
	m.state.services = map[string]*ServiceState{
		"id-api": {ID: "id-api", Name: "api", Status: StatusRunning},
		"id-web": {ID: "id-web", Name: "web", Status: StatusRunning},
		"id-db":  {ID: "id-db", Name: "db", Status: StatusRunning},
	}
	m.state.filterQuery = "db"
	m.state.filteredIDs = []string{"id-db"}
	m.state.filteredTiers = []Tier{
		{Name: "tier1", Services: []string{"id-db"}},
	}
	m.state.selected = 0

	teaModel, cmd := m.handleRestartKey()
	result := teaModel.(Model)

	assert.True(t, result.loader.Has("id-db"))
	assert.NotNil(t, cmd)
}

func Test_HandleRestartKey_ZeroMatches_IsNoop(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockController := NewMockController(ctrl)
	loader := &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}

	m := Model{loader: loader, controller: mockController}
	m.state.serviceIDs = []string{"id-api"}
	m.state.services = map[string]*ServiceState{
		"id-api": {ID: "id-api", Name: "api", Status: StatusRunning},
	}
	m.state.filterQuery = "xyz"
	m.state.filteredIDs = []string{}
	m.state.filteredTiers = []Tier{}
	m.state.selected = 0

	teaModel, cmd := m.handleRestartKey()
	result := teaModel.(Model)

	assert.False(t, result.loader.Active)
	assert.Nil(t, cmd)
}

func Test_CalculateScrollOffset_WithFilter(t *testing.T) {
	m := Model{}
	m.state.serviceIDs = []string{"id-api", "id-web", "id-db", "id-cache"}
	m.state.services = map[string]*ServiceState{
		"id-api":   {ID: "id-api", Name: "api"},
		"id-web":   {ID: "id-web", Name: "web"},
		"id-db":    {ID: "id-db", Name: "db"},
		"id-cache": {ID: "id-cache", Name: "cache"},
	}
	m.state.tiers = []Tier{
		{Name: "tier1", Services: []string{"id-api", "id-web"}},
		{Name: "tier2", Services: []string{"id-db", "id-cache"}},
	}
	m.state.filterQuery = "b"
	m.state.filteredIDs = []string{"id-web", "id-db"}
	m.state.filteredTiers = []Tier{
		{Name: "tier1", Services: []string{"id-web"}},
		{Name: "tier2", Services: []string{"id-db"}},
	}
	m.state.selected = 1
	m.ui.servicesViewport = viewport.New()
	m.ui.servicesViewport.SetWidth(80)
	m.ui.servicesViewport.SetHeight(3)

	offset := m.calculateScrollOffset()

	assert.Equal(t, 3, offset)
}

func Test_HandleProfileResolved_CreatesTimelines(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLog := logger.NewMockLogger(ctrl)
	noopLogger := zerolog.New(io.Discard)
	mockLog.EXPECT().Debug().Return(noopLogger.Debug()).AnyTimes()
	mockLog.EXPECT().Info().Return(noopLogger.Info()).AnyTimes()
	mockLog.EXPECT().Warn().Return(noopLogger.Warn()).AnyTimes()
	mockLog.EXPECT().Error().Return(noopLogger.Error()).AnyTimes()

	m := Model{log: mockLog, loader: NewLoader()}
	m.state.services = make(map[string]*ServiceState)
	m.state.restarting = make(map[string]bool)
	m.state.tiers = make([]Tier, 0)

	event := bus.Message{
		Type: bus.EventProfileResolved,
		Data: bus.ProfileResolved{
			Profile: "dev",
			Tiers: []bus.Tier{
				{Name: "tier1", Services: []bus.Service{{ID: "test-id-db", Name: "db"}, {ID: "test-id-api", Name: "api"}}},
			},
		},
	}

	result := m.handleProfileResolved(event)

	for _, id := range []string{"test-id-db", "test-id-api"} {
		svc := result.state.services[id]
		require.NotNil(t, svc.Timeline, "service %s should have a timeline", id)
		assert.Equal(t, components.DefaultTimelineSlots, svc.Timeline.capacity, "timeline capacity should be %d", components.DefaultTimelineSlots)
		assert.Equal(t, 0, svc.Timeline.count, "timeline should start empty")
	}
}

func Test_HandleProfileResolved_FreshTimelinesOnReload(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLog := logger.NewMockLogger(ctrl)
	noopLogger := zerolog.New(io.Discard)
	mockLog.EXPECT().Debug().Return(noopLogger.Debug()).AnyTimes()
	mockLog.EXPECT().Info().Return(noopLogger.Info()).AnyTimes()
	mockLog.EXPECT().Warn().Return(noopLogger.Warn()).AnyTimes()
	mockLog.EXPECT().Error().Return(noopLogger.Error()).AnyTimes()

	m := Model{log: mockLog, loader: NewLoader()}
	m.state.services = make(map[string]*ServiceState)
	m.state.restarting = make(map[string]bool)
	m.state.tiers = make([]Tier, 0)

	event1 := bus.Message{
		Type: bus.EventProfileResolved,
		Data: bus.ProfileResolved{
			Profile: "profile1",
			Tiers: []bus.Tier{
				{Name: "tier1", Services: []bus.Service{{ID: "test-id-db", Name: "db"}}},
			},
		},
	}

	result := m.handleProfileResolved(event1)
	result.state.services["test-id-db"].Timeline.Append(SlotRunning)
	result.state.services["test-id-db"].Timeline.Append(SlotRunning)
	assert.Equal(t, 2, result.state.services["test-id-db"].Timeline.count)

	event2 := bus.Message{
		Type: bus.EventProfileResolved,
		Data: bus.ProfileResolved{
			Profile: "profile2",
			Tiers: []bus.Tier{
				{Name: "tier1", Services: []bus.Service{{ID: "test-id-db", Name: "db"}}},
			},
		},
	}

	result = result.handleProfileResolved(event2)
	assert.Equal(t, 0, result.state.services["test-id-db"].Timeline.count, "timeline should be fresh after profile reload")
}

func Test_SampleTimelines(t *testing.T) {
	started := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	m := Model{}
	m.state.services = map[string]*ServiceState{
		"id-api": {
			ID:        "id-api",
			Name:      "api",
			Status:    StatusRunning,
			StartTime: started,
			Timeline:  NewTimeline(components.DefaultTimelineSlots),
		},
		"id-db": {
			ID:        "id-db",
			Name:      "db",
			Status:    StatusStarting,
			StartTime: started,
			Timeline:  NewTimeline(components.DefaultTimelineSlots),
		},
		"id-web": {
			ID:        "id-web",
			Name:      "web",
			Status:    StatusFailed,
			StartTime: started,
			Timeline:  NewTimeline(components.DefaultTimelineSlots),
		},
		"id-queued": {
			ID:       "id-queued",
			Name:     "queued",
			Status:   StatusStarting,
			Timeline: NewTimeline(components.DefaultTimelineSlots),
		},
		"id-stopped": {
			ID:       "id-stopped",
			Name:     "stopped",
			Status:   StatusStopped,
			Timeline: timelineWithHistory(SlotRunning),
		},
		"id-preflight-fail": {
			ID:       "id-preflight-fail",
			Name:     "preflight-fail",
			Status:   StatusFailed,
			Timeline: NewTimeline(components.DefaultTimelineSlots),
		},
	}

	m.sampleTimelines()

	assert.Equal(t, 1, m.state.services["id-api"].Timeline.count)
	assert.Equal(t, SlotRunning, m.state.services["id-api"].Timeline.Slots()[0])

	assert.Equal(t, 1, m.state.services["id-db"].Timeline.count)
	assert.Equal(t, SlotStarting, m.state.services["id-db"].Timeline.Slots()[0])

	assert.Equal(t, 1, m.state.services["id-web"].Timeline.count)
	assert.Equal(t, SlotFailed, m.state.services["id-web"].Timeline.Slots()[0])

	assert.Equal(t, 0, m.state.services["id-queued"].Timeline.count)

	assert.Equal(t, 2, m.state.services["id-stopped"].Timeline.count)
	assert.Equal(t, SlotStopped, m.state.services["id-stopped"].Timeline.Slots()[1])

	assert.Equal(t, 1, m.state.services["id-preflight-fail"].Timeline.count)
	assert.Equal(t, SlotFailed, m.state.services["id-preflight-fail"].Timeline.Slots()[0])
}

func Test_SampleTimelines_MultipleSamples(t *testing.T) {
	started := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	m := Model{}
	m.state.services = map[string]*ServiceState{
		"id-api": {
			ID:        "id-api",
			Name:      "api",
			Status:    StatusStarting,
			StartTime: started,
			Timeline:  NewTimeline(components.DefaultTimelineSlots),
		},
	}

	m.sampleTimelines()
	m.state.services["id-api"].Status = StatusRunning
	m.sampleTimelines()
	m.sampleTimelines()

	slots := m.state.services["id-api"].Timeline.Slots()
	assert.Equal(t, SlotStarting, slots[0])
	assert.Equal(t, SlotRunning, slots[1])
	assert.Equal(t, SlotRunning, slots[2])
	assert.Equal(t, SlotEmpty, slots[3])
}

func timelineWithHistory(slots ...TimelineSlot) *Timeline {
	tl := NewTimeline(components.DefaultTimelineSlots)
	for _, s := range slots {
		tl.Append(s)
	}

	return tl
}

func Test_RefreshFromStore_StaleSnapshotDoesNotRevertStatus(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	t1 := t0.Add(time.Second)

	mockStore := registry.NewMockStore(ctrl)
	mockStore.EXPECT().Services().Return([]registry.ServiceSnapshot{
		{
			ID:           "id-api",
			Status:       registry.StatusStarting,
			PID:          1234,
			CPU:          10.0,
			Memory:       128 * 1024 * 1024,
			StartTime:    t0,
			LifecycleAt:  t0,
			LifecycleSeq: 1,
		},
	})

	m := &Model{store: mockStore, loader: &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}}
	m.state.restarting = map[string]bool{}
	m.state.services = map[string]*ServiceState{
		"id-api": {
			ID:           "id-api",
			Name:         "api",
			Status:       StatusFailed,
			PID:          0,
			StartTime:    time.Time{},
			Error:        assert.AnError,
			LifecycleAt:  t1,
			LifecycleSeq: 2,
			Timeline:     timelineWithHistory(SlotStarting, SlotFailed),
		},
	}

	m.refreshFromStore()
	m.sampleTimelines()

	api := m.state.services["id-api"]
	assert.Equal(t, StatusFailed, api.Status)
	require.EqualError(t, api.Error, assert.AnError.Error())
	assert.True(t, api.StartTime.IsZero())
	assert.Equal(t, 0, api.PID)
	assert.InDelta(t, 0.0, api.CPU, 0.01)
	assert.InDelta(t, 0.0, api.MEM, 0.01)

	slots := api.Timeline.Slots()
	assert.Equal(t, SlotStarting, slots[0])
	assert.Equal(t, SlotFailed, slots[1])
	assert.Equal(t, SlotFailed, slots[2])
}

func Test_DelayedStartingDoesNotRevertReady(t *testing.T) {
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	t1 := t0.Add(time.Second)
	t2 := t0.Add(2 * time.Second)

	m := Model{loader: &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}}
	m.state.services = map[string]*ServiceState{
		"id-api": {
			ID:       "id-api",
			Name:     "api",
			Timeline: NewTimeline(components.DefaultTimelineSlots),
		},
	}
	m.state.restarting = map[string]bool{}

	m = m.handleServiceReady(bus.Message{
		Type:      bus.EventServiceReady,
		Timestamp: t2,
		Seq:       2,
		Data: bus.ServiceReady{
			ServiceEvent: bus.ServiceEvent{Service: bus.Service{ID: "id-api", Name: "api"}, Tier: "platform"},
			PID:          5678,
			StartedAt:    t1,
		},
	})

	m = m.handleServiceStarting(bus.Message{
		Type:      bus.EventServiceStarting,
		Timestamp: t1,
		Seq:       1,
		Data:      bus.ServiceStarting{ServiceEvent: bus.ServiceEvent{Service: bus.Service{ID: "id-api", Name: "api"}}, PID: 5678},
	})

	api := m.state.services["id-api"]
	assert.Equal(t, StatusRunning, api.Status)
	assert.Equal(t, 5678, api.PID)
	assert.Equal(t, t1, api.StartTime)
	assert.Equal(t, t2, api.LifecycleAt)
}

func Test_TimelineSamplesReconciledState(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	t1 := t0.Add(time.Second)

	mockStore := registry.NewMockStore(ctrl)
	mockStore.EXPECT().Services().Return([]registry.ServiceSnapshot{
		{
			ID:           "id-api",
			Status:       registry.StatusRunning,
			PID:          5678,
			StartTime:    t0,
			LifecycleAt:  t1,
			LifecycleSeq: 2,
		},
	})

	m := &Model{store: mockStore, loader: &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}}
	m.state.restarting = map[string]bool{}
	m.state.services = map[string]*ServiceState{
		"id-api": {
			ID:            "id-api",
			Name:          "api",
			Status:        StatusStarting,
			PID:           5678,
			StartTime:     t0,
			LifecycleAt:   t0,
			LifecycleSeq:  1,
			BackfilledSeq: 2,
			Timeline:      timelineWithHistory(SlotStarting),
		},
	}

	m.refreshFromStore()
	m.sampleTimelines()

	api := m.state.services["id-api"]
	assert.Equal(t, registry.StatusRunning, api.Status)

	slots := api.Timeline.Slots()
	assert.Equal(t, SlotStarting, slots[0])
	assert.Equal(t, SlotRunning, slots[1])
}

func Test_HandleServiceReady_ClearsErrorAndRestarting(t *testing.T) {
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	t1 := t0.Add(time.Second)

	m := Model{loader: &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}}
	m.state.services = map[string]*ServiceState{
		"id-api": {
			ID:          "id-api",
			Name:        "api",
			Status:      StatusRestarting,
			Error:       assert.AnError,
			LifecycleAt: t0,
		},
	}
	m.state.restarting = map[string]bool{"id-api": true}

	m = m.handleServiceReady(bus.Message{
		Type:      bus.EventServiceReady,
		Timestamp: t1,
		Seq:       2,
		Data: bus.ServiceReady{
			ServiceEvent: bus.ServiceEvent{Service: bus.Service{ID: "id-api", Name: "api"}, Tier: "platform"},
			PID:          9876,
			StartedAt:    t0,
		},
	})

	api := m.state.services["id-api"]
	assert.Equal(t, StatusRunning, api.Status)
	assert.Equal(t, 9876, api.PID)
	assert.Equal(t, t0, api.StartTime)
	require.NoError(t, api.Error)
	assert.False(t, m.state.restarting["id-api"])
}

func Test_SameSeqReadyHealingAllowsRunningTimelineSample(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	mockStore := registry.NewMockStore(ctrl)
	mockStore.EXPECT().Services().Return([]registry.ServiceSnapshot{
		{
			ID:           "id-api",
			Status:       registry.StatusRunning,
			PID:          5678,
			StartTime:    t0,
			LifecycleAt:  t0,
			LifecycleSeq: 10,
		},
	})

	m := &Model{store: mockStore, loader: &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}}
	m.state.restarting = map[string]bool{}
	m.state.services = map[string]*ServiceState{
		"id-api": {
			ID:           "id-api",
			Name:         "api",
			Status:       StatusRunning,
			LifecycleAt:  t0,
			LifecycleSeq: 10,
			Timeline:     NewTimeline(components.DefaultTimelineSlots),
		},
	}

	m.refreshFromStore()
	m.sampleTimelines()

	api := m.state.services["id-api"]
	assert.Equal(t, t0, api.StartTime)
	assert.Equal(t, 1, api.Timeline.Count())
	assert.Equal(t, SlotRunning, api.Timeline.Slots()[0])
}

func Test_HandleServiceReady_BackfillsStartupHistory(t *testing.T) {
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	m := Model{loader: &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}}
	m.state.services = map[string]*ServiceState{
		"id-api": {
			ID:       "id-api",
			Name:     "api",
			Status:   StatusStarting,
			Timeline: NewTimeline(components.DefaultTimelineSlots),
		},
	}
	m.state.restarting = map[string]bool{}

	m = m.handleServiceReady(bus.Message{
		Type:      bus.EventServiceReady,
		Timestamp: t0.Add(3 * time.Second),
		Seq:       2,
		Data: bus.ServiceReady{
			ServiceEvent: bus.ServiceEvent{Service: bus.Service{ID: "id-api", Name: "api"}},
			PID:          1234,
			StartedAt:    t0,
			Duration:     3 * time.Second,
		},
	})

	tl := m.state.services["id-api"].Timeline
	assert.Equal(t, 3, tl.Count())

	slots := tl.Slots()
	assert.Equal(t, SlotStarting, slots[0])
	assert.Equal(t, SlotStarting, slots[1])
	assert.Equal(t, SlotStarting, slots[2])
}

func Test_HandleServiceReady_NoBackfillWhenHistoryExists(t *testing.T) {
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	tl := NewTimeline(components.DefaultTimelineSlots)
	tl.Append(SlotStarting)
	tl.Append(SlotStarting)

	m := Model{loader: &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}}
	m.state.services = map[string]*ServiceState{
		"id-api": {
			ID:               "id-api",
			Name:             "api",
			Status:           StatusStarting,
			PID:              1234,
			StartTime:        t0,
			AttemptStartedAt: t0,
			StartupSampled:   2,
			BackfilledSeq:    2,
			Timeline:         tl,
		},
	}
	m.state.restarting = map[string]bool{}

	m = m.handleServiceReady(bus.Message{
		Type:      bus.EventServiceReady,
		Timestamp: t0.Add(3 * time.Second),
		Seq:       2,
		Data: bus.ServiceReady{
			ServiceEvent: bus.ServiceEvent{Service: bus.Service{ID: "id-api", Name: "api"}},
			PID:          1234,
			StartedAt:    t0,
			Duration:     3 * time.Second,
		},
	})

	assert.Equal(t, 2, m.state.services["id-api"].Timeline.Count())
}

func Test_HandleServiceReady_ZerosMetricsOnNewProcess(t *testing.T) {
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	m := Model{loader: &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}}
	m.state.services = map[string]*ServiceState{
		"id-api": {
			ID:     "id-api",
			Name:   "api",
			Status: StatusStarting,
			CPU:    45.0,
			MEM:    512.0,
		},
	}
	m.state.restarting = map[string]bool{}

	m = m.handleServiceReady(bus.Message{
		Type:      bus.EventServiceReady,
		Timestamp: t0,
		Seq:       1,
		Data: bus.ServiceReady{
			ServiceEvent: bus.ServiceEvent{Service: bus.Service{ID: "id-api", Name: "api"}},
			PID:          5678,
			StartedAt:    t0,
		},
	})

	api := m.state.services["id-api"]
	assert.InDelta(t, 0.0, api.CPU, 0.01)
	assert.InDelta(t, 0.0, api.MEM, 0.01)
}

func Test_HandleServiceReady_PreservesMetricsOnSameProcess(t *testing.T) {
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	m := Model{loader: &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}}
	m.state.services = map[string]*ServiceState{
		"id-api": {
			ID:               "id-api",
			Name:             "api",
			Status:           StatusStarting,
			PID:              5678,
			StartTime:        t0,
			AttemptStartedAt: t0,
			CPU:              25.5,
			MEM:              256.0,
		},
	}
	m.state.restarting = map[string]bool{}

	m = m.handleServiceReady(bus.Message{
		Type:      bus.EventServiceReady,
		Timestamp: t0.Add(time.Second),
		Seq:       2,
		Data: bus.ServiceReady{
			ServiceEvent: bus.ServiceEvent{Service: bus.Service{ID: "id-api", Name: "api"}},
			PID:          5678,
			StartedAt:    t0,
		},
	})

	api := m.state.services["id-api"]
	assert.InDelta(t, 25.5, api.CPU, 0.01)
	assert.InDelta(t, 256.0, api.MEM, 0.01)
}

func Test_RefreshBeforeReady_StillBackfillsStartupHistory(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	t1 := t0.Add(3 * time.Second)

	mockStore := registry.NewMockStore(ctrl)
	mockStore.EXPECT().Services().Return([]registry.ServiceSnapshot{
		{
			ID:               "id-api",
			Status:           registry.StatusRunning,
			PID:              5678,
			StartTime:        t0,
			AttemptStartedAt: t0,
			LifecycleAt:      t1,
			LifecycleSeq:     5,
		},
	})

	m := &Model{
		store:  mockStore,
		loader: &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)},
	}
	m.state.services = map[string]*ServiceState{
		"id-api": {
			ID:       "id-api",
			Name:     "api",
			Status:   StatusStarting,
			Timeline: NewTimeline(components.DefaultTimelineSlots),
		},
	}
	m.state.restarting = map[string]bool{}

	m.refreshFromStore()
	m.sampleTimelines()

	tl := m.state.services["id-api"].Timeline
	slots := tl.Slots()
	assert.Equal(t, 4, tl.Count())
	assert.Equal(t, SlotStarting, slots[0])
	assert.Equal(t, SlotStarting, slots[1])
	assert.Equal(t, SlotStarting, slots[2])
	assert.Equal(t, SlotRunning, slots[3])

	m2 := m.handleServiceReady(bus.Message{
		Type:      bus.EventServiceReady,
		Timestamp: t1,
		Seq:       5,
		Data: bus.ServiceReady{
			ServiceEvent: bus.ServiceEvent{Service: bus.Service{ID: "id-api", Name: "api"}},
			PID:          5678,
			StartedAt:    t0,
			Duration:     3 * time.Second,
		},
	})

	assert.Equal(t, 4, m2.state.services["id-api"].Timeline.Count())
}

func Test_SameSeqReadyAfterStoreHeal_RunsSideEffects(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	mockStore := registry.NewMockStore(ctrl)
	mockStore.EXPECT().Services().Return([]registry.ServiceSnapshot{
		{
			ID:           "id-api",
			Status:       registry.StatusRunning,
			PID:          5678,
			StartTime:    t0,
			LifecycleAt:  t0,
			LifecycleSeq: 5,
		},
	})

	loader := &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}
	loader.Start("id-api", "starting api...")

	m := &Model{store: mockStore, loader: loader}
	m.state.services = map[string]*ServiceState{
		"id-api": {
			ID:       "id-api",
			Name:     "api",
			Status:   StatusStarting,
			Timeline: NewTimeline(components.DefaultTimelineSlots),
		},
	}
	m.state.restarting = map[string]bool{"id-api": true}

	m.refreshFromStore()

	assert.Equal(t, StatusRunning, m.state.services["id-api"].Status)
	assert.Equal(t, uint64(5), m.state.services["id-api"].LifecycleSeq)
	assert.False(t, m.loader.Active)
	assert.False(t, m.state.restarting["id-api"])

	result := m.handleServiceReady(bus.Message{
		Type:      bus.EventServiceReady,
		Timestamp: t0,
		Seq:       5,
		Data: bus.ServiceReady{
			ServiceEvent: bus.ServiceEvent{Service: bus.Service{ID: "id-api", Name: "api"}},
			PID:          5678,
			StartedAt:    t0.Add(-3 * time.Second),
			Duration:     3 * time.Second,
		},
	})

	assert.False(t, result.loader.Active)
	assert.False(t, result.loader.Has("id-api"))
	assert.False(t, result.state.restarting["id-api"])
	assert.Equal(t, 3, result.state.services["id-api"].Timeline.Count())
}

func Test_LateStartingDoesNotResetStartupSampled(t *testing.T) {
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	m := Model{loader: &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}}
	m.state.services = map[string]*ServiceState{
		"id-api": {
			ID:               "id-api",
			Name:             "api",
			Status:           StatusStarting,
			PID:              5678,
			StartTime:        t0,
			AttemptStartedAt: t0,
			StartupSampled:   3,
			BackfilledSeq:    0,
			LifecycleSeq:     5,
			Timeline:         NewTimeline(components.DefaultTimelineSlots),
		},
	}
	m.state.restarting = map[string]bool{}

	m = m.handleServiceStarting(bus.Message{
		Type:      bus.EventServiceStarting,
		Timestamp: t0,
		Seq:       5,
		Data: bus.ServiceStarting{
			ServiceEvent: bus.ServiceEvent{Service: bus.Service{ID: "id-api", Name: "api"}},
			PID:          5678,
			StartedAt:    t0,
		},
	})

	assert.Equal(t, 3, m.state.services["id-api"].StartupSampled)
}

func Test_RestartReadyBeforeStarting_BackfillsAmber(t *testing.T) {
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	t1 := t0.Add(3 * time.Second)

	tl := NewTimeline(components.DefaultTimelineSlots)
	tl.Append(SlotRunning)
	tl.Append(SlotRunning)

	m := Model{loader: &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}}
	m.state.services = map[string]*ServiceState{
		"id-api": {
			ID:             "id-api",
			Name:           "api",
			Status:         StatusRunning,
			PID:            1111,
			StartTime:      t0.Add(-10 * time.Second),
			StartupSampled: 2,
			BackfilledSeq:  1,
			LifecycleSeq:   1,
			Timeline:       tl,
		},
	}
	m.state.restarting = map[string]bool{}

	m = m.handleServiceReady(bus.Message{
		Type:      bus.EventServiceReady,
		Timestamp: t1,
		Seq:       3,
		Data: bus.ServiceReady{
			ServiceEvent: bus.ServiceEvent{Service: bus.Service{ID: "id-api", Name: "api"}},
			PID:          2222,
			StartedAt:    t0,
			Duration:     3 * time.Second,
		},
	})

	svc := m.state.services["id-api"]
	assert.Equal(t, 2222, svc.PID)
	assert.Equal(t, 5, svc.Timeline.Count())

	slots := svc.Timeline.Slots()
	assert.Equal(t, SlotRunning, slots[0])
	assert.Equal(t, SlotRunning, slots[1])
	assert.Equal(t, SlotStarting, slots[2])
	assert.Equal(t, SlotStarting, slots[3])
	assert.Equal(t, SlotStarting, slots[4])
}

func Test_SameSeqReadyAfterRefresh_NewProcessStillBackfills(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	t1 := t0.Add(2 * time.Second)

	mockStore := registry.NewMockStore(ctrl)
	mockStore.EXPECT().Services().Return([]registry.ServiceSnapshot{
		{
			ID:               "id-api",
			Status:           registry.StatusRunning,
			PID:              2222,
			StartTime:        t0,
			AttemptStartedAt: t0,
			LifecycleAt:      t1,
			LifecycleSeq:     5,
		},
	})

	oldStart := t0.Add(-10 * time.Second)

	m := &Model{
		store:  mockStore,
		loader: &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)},
	}
	m.state.services = map[string]*ServiceState{
		"id-api": {
			ID:               "id-api",
			Name:             "api",
			Status:           StatusRunning,
			PID:              1111,
			StartTime:        oldStart,
			AttemptStartedAt: oldStart,
			StartupSampled:   5,
			BackfilledSeq:    3,
			LifecycleSeq:     3,
			Timeline:         NewTimeline(components.DefaultTimelineSlots),
		},
	}
	m.state.restarting = map[string]bool{}

	m.refreshFromStore()

	svc := m.state.services["id-api"]
	assert.Equal(t, 2222, svc.PID)
	assert.Equal(t, 2, svc.Timeline.Count())

	slots := svc.Timeline.Slots()
	assert.Equal(t, SlotStarting, slots[0])
	assert.Equal(t, SlotStarting, slots[1])

	m2 := m.handleServiceReady(bus.Message{
		Type:      bus.EventServiceReady,
		Timestamp: t1,
		Seq:       5,
		Data: bus.ServiceReady{
			ServiceEvent: bus.ServiceEvent{Service: bus.Service{ID: "id-api", Name: "api"}},
			PID:          2222,
			StartedAt:    t0,
			Duration:     2 * time.Second,
		},
	})

	assert.Equal(t, 2, m2.state.services["id-api"].Timeline.Count())
}

func Test_ReconcileLifecycle_ClearsRestartingAndStopsLoader(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	mockStore := registry.NewMockStore(ctrl)
	mockStore.EXPECT().Services().Return([]registry.ServiceSnapshot{
		{
			ID:           "id-api",
			Status:       registry.StatusRunning,
			PID:          5678,
			StartTime:    t0,
			LifecycleAt:  t0.Add(3 * time.Second),
			LifecycleSeq: 5,
		},
	})

	loader := &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}
	loader.Start("id-api", "restarting api...")

	m := &Model{
		store:  mockStore,
		loader: loader,
	}
	m.state.services = map[string]*ServiceState{
		"id-api": {
			ID:           "id-api",
			Name:         "api",
			Status:       StatusRestarting,
			PID:          5678,
			StartTime:    t0,
			LifecycleSeq: 3,
		},
	}
	m.state.restarting = map[string]bool{"id-api": true}

	m.refreshFromStore()

	assert.Equal(t, StatusRunning, m.state.services["id-api"].Status)
	assert.False(t, m.loader.Active)
	assert.False(t, m.loader.Has("id-api"))
	assert.False(t, m.state.restarting["id-api"])
}

func Test_ReconcileLifecycle_StartingToFailed_BackfillsAmber(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	t1 := t0.Add(3 * time.Second)

	mockStore := registry.NewMockStore(ctrl)
	mockStore.EXPECT().Services().Return([]registry.ServiceSnapshot{
		{
			ID:           "id-api",
			Status:       registry.StatusFailed,
			Error:        "crash",
			LifecycleAt:  t1,
			LifecycleSeq: 5,
		},
	})

	m := &Model{
		store:  mockStore,
		loader: &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)},
	}
	m.state.services = map[string]*ServiceState{
		"id-api": {
			ID:               "id-api",
			Name:             "api",
			Status:           StatusStarting,
			StartupActive:    true,
			PID:              1234,
			StartTime:        t0,
			AttemptStartedAt: t0,
			LifecycleSeq:     3,
			Timeline:         NewTimeline(components.DefaultTimelineSlots),
		},
	}
	m.state.restarting = map[string]bool{}

	m.refreshFromStore()
	m.sampleTimelines()

	svc := m.state.services["id-api"]
	assert.Equal(t, StatusFailed, svc.Status)

	slots := svc.Timeline.Slots()
	assert.Equal(t, SlotStarting, slots[0])
	assert.Equal(t, SlotStarting, slots[1])
	assert.Equal(t, SlotStarting, slots[2])
	assert.Equal(t, SlotFailed, slots[3])
}

func Test_ReconcileLifecycle_StartingToStopped_BackfillsAmber(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	t1 := t0.Add(2 * time.Second)

	mockStore := registry.NewMockStore(ctrl)
	mockStore.EXPECT().Services().Return([]registry.ServiceSnapshot{
		{
			ID:           "id-api",
			Status:       registry.StatusStopped,
			LifecycleAt:  t1,
			LifecycleSeq: 5,
		},
	})

	m := &Model{
		store:  mockStore,
		loader: &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)},
	}
	m.state.services = map[string]*ServiceState{
		"id-api": {
			ID:               "id-api",
			Name:             "api",
			Status:           StatusStarting,
			StartupActive:    true,
			PID:              1234,
			StartTime:        t0,
			AttemptStartedAt: t0,
			LifecycleSeq:     3,
			Timeline:         NewTimeline(components.DefaultTimelineSlots),
		},
	}
	m.state.restarting = map[string]bool{}

	m.refreshFromStore()
	m.sampleTimelines()

	svc := m.state.services["id-api"]
	assert.Equal(t, StatusStopped, svc.Status)

	slots := svc.Timeline.Slots()
	assert.Equal(t, SlotStarting, slots[0])
	assert.Equal(t, SlotStarting, slots[1])
	assert.Equal(t, SlotStopped, slots[2])
}

func Test_ReconcileLifecycle_RestartingStopped_KeepsLoader(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	mockStore := registry.NewMockStore(ctrl)
	mockStore.EXPECT().Services().Return([]registry.ServiceSnapshot{
		{
			ID:           "id-api",
			Status:       registry.StatusStopped,
			LifecycleAt:  t0,
			LifecycleSeq: 5,
		},
	})

	loader := &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}
	loader.Start("id-api", "restarting api...")

	m := &Model{
		store:  mockStore,
		loader: loader,
	}
	m.state.services = map[string]*ServiceState{
		"id-api": {
			ID:           "id-api",
			Name:         "api",
			Status:       StatusRestarting,
			LifecycleSeq: 3,
		},
	}
	m.state.restarting = map[string]bool{"id-api": true}

	m.refreshFromStore()

	assert.Equal(t, StatusStopped, m.state.services["id-api"].Status)
	assert.True(t, m.loader.Active)
	assert.True(t, m.state.restarting["id-api"])
}

func Test_ReconcileLifecycle_RestartingWithOldStartTime_NoBackfill(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	mockStore := registry.NewMockStore(ctrl)
	mockStore.EXPECT().Services().Return([]registry.ServiceSnapshot{
		{
			ID:           "id-api",
			Status:       registry.StatusStopped,
			LifecycleAt:  t0.Add(30 * time.Second),
			LifecycleSeq: 5,
		},
	})

	m := &Model{
		store:  mockStore,
		loader: &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)},
	}
	m.state.services = map[string]*ServiceState{
		"id-api": {
			ID:           "id-api",
			Name:         "api",
			Status:       StatusStopping,
			PID:          1234,
			StartTime:    t0,
			LifecycleSeq: 3,
			Timeline:     NewTimeline(components.DefaultTimelineSlots),
		},
	}
	m.state.restarting = map[string]bool{}

	m.refreshFromStore()

	assert.Equal(t, 0, m.state.services["id-api"].Timeline.Count())
}

func Test_ReconcileLifecycle_RealStartingToFailed_StillBackfills(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	mockStore := registry.NewMockStore(ctrl)
	mockStore.EXPECT().Services().Return([]registry.ServiceSnapshot{
		{
			ID:           "id-api",
			Status:       registry.StatusFailed,
			Error:        "crash",
			LifecycleAt:  t0.Add(3 * time.Second),
			LifecycleSeq: 5,
		},
	})

	m := &Model{
		store:  mockStore,
		loader: &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)},
	}
	m.state.services = map[string]*ServiceState{
		"id-api": {
			ID:               "id-api",
			Name:             "api",
			Status:           StatusStarting,
			StartupActive:    true,
			PID:              1234,
			StartTime:        t0,
			AttemptStartedAt: t0,
			LifecycleSeq:     3,
			Timeline:         NewTimeline(components.DefaultTimelineSlots),
		},
	}
	m.state.restarting = map[string]bool{}

	m.refreshFromStore()

	assert.Equal(t, 3, m.state.services["id-api"].Timeline.Count())

	slots := m.state.services["id-api"].Timeline.Slots()
	assert.Equal(t, SlotStarting, slots[0])
	assert.Equal(t, SlotStarting, slots[1])
	assert.Equal(t, SlotStarting, slots[2])
}

func Test_ReconcileLifecycle_MissedStartup_FailedSnapshotBackfills(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	mockStore := registry.NewMockStore(ctrl)
	mockStore.EXPECT().Services().Return([]registry.ServiceSnapshot{
		{
			ID:               "id-api",
			Status:           registry.StatusFailed,
			Error:            "crash",
			PID:              0,
			AttemptStartedAt: t0,
			LifecycleAt:      t0.Add(3 * time.Second),
			LifecycleSeq:     5,
		},
	})

	m := &Model{
		store:  mockStore,
		loader: &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)},
	}
	m.state.services = map[string]*ServiceState{
		"id-api": {
			ID:           "id-api",
			Name:         "api",
			Status:       StatusStopped,
			LifecycleSeq: 1,
			Timeline:     NewTimeline(components.DefaultTimelineSlots),
		},
	}
	m.state.restarting = map[string]bool{}

	m.refreshFromStore()
	m.sampleTimelines()

	svc := m.state.services["id-api"]
	assert.Equal(t, StatusFailed, svc.Status)
	assert.Equal(t, 4, svc.Timeline.Count())

	slots := svc.Timeline.Slots()
	assert.Equal(t, SlotStarting, slots[0])
	assert.Equal(t, SlotStarting, slots[1])
	assert.Equal(t, SlotStarting, slots[2])
	assert.Equal(t, SlotFailed, slots[3])
}

func Test_ReconcileLifecycle_MissedStartup_RestartFailedBackfills(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	oldStart := t0.Add(-60 * time.Second)

	mockStore := registry.NewMockStore(ctrl)
	mockStore.EXPECT().Services().Return([]registry.ServiceSnapshot{
		{
			ID:               "id-api",
			Status:           registry.StatusFailed,
			Error:            "crash",
			AttemptStartedAt: t0,
			LifecycleAt:      t0.Add(2 * time.Second),
			LifecycleSeq:     10,
		},
	})

	m := &Model{
		store:  mockStore,
		loader: &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)},
	}
	m.state.services = map[string]*ServiceState{
		"id-api": {
			ID:               "id-api",
			Name:             "api",
			Status:           StatusRunning,
			PID:              1111,
			StartTime:        oldStart,
			AttemptStartedAt: oldStart,
			LifecycleSeq:     5,
			Timeline:         NewTimeline(components.DefaultTimelineSlots),
		},
	}
	m.state.restarting = map[string]bool{}

	m.refreshFromStore()

	svc := m.state.services["id-api"]
	assert.Equal(t, StatusFailed, svc.Status)
	assert.Equal(t, 2, svc.Timeline.Count())

	slots := svc.Timeline.Slots()
	assert.Equal(t, SlotStarting, slots[0])
	assert.Equal(t, SlotStarting, slots[1])
}

func Test_HandleRestartFailedKey(t *testing.T) {
	tests := []struct {
		name         string
		serviceIDs   []string
		services     map[string]*ServiceState
		filterQuery  string
		filteredIDs  []string
		expectCalls  []bus.Service
		expectLoader []string
		expectCmd    bool
	}{
		{
			name:       "multiple failed services",
			serviceIDs: []string{"id-api", "id-web", "id-db"},
			services: map[string]*ServiceState{
				"id-api": {ID: "id-api", Name: "api", Status: StatusFailed},
				"id-web": {ID: "id-web", Name: "web", Status: StatusRunning},
				"id-db":  {ID: "id-db", Name: "db", Status: StatusFailed},
			},
			expectCalls: []bus.Service{
				{ID: "id-api", Name: "api"},
				{ID: "id-db", Name: "db"},
			},
			expectLoader: []string{"id-api", "id-db"},
			expectCmd:    true,
		},
		{
			name:       "no failed services",
			serviceIDs: []string{"id-api", "id-web"},
			services: map[string]*ServiceState{
				"id-api": {ID: "id-api", Name: "api", Status: StatusRunning},
				"id-web": {ID: "id-web", Name: "web", Status: StatusStopped},
			},
			expectCalls:  nil,
			expectLoader: nil,
			expectCmd:    false,
		},
		{
			name:       "mixed states - only failed restarted",
			serviceIDs: []string{"id-api", "id-web", "id-db", "id-cache"},
			services: map[string]*ServiceState{
				"id-api":   {ID: "id-api", Name: "api", Status: StatusRunning},
				"id-web":   {ID: "id-web", Name: "web", Status: StatusStopped},
				"id-db":    {ID: "id-db", Name: "db", Status: StatusFailed},
				"id-cache": {ID: "id-cache", Name: "cache", Status: StatusStarting},
			},
			expectCalls: []bus.Service{
				{ID: "id-db", Name: "db"},
			},
			expectLoader: []string{"id-db"},
			expectCmd:    true,
		},
		{
			name:         "empty services",
			serviceIDs:   []string{},
			services:     map[string]*ServiceState{},
			expectCalls:  nil,
			expectLoader: nil,
			expectCmd:    false,
		},
		{
			name:       "filter applied - restarts all failed services regardless of filter",
			serviceIDs: []string{"id-api", "id-web", "id-db"},
			services: map[string]*ServiceState{
				"id-api": {ID: "id-api", Name: "api", Status: StatusFailed},
				"id-web": {ID: "id-web", Name: "web", Status: StatusFailed},
				"id-db":  {ID: "id-db", Name: "db", Status: StatusFailed},
			},
			filterQuery: "api",
			filteredIDs: []string{"id-api"},
			expectCalls: []bus.Service{
				{ID: "id-api", Name: "api"},
				{ID: "id-web", Name: "web"},
				{ID: "id-db", Name: "db"},
			},
			expectLoader: []string{"id-api", "id-web", "id-db"},
			expectCmd:    true,
		},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockController := NewMockController(ctrl)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, svc := range tt.expectCalls {
				mockController.EXPECT().Restart(svc)
			}

			loader := &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}

			m := Model{loader: loader, controller: mockController}
			m.state.serviceIDs = tt.serviceIDs
			m.state.services = tt.services
			m.state.filterQuery = tt.filterQuery
			m.state.filteredIDs = tt.filteredIDs

			teaModel, cmd := m.handleRestartFailedKey()
			result := teaModel.(Model)

			for _, id := range tt.expectLoader {
				assert.True(t, result.loader.Has(id))
			}

			if tt.expectCmd {
				assert.NotNil(t, cmd)
			} else {
				assert.Nil(t, cmd)
			}
		})
	}
}

func Test_HandleKeyPress_CtrlRRoutesToRestartFailed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockController := NewMockController(ctrl)
	mockController.EXPECT().Restart(bus.Service{ID: "id-api", Name: "api"})

	loader := &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}

	m := Model{loader: loader, controller: mockController}
	m.state.shuttingDown = false
	m.ui.servicesKeys = DefaultKeyMap()
	m.state.serviceIDs = []string{"id-api"}
	m.state.services = map[string]*ServiceState{
		"id-api": {ID: "id-api", Name: "api", Status: StatusFailed},
	}

	msg := tea.KeyPressMsg{Code: 'r', Mod: tea.ModCtrl}
	teaModel, cmd := m.handleKeyPress(msg)
	result := teaModel.(Model)

	assert.True(t, result.loader.Has("id-api"))
	assert.NotNil(t, cmd)
}

func toKeyMsg(s string) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: rune(s[0]), Text: s}
}
