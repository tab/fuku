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
		tiers []TierView
		want  int
	}{
		{name: "empty tiers", tiers: []TierView{}, want: 0},
		{name: "single tier single service", tiers: []TierView{{Services: []string{"api"}}}, want: 1},
		{name: "single tier multiple services", tiers: []TierView{{Services: []string{"api", "db", "cache"}}}, want: 3},
		{name: "multiple tiers", tiers: []TierView{{Services: []string{"db"}}, {Services: []string{"api", "web"}}}, want: 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{tiers: tt.tiers}
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
			m := Model{services: tt.services}
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
	tiers := []TierView{
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
			m := Model{tiers: tiers, services: services, selected: tt.selected}

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
