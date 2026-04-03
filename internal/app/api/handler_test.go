package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"fuku/internal/app/bus"
	"fuku/internal/app/registry"
)

func newTestHandler(t *testing.T) (*handler, *registry.MockStore, *bus.MockBus) {
	ctrl := gomock.NewController(t)
	mockStore := registry.NewMockStore(ctrl)
	mockBus := bus.NewMockBus(ctrl)

	return &handler{store: mockStore, bus: mockBus}, mockStore, mockBus
}

func Test_HandleStatus(t *testing.T) {
	h, mockStore, _ := newTestHandler(t)

	mockStore.EXPECT().Services().Return([]registry.ServiceSnapshot{
		{ID: "id-1", Name: "api", Status: registry.StatusRunning},
		{ID: "id-2", Name: "db", Status: registry.StatusRunning},
		{ID: "id-3", Name: "cache", Status: registry.StatusStopped},
		{ID: "id-4", Name: "worker", Status: registry.StatusFailed},
	})
	mockStore.EXPECT().Profile().Return("default")
	mockStore.EXPECT().Phase().Return(string(bus.PhaseRunning))
	mockStore.EXPECT().Uptime().Return(3600 * time.Second)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/status", nil)
	w := httptest.NewRecorder()

	h.handleStatus(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var body StatusSerializer
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "default", body.Profile)
	assert.Equal(t, string(bus.PhaseRunning), body.Phase)
	assert.Equal(t, int64(3600), body.Uptime)
	assert.Equal(t, 4, body.Services.Total)
	assert.Equal(t, 2, body.Services.Running)
	assert.Equal(t, 1, body.Services.Stopped)
	assert.Equal(t, 1, body.Services.Failed)
}

func Test_HandleListServices(t *testing.T) {
	h, mockStore, _ := newTestHandler(t)

	now := time.Now()
	mockStore.EXPECT().Services().Return([]registry.ServiceSnapshot{
		{ID: "id-1", Name: "db", Tier: "foundation", Status: registry.StatusRunning, PID: 100, CPU: 1.5, Memory: 1024, StartTime: now},
		{ID: "id-2", Name: "api", Tier: "application", Status: registry.StatusStopped},
		{ID: "id-3", Name: "worker", Tier: "application", Status: registry.StatusStarting, PID: 200, CPU: 0.5, Memory: 512, StartTime: now},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/services", nil)
	w := httptest.NewRecorder()

	h.handleListServices(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var body ServiceListSerializer
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Len(t, body.Services, 3)
	assert.Equal(t, "db", body.Services[0].Name)
	assert.Equal(t, registry.StatusRunning, body.Services[0].Status)
	assert.Equal(t, 100, body.Services[0].PID)
	assert.InDelta(t, 1.5, body.Services[0].CPU, 0.01)
	assert.Equal(t, uint64(1024), body.Services[0].Memory)

	assert.Equal(t, "api", body.Services[1].Name)
	assert.Equal(t, registry.StatusStopped, body.Services[1].Status)
	assert.Equal(t, 0, body.Services[1].PID)
	assert.Equal(t, int64(0), body.Services[1].Uptime)

	assert.Equal(t, "worker", body.Services[2].Name)
	assert.Equal(t, registry.StatusStarting, body.Services[2].Status)
	assert.Equal(t, 0, body.Services[2].PID)
	assert.InDelta(t, 0, body.Services[2].CPU, 0.01)
	assert.Equal(t, uint64(0), body.Services[2].Memory)
	assert.Equal(t, int64(0), body.Services[2].Uptime)
}

func Test_HandleGetService(t *testing.T) {
	tests := []struct {
		name         string
		serviceID    string
		before       func(*registry.MockStore)
		expectStatus int
		expectBody   string
	}{
		{
			name:      "service found",
			serviceID: "id-api",
			before: func(s *registry.MockStore) {
				s.EXPECT().ServiceByID("id-api").Return(registry.ServiceSnapshot{
					ID:     "id-api",
					Name:   "api",
					Tier:   "foundation",
					Status: registry.StatusRunning,
					PID:    1234,
				}, true)
			},
			expectStatus: http.StatusOK,
			expectBody:   "api",
		},
		{
			name:      "service not found",
			serviceID: "id-unknown",
			before: func(s *registry.MockStore) {
				s.EXPECT().ServiceByID("id-unknown").Return(registry.ServiceSnapshot{}, false)
			},
			expectStatus: http.StatusNotFound,
			expectBody:   "service not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, mockStore, _ := newTestHandler(t)
			tt.before(mockStore)

			mux := http.NewServeMux()
			mux.HandleFunc("GET /api/v1/services/{id}", h.handleGetService)

			req := httptest.NewRequest(http.MethodGet, "/api/v1/services/"+tt.serviceID, nil)
			w := httptest.NewRecorder()

			mux.ServeHTTP(w, req)

			assert.Equal(t, tt.expectStatus, w.Code)
			assert.Contains(t, w.Body.String(), tt.expectBody)
		})
	}
}

func Test_HandleStartService(t *testing.T) {
	tests := []struct {
		name         string
		serviceID    string
		before       func(*registry.MockStore, *bus.MockBus)
		expectStatus int
		expectBody   string
	}{
		{
			name:      "start stopped service",
			serviceID: "id-api",
			before: func(s *registry.MockStore, b *bus.MockBus) {
				s.EXPECT().Phase().Return(string(bus.PhaseRunning))
				s.EXPECT().ServiceByID("id-api").Return(registry.ServiceSnapshot{ID: "id-api", Name: "api", Status: registry.StatusStopped}, true)
				b.EXPECT().Publish(gomock.Any()).Do(func(msg bus.Message) {
					assert.Equal(t, bus.CommandStartService, msg.Type)
				})
			},
			expectStatus: http.StatusAccepted,
			expectBody:   "starting",
		},
		{
			name:      "start failed service",
			serviceID: "id-api",
			before: func(s *registry.MockStore, b *bus.MockBus) {
				s.EXPECT().Phase().Return(string(bus.PhaseRunning))
				s.EXPECT().ServiceByID("id-api").Return(registry.ServiceSnapshot{ID: "id-api", Name: "api", Status: registry.StatusFailed}, true)
				b.EXPECT().Publish(gomock.Any())
			},
			expectStatus: http.StatusAccepted,
			expectBody:   "starting",
		},
		{
			name:      "cannot start running service",
			serviceID: "id-api",
			before: func(s *registry.MockStore, _ *bus.MockBus) {
				s.EXPECT().Phase().Return(string(bus.PhaseRunning))
				s.EXPECT().ServiceByID("id-api").Return(registry.ServiceSnapshot{ID: "id-api", Name: "api", Status: registry.StatusRunning}, true)
			},
			expectStatus: http.StatusConflict,
			expectBody:   "service is running",
		},
		{
			name:      "service not found",
			serviceID: "id-unknown",
			before: func(s *registry.MockStore, _ *bus.MockBus) {
				s.EXPECT().Phase().Return(string(bus.PhaseRunning))
				s.EXPECT().ServiceByID("id-unknown").Return(registry.ServiceSnapshot{}, false)
			},
			expectStatus: http.StatusNotFound,
			expectBody:   "service not found",
		},
		{
			name:      "instance not accepting actions",
			serviceID: "id-api",
			before: func(s *registry.MockStore, _ *bus.MockBus) {
				s.EXPECT().Phase().Return("startup")
			},
			expectStatus: http.StatusConflict,
			expectBody:   "instance is not accepting actions",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, mockStore, mockBus := newTestHandler(t)
			tt.before(mockStore, mockBus)

			mux := http.NewServeMux()
			mux.HandleFunc("POST /api/v1/services/{id}/start", h.handleStartService)

			req := httptest.NewRequest(http.MethodPost, "/api/v1/services/"+tt.serviceID+"/start", nil)
			w := httptest.NewRecorder()

			mux.ServeHTTP(w, req)

			assert.Equal(t, tt.expectStatus, w.Code)
			assert.Contains(t, w.Body.String(), tt.expectBody)
		})
	}
}

func Test_HandleStopService(t *testing.T) {
	tests := []struct {
		name         string
		serviceID    string
		before       func(*registry.MockStore, *bus.MockBus)
		expectStatus int
		expectBody   string
	}{
		{
			name:      "stop running service",
			serviceID: "id-api",
			before: func(s *registry.MockStore, b *bus.MockBus) {
				s.EXPECT().Phase().Return(string(bus.PhaseRunning))
				s.EXPECT().ServiceByID("id-api").Return(registry.ServiceSnapshot{ID: "id-api", Name: "api", Status: registry.StatusRunning}, true)
				b.EXPECT().Publish(gomock.Any()).Do(func(msg bus.Message) {
					assert.Equal(t, bus.CommandStopService, msg.Type)
				})
			},
			expectStatus: http.StatusAccepted,
			expectBody:   "stopping",
		},
		{
			name:      "cannot stop stopped service",
			serviceID: "id-api",
			before: func(s *registry.MockStore, _ *bus.MockBus) {
				s.EXPECT().Phase().Return(string(bus.PhaseRunning))
				s.EXPECT().ServiceByID("id-api").Return(registry.ServiceSnapshot{ID: "id-api", Name: "api", Status: registry.StatusStopped}, true)
			},
			expectStatus: http.StatusConflict,
			expectBody:   "service is not running",
		},
		{
			name:      "instance not accepting actions",
			serviceID: "id-api",
			before: func(s *registry.MockStore, _ *bus.MockBus) {
				s.EXPECT().Phase().Return(string(bus.PhaseStopping))
			},
			expectStatus: http.StatusConflict,
			expectBody:   "instance is not accepting actions",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, mockStore, mockBus := newTestHandler(t)
			tt.before(mockStore, mockBus)

			mux := http.NewServeMux()
			mux.HandleFunc("POST /api/v1/services/{id}/stop", h.handleStopService)

			req := httptest.NewRequest(http.MethodPost, "/api/v1/services/"+tt.serviceID+"/stop", nil)
			w := httptest.NewRecorder()

			mux.ServeHTTP(w, req)

			assert.Equal(t, tt.expectStatus, w.Code)
			assert.Contains(t, w.Body.String(), tt.expectBody)
		})
	}
}

func Test_HandleRestartService(t *testing.T) {
	tests := []struct {
		name         string
		serviceID    string
		before       func(*registry.MockStore, *bus.MockBus)
		expectStatus int
		expectBody   string
	}{
		{
			name:      "restart running service",
			serviceID: "id-api",
			before: func(s *registry.MockStore, b *bus.MockBus) {
				s.EXPECT().Phase().Return(string(bus.PhaseRunning))
				s.EXPECT().ServiceByID("id-api").Return(registry.ServiceSnapshot{ID: "id-api", Name: "api", Status: registry.StatusRunning}, true)
				b.EXPECT().Publish(gomock.Any()).Do(func(msg bus.Message) {
					assert.Equal(t, bus.CommandRestartService, msg.Type)
				})
			},
			expectStatus: http.StatusAccepted,
			expectBody:   "restarting",
		},
		{
			name:      "cannot restart stopped service",
			serviceID: "id-api",
			before: func(s *registry.MockStore, _ *bus.MockBus) {
				s.EXPECT().Phase().Return(string(bus.PhaseRunning))
				s.EXPECT().ServiceByID("id-api").Return(registry.ServiceSnapshot{ID: "id-api", Name: "api", Status: registry.StatusStopped}, true)
			},
			expectStatus: http.StatusConflict,
			expectBody:   "service is not running",
		},
		{
			name:      "instance not accepting actions",
			serviceID: "id-api",
			before: func(s *registry.MockStore, _ *bus.MockBus) {
				s.EXPECT().Phase().Return("startup")
			},
			expectStatus: http.StatusConflict,
			expectBody:   "instance is not accepting actions",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, mockStore, mockBus := newTestHandler(t)
			tt.before(mockStore, mockBus)

			mux := http.NewServeMux()
			mux.HandleFunc("POST /api/v1/services/{id}/restart", h.handleRestartService)

			req := httptest.NewRequest(http.MethodPost, "/api/v1/services/"+tt.serviceID+"/restart", nil)
			w := httptest.NewRecorder()

			mux.ServeHTTP(w, req)

			assert.Equal(t, tt.expectStatus, w.Code)
			assert.Contains(t, w.Body.String(), tt.expectBody)
		})
	}
}
