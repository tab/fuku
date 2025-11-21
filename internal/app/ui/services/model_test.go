package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_ServiceState_MarkStarting(t *testing.T) {
	service := &ServiceState{Status: StatusStopped}
	service.MarkStarting()
	assert.Equal(t, StatusStarting, service.Status)
}

func Test_ServiceState_MarkRunning(t *testing.T) {
	service := &ServiceState{Status: StatusStarting}
	service.MarkRunning()
	assert.Equal(t, StatusReady, service.Status)
}

func Test_ServiceState_MarkStopping(t *testing.T) {
	service := &ServiceState{Status: StatusReady}
	service.MarkStopping()
	assert.Equal(t, StatusStopping, service.Status)
}

func Test_ServiceState_MarkStopped(t *testing.T) {
	service := &ServiceState{Status: StatusReady, Monitor: ServiceMonitor{PID: 1234}}
	service.MarkStopped()
	assert.Equal(t, StatusStopped, service.Status)
	assert.Equal(t, 0, service.Monitor.PID)
}

func Test_ServiceState_MarkFailed(t *testing.T) {
	service := &ServiceState{Status: StatusStarting}
	service.MarkFailed()
	assert.Equal(t, StatusFailed, service.Status)
}

func Test_GetTotalServices(t *testing.T) {
	tests := []struct {
		name  string
		tiers []Tier
		want  int
	}{
		{name: "empty tiers", tiers: []Tier{}, want: 0},
		{name: "single tier single service", tiers: []Tier{{Services: []string{"api"}}}, want: 1},
		{name: "single tier multiple services", tiers: []Tier{{Services: []string{"api", "db", "cache"}}}, want: 3},
		{name: "multiple tiers", tiers: []Tier{{Services: []string{"db"}}, {Services: []string{"api", "web"}}}, want: 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{}
			m.state.tiers = tt.tiers
			assert.Equal(t, tt.want, m.getTotalServices())
		})
	}
}

func Test_GetReadyServices(t *testing.T) {
	tests := []struct {
		name     string
		services map[string]*ServiceState
		want     int
	}{
		{name: "empty services", services: map[string]*ServiceState{}, want: 0},
		{name: "no ready services", services: map[string]*ServiceState{"api": {Status: StatusStarting}, "db": {Status: StatusFailed}}, want: 0},
		{name: "some ready", services: map[string]*ServiceState{"api": {Status: StatusReady}, "db": {Status: StatusStarting}}, want: 1},
		{name: "all ready", services: map[string]*ServiceState{"api": {Status: StatusReady}, "db": {Status: StatusReady}}, want: 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{}
			m.state.services = tt.services
			assert.Equal(t, tt.want, m.getReadyServices())
		})
	}
}

func Test_GetSelectedService(t *testing.T) {
	services := map[string]*ServiceState{
		"db":  {Name: "db"},
		"api": {Name: "api"},
		"web": {Name: "web"},
	}
	tiers := []Tier{
		{Name: "tier1", Services: []string{"db"}},
		{Name: "tier2", Services: []string{"api", "web"}},
	}

	tests := []struct {
		name     string
		selected int
		want     string
	}{
		{name: "first service", selected: 0, want: "db"},
		{name: "second service", selected: 1, want: "api"},
		{name: "third service", selected: 2, want: "web"},
		{name: "negative index", selected: -1, want: ""},
		{name: "out of bounds", selected: 10, want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{}
			m.state.tiers = tiers
			m.state.services = services
			m.state.selected = tt.selected

			result := m.getSelectedService()
			if tt.want == "" {
				assert.Nil(t, result)
			} else {
				assert.NotNil(t, result)
				assert.Equal(t, tt.want, result.Name)
			}
		})
	}
}

func Test_GetMaxServiceNameLength(t *testing.T) {
	tests := []struct {
		name     string
		services map[string]*ServiceState
		want     int
	}{
		{name: "empty services returns default", services: map[string]*ServiceState{}, want: 20},
		{name: "short names return default", services: map[string]*ServiceState{"api": {Name: "api"}, "db": {Name: "db"}}, want: 20},
		{name: "long name exceeds default", services: map[string]*ServiceState{"action-confirmation-management-service": {Name: "action-confirmation-management-service"}}, want: 38},
		{name: "mixed lengths uses longest", services: map[string]*ServiceState{"api": {Name: "api"}, "user-management-service": {Name: "user-management-service"}}, want: 23},
		{name: "emoji display width", services: map[string]*ServiceState{"api-🔥": {Name: "api-🔥"}}, want: 20},
		{name: "CJK double-width characters", services: map[string]*ServiceState{"测试服务": {Name: "测试服务"}}, want: 20},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{}
			m.state.services = tt.services
			assert.Equal(t, tt.want, m.getMaxServiceNameLength())
		})
	}
}

func Test_CalculateScrollOffset(t *testing.T) {
	t.Run("zero height viewport returns current offset", func(t *testing.T) {
		m := Model{}
		m.state.tiers = []Tier{{Services: []string{"api"}}}
		m.state.services = map[string]*ServiceState{"api": {Name: "api"}}
		m.state.selected = 0
		m.ui.servicesViewport.Height = 0

		offset := m.calculateScrollOffset()

		assert.Equal(t, 0, offset)
	})

	t.Run("selection visible returns current offset", func(t *testing.T) {
		m := Model{}
		m.state.tiers = []Tier{{Services: []string{"api", "db"}}}
		m.state.services = map[string]*ServiceState{"api": {Name: "api"}, "db": {Name: "db"}}
		m.state.selected = 0
		m.ui.servicesViewport.Height = 10

		offset := m.calculateScrollOffset()

		assert.Equal(t, 0, offset)
	})

	t.Run("calculates offset for multi-tier selection", func(t *testing.T) {
		m := Model{}
		m.state.tiers = []Tier{
			{Services: []string{"a", "b"}},
			{Services: []string{"c", "d"}},
		}
		m.state.services = map[string]*ServiceState{
			"a": {Name: "a"}, "b": {Name: "b"},
			"c": {Name: "c"}, "d": {Name: "d"},
		}
		m.state.selected = 3
		m.ui.servicesViewport.Height = 20

		offset := m.calculateScrollOffset()

		assert.Equal(t, 0, offset)
	})

	t.Run("scrolls down when selection below viewport", func(t *testing.T) {
		m := Model{}
		m.state.tiers = []Tier{{Services: []string{"a", "b", "c", "d", "e", "f"}}}
		m.state.services = map[string]*ServiceState{"a": {}, "b": {}, "c": {}, "d": {}, "e": {}, "f": {}}
		m.state.selected = 5
		m.ui.servicesViewport.Height = 3
		m.ui.servicesViewport.YOffset = 0

		offset := m.calculateScrollOffset()

		assert.True(t, offset > 0)
	})

	t.Run("scrolls up when selection above viewport", func(t *testing.T) {
		m := Model{}
		m.state.tiers = []Tier{{Services: []string{"a", "b", "c", "d", "e"}}}
		m.state.services = map[string]*ServiceState{"a": {}, "b": {}, "c": {}, "d": {}, "e": {}}
		m.state.selected = 0
		m.ui.servicesViewport.Height = 3
		m.ui.servicesViewport.YOffset = 10

		offset := m.calculateScrollOffset()

		assert.True(t, offset < 10)
	})
}
