package rest

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fuku/internal/app/services"
	"fuku/internal/contracts"
	"fuku/internal/model"
)

func Test_toActionSerializer(t *testing.T) {
	api := model.Service{ID: "id-api", Name: "api"}

	tests := []struct {
		name      string
		admission services.Admission
		expect    string
	}{
		{
			name:      "start leads to starting",
			admission: services.Admission{Service: api, Action: contracts.ActionStart, Status: model.StatusStarting},
			expect:    `{"id":"id-api","name":"api","action":"start","status":"starting"}`,
		},
		{
			name:      "stop leads to stopping",
			admission: services.Admission{Service: api, Action: contracts.ActionStop, Status: model.StatusStopping},
			expect:    `{"id":"id-api","name":"api","action":"stop","status":"stopping"}`,
		},
		{
			name:      "restart leads to restarting",
			admission: services.Admission{Service: api, Action: contracts.ActionRestart, Status: model.StatusRestarting},
			expect:    `{"id":"id-api","name":"api","action":"restart","status":"restarting"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, err := json.Marshal(toActionSerializer(tt.admission))

			require.NoError(t, err)
			assert.JSONEq(t, tt.expect, string(body))
		})
	}
}

func Test_toServiceListSerializer(t *testing.T) {
	db := &model.Service{ID: "id-db", Name: "db", Tier: "foundation", Status: model.StatusStopped}
	worker := &model.Service{ID: "id-worker", Name: "worker", Tier: "application", Status: model.StatusStopped}
	api := &model.Service{ID: "id-api", Name: "api", Tier: "application", Status: model.StatusStopped}

	tests := []struct {
		name   string
		tiers  []*model.Tier
		expect []ServiceSerializer
	}{
		{
			name: "tier order, then the order within each tier",
			tiers: []*model.Tier{
				{Name: "foundation", Services: []*model.Service{db}},
				{Name: "application", Services: []*model.Service{worker, api}},
			},
			expect: []ServiceSerializer{
				{ID: "id-db", Name: "db", Tier: "foundation", Status: model.StatusStopped},
				{ID: "id-worker", Name: "worker", Tier: "application", Status: model.StatusStopped},
				{ID: "id-api", Name: "api", Tier: "application", Status: model.StatusStopped},
			},
		},
		{
			name:   "no tiers is an empty list, not null",
			tiers:  nil,
			expect: []ServiceSerializer{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := toServiceListSerializer(tt.tiers)

			assert.Equal(t, tt.expect, result.Services)
		})
	}
}

func Test_toServiceSerializer(t *testing.T) {
	started := time.Now().Add(-time.Minute)

	tests := []struct {
		name    string
		service *model.Service
		expect  string
	}{
		{
			name:    "running service carries its process facts",
			service: &model.Service{ID: "id-api", Name: "api", Tier: "app", Status: model.StatusRunning, Watching: true, Process: model.Process{PID: 42, CPU: 1.5, Memory: 1024, StartedAt: started}},
			expect:  `{"id":"id-api","name":"api","tier":"app","status":"running","watching":true,"pid":42,"cpu":1.5,"memory":1024,"uptime":60}`,
		},
		{
			name:    "stopped service omits the process facts and the empty error",
			service: &model.Service{ID: "id-api", Name: "api", Tier: "app", Status: model.StatusStopped, Process: model.Process{PID: 42, CPU: 1.5, Memory: 1024, StartedAt: started}},
			expect:  `{"id":"id-api","name":"api","tier":"app","status":"stopped","watching":false,"pid":0,"cpu":0,"memory":0,"uptime":0}`,
		},
		{
			name:    "failed service carries its error",
			service: &model.Service{ID: "id-api", Name: "api", Tier: "app", Status: model.StatusFailed, Error: "readiness check timed out"},
			expect:  `{"id":"id-api","name":"api","tier":"app","status":"failed","watching":false,"error":"readiness check timed out","pid":0,"cpu":0,"memory":0,"uptime":0}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, err := json.Marshal(toServiceSerializer(tt.service))

			require.NoError(t, err)
			assert.JSONEq(t, tt.expect, string(body))
		})
	}
}
