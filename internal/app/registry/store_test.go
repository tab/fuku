package registry

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fuku/internal/app/bus"
	"fuku/internal/app/monitor"
	"fuku/internal/config"
)

const (
	testTimeout  = time.Second
	testInterval = 10 * time.Millisecond
)

func newTestStore(t *testing.T, cfg *config.Config) (*store, bus.Bus) {
	t.Helper()

	b := bus.NewBus(cfg, nil, nil)
	mon := monitor.NewMonitor()
	s := NewStore(cfg, b, mon).(*store)

	go s.Run(t.Context())

	<-s.ready

	return s, b
}

func Test_Store_ProfileResolved(t *testing.T) {
	s, b := newTestStore(t, config.DefaultConfig())

	b.Publish(bus.Message{
		Type: bus.EventProfileResolved,
		Data: bus.ProfileResolved{
			Profile: "default",
			Tiers: []bus.Tier{
				{Name: "foundation", Services: []string{"db", "cache"}},
				{Name: "application", Services: []string{"api", "web"}},
			},
		},
	})

	require.Eventually(t, func() bool {
		return s.Profile() == "default"
	}, testTimeout, testInterval)

	services := s.Services()
	require.Len(t, services, 4)
	assert.Equal(t, "cache", services[0].Name)
	assert.Equal(t, "db", services[1].Name)
	assert.Equal(t, "api", services[2].Name)
	assert.Equal(t, "web", services[3].Name)

	for _, svc := range services {
		assert.Equal(t, StatusStarting, svc.Status)
		assert.NotEmpty(t, svc.ID)
	}
}

func Test_Store_PhaseTransitions(t *testing.T) {
	s, b := newTestStore(t, config.DefaultConfig())

	b.Publish(bus.Message{
		Type: bus.EventPhaseChanged,
		Data: bus.PhaseChanged{Phase: bus.PhaseStartup},
	})

	require.Eventually(t, func() bool {
		return s.Phase() == "startup"
	}, testTimeout, testInterval)

	b.Publish(bus.Message{
		Type: bus.EventPhaseChanged,
		Data: bus.PhaseChanged{Phase: bus.PhaseRunning},
	})

	require.Eventually(t, func() bool {
		return s.Phase() == string(bus.PhaseRunning)
	}, testTimeout, testInterval)

	assert.Positive(t, s.Uptime())

	b.Publish(bus.Message{
		Type: bus.EventPhaseChanged,
		Data: bus.PhaseChanged{Phase: bus.PhaseStopping},
	})

	require.Eventually(t, func() bool {
		return s.Phase() == string(bus.PhaseStopping)
	}, testTimeout, testInterval)
}

func Test_Store_ServiceLifecycle(t *testing.T) {
	s, b := newTestStore(t, config.DefaultConfig())

	b.Publish(bus.Message{
		Type: bus.EventProfileResolved,
		Data: bus.ProfileResolved{
			Profile: "default",
			Tiers:   []bus.Tier{{Name: "foundation", Services: []string{"api"}}},
		},
	})

	require.Eventually(t, func() bool {
		_, found := s.Service("api")
		return found
	}, testTimeout, testInterval)

	b.Publish(bus.Message{
		Type: bus.EventServiceStarting,
		Data: bus.ServiceStarting{
			ServiceEvent: bus.ServiceEvent{Service: "api", Tier: "foundation"},
			PID:          1234,
		},
	})

	require.Eventually(t, func() bool {
		svc, _ := s.Service("api")
		return svc.PID == 1234
	}, testTimeout, testInterval)

	svc, found := s.Service("api")
	require.True(t, found)
	assert.Equal(t, StatusStarting, svc.Status)

	b.Publish(bus.Message{
		Type: bus.EventServiceReady,
		Data: bus.ServiceReady{ServiceEvent: bus.ServiceEvent{Service: "api", Tier: "foundation"}},
	})

	require.Eventually(t, func() bool {
		svc, _ := s.Service("api")
		return svc.Status == StatusRunning
	}, testTimeout, testInterval)

	b.Publish(bus.Message{
		Type: bus.EventServiceStopping,
		Data: bus.ServiceStopping{ServiceEvent: bus.ServiceEvent{Service: "api", Tier: "foundation"}},
	})

	require.Eventually(t, func() bool {
		svc, _ := s.Service("api")
		return svc.Status == StatusStopping
	}, testTimeout, testInterval)

	b.Publish(bus.Message{
		Type: bus.EventServiceStopped,
		Data: bus.ServiceStopped{ServiceEvent: bus.ServiceEvent{Service: "api", Tier: "foundation"}},
	})

	require.Eventually(t, func() bool {
		svc, _ := s.Service("api")
		return svc.Status == StatusStopped
	}, testTimeout, testInterval)

	svc, found = s.Service("api")
	require.True(t, found)
	assert.Equal(t, 0, svc.PID)
}

func Test_Store_ServiceFailed(t *testing.T) {
	s, b := newTestStore(t, config.DefaultConfig())

	b.Publish(bus.Message{
		Type: bus.EventProfileResolved,
		Data: bus.ProfileResolved{
			Profile: "default",
			Tiers:   []bus.Tier{{Name: "foundation", Services: []string{"api"}}},
		},
	})

	require.Eventually(t, func() bool {
		_, found := s.Service("api")
		return found
	}, testTimeout, testInterval)

	b.Publish(bus.Message{
		Type: bus.EventServiceStarting,
		Data: bus.ServiceStarting{
			ServiceEvent: bus.ServiceEvent{Service: "api", Tier: "foundation"},
			PID:          5678,
		},
	})

	b.Publish(bus.Message{
		Type: bus.EventServiceFailed,
		Data: bus.ServiceFailed{ServiceEvent: bus.ServiceEvent{Service: "api", Tier: "foundation"}},
	})

	require.Eventually(t, func() bool {
		svc, _ := s.Service("api")
		return svc.Status == StatusFailed
	}, testTimeout, testInterval)

	svc, found := s.Service("api")
	require.True(t, found)
	assert.Equal(t, 0, svc.PID)
	assert.True(t, svc.StartTime.IsZero())
}

func Test_Store_ServiceNotFound(t *testing.T) {
	s, _ := newTestStore(t, config.DefaultConfig())

	_, found := s.Service("nonexistent")
	assert.False(t, found)
}

func Test_Store_ServiceOrder(t *testing.T) {
	s, b := newTestStore(t, config.DefaultConfig())

	b.Publish(bus.Message{
		Type: bus.EventProfileResolved,
		Data: bus.ProfileResolved{
			Profile: "default",
			Tiers: []bus.Tier{
				{Name: "platform", Services: []string{"zebra", "alpha"}},
				{Name: "foundation", Services: []string{"beta"}},
			},
		},
	})

	require.Eventually(t, func() bool {
		return len(s.Services()) == 3
	}, testTimeout, testInterval)

	services := s.Services()
	assert.Equal(t, "alpha", services[0].Name)
	assert.Equal(t, "zebra", services[1].Name)
	assert.Equal(t, "beta", services[2].Name)
}

func Test_Store_Uptime_ZeroBeforeRunning(t *testing.T) {
	cfg := config.DefaultConfig()
	b := bus.NewBus(cfg, nil, nil)
	mon := monitor.NewMonitor()
	s := NewStore(cfg, b, mon)

	assert.Equal(t, time.Duration(0), s.Uptime())
}

func Test_Status_IsRunning(t *testing.T) {
	tests := []struct {
		name   string
		status Status
		want   bool
	}{
		{
			name:   "running",
			status: StatusRunning,
			want:   true,
		},
		{
			name:   "stopped",
			status: StatusStopped,
			want:   false,
		},
		{
			name:   "starting",
			status: StatusStarting,
			want:   false,
		},
		{
			name:   "failed",
			status: StatusFailed,
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.status.IsRunning())
		})
	}
}

func Test_Status_IsStartable(t *testing.T) {
	tests := []struct {
		name   string
		status Status
		want   bool
	}{
		{
			name:   "stopped",
			status: StatusStopped,
			want:   true,
		},
		{
			name:   "failed",
			status: StatusFailed,
			want:   true,
		},
		{
			name:   "running",
			status: StatusRunning,
			want:   false,
		},
		{
			name:   "starting",
			status: StatusStarting,
			want:   false,
		},
		{
			name:   "stopping",
			status: StatusStopping,
			want:   false,
		},
		{
			name:   "restarting",
			status: StatusRestarting,
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.status.IsStartable())
		})
	}
}

func Test_Status_IsStoppable(t *testing.T) {
	tests := []struct {
		name   string
		status Status
		want   bool
	}{
		{
			name:   "running",
			status: StatusRunning,
			want:   true,
		},
		{
			name:   "stopped",
			status: StatusStopped,
			want:   false,
		},
		{
			name:   "starting",
			status: StatusStarting,
			want:   false,
		},
		{
			name:   "failed",
			status: StatusFailed,
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.status.IsStoppable())
		})
	}
}

func Test_Status_IsRestartable(t *testing.T) {
	tests := []struct {
		name   string
		status Status
		want   bool
	}{
		{
			name:   "running",
			status: StatusRunning,
			want:   true,
		},
		{
			name:   "failed",
			status: StatusFailed,
			want:   true,
		},
		{
			name:   "stopped",
			status: StatusStopped,
			want:   true,
		},
		{
			name:   "starting",
			status: StatusStarting,
			want:   false,
		},
		{
			name:   "stopping",
			status: StatusStopping,
			want:   false,
		},
		{
			name:   "restarting",
			status: StatusRestarting,
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.status.IsRestartable())
		})
	}
}

func Test_Store_ServiceRestarting(t *testing.T) {
	s, b := newTestStore(t, config.DefaultConfig())

	b.Publish(bus.Message{
		Type: bus.EventProfileResolved,
		Data: bus.ProfileResolved{
			Profile: "default",
			Tiers:   []bus.Tier{{Name: "foundation", Services: []string{"api"}}},
		},
	})

	require.Eventually(t, func() bool {
		_, found := s.Service("api")
		return found
	}, testTimeout, testInterval)

	b.Publish(bus.Message{
		Type: bus.EventServiceRestarting,
		Data: bus.ServiceRestarting{ServiceEvent: bus.ServiceEvent{Service: "api", Tier: "foundation"}},
	})

	require.Eventually(t, func() bool {
		svc, _ := s.Service("api")
		return svc.Status == StatusRestarting
	}, testTimeout, testInterval)
}
