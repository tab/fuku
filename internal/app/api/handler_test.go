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
	"fuku/internal/app/errors"
	"fuku/internal/app/instance"
	"fuku/internal/app/registry"
	"fuku/internal/app/relay"
	"fuku/internal/config"
)

// testIdentity is the instance identity used by the handler tests
var testIdentity = instance.Identity{
	ID:          "1f0c6e4a-2b8d-4c3e-9a7f-5d6b8c0e1a24",
	Project:     "/tmp/project",
	Fingerprint: instance.Fingerprint("/tmp/project"),
}

func Test_HandleLive(t *testing.T) {
	h := &handler{identity: testIdentity}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/live", nil)
	w := httptest.NewRecorder()

	h.handleLive(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var body LiveSerializer
	require.NoError(t, json.NewDecoder(w.Body).Decode(&body))
	assert.Equal(t, "alive", body.Status)
	assert.Equal(t, config.AppName, body.Product)
	assert.Equal(t, testIdentity.ID, body.Instance)
	assert.Equal(t, testIdentity.Fingerprint, body.Project)
}

func Test_HandleLive_DoesNotDiscloseTheProjectPath(t *testing.T) {
	h := &handler{identity: testIdentity}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/live", nil)
	w := httptest.NewRecorder()

	h.handleLive(w, req)

	assert.NotContains(t, w.Body.String(), testIdentity.Project)
}

func Test_HandleReady(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockStore := registry.NewMockStore(ctrl)
	h := &handler{store: mockStore}

	tests := []struct {
		name       string
		before     func()
		statusCode int
		status     string
	}{
		{
			name: "ready when store is resolved",
			before: func() {
				mockStore.EXPECT().IsResolved().Return(true)
			},
			statusCode: http.StatusOK,
			status:     "ready",
		},
		{
			name: "not ready when store is not resolved",
			before: func() {
				mockStore.EXPECT().IsResolved().Return(false)
			},
			statusCode: http.StatusServiceUnavailable,
			status:     "not ready",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.before()

			req := httptest.NewRequest(http.MethodGet, "/api/v1/ready", nil)
			w := httptest.NewRecorder()

			h.handleReady(w, req)

			assert.Equal(t, tt.statusCode, w.Code)

			var body ProbeSerializer
			require.NoError(t, json.NewDecoder(w.Body).Decode(&body))
			assert.Equal(t, tt.status, body.Status)
		})
	}
}

func Test_HandleStatus(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockStore := registry.NewMockStore(ctrl)
	h := &handler{store: mockStore, bus: bus.NewMockBus(ctrl), identity: testIdentity}

	mockStore.EXPECT().Counts().Return(registry.StatusCounts{
		Total:   4,
		Running: 2,
		Stopped: 1,
		Failed:  1,
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
	assert.Equal(t, testIdentity.ID, body.Instance)
	assert.Equal(t, testIdentity.Project, body.Project)
	assert.Equal(t, string(bus.PhaseRunning), body.Phase)
	assert.Equal(t, int64(3600), body.Uptime)
	assert.Equal(t, 4, body.Services.Total)
	assert.Equal(t, 2, body.Services.Running)
	assert.Equal(t, 1, body.Services.Stopped)
	assert.Equal(t, 1, body.Services.Failed)
}

func Test_HandleListServices(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockStore := registry.NewMockStore(ctrl)
	h := &handler{store: mockStore, bus: bus.NewMockBus(ctrl)}

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
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := registry.NewMockStore(ctrl)
	h := &handler{store: mockStore, bus: bus.NewMockBus(ctrl)}

	tests := []struct {
		name         string
		serviceID    string
		before       func()
		expectStatus int
		expectBody   string
	}{
		{
			name:      "service found",
			serviceID: "id-api",
			before: func() {
				mockStore.EXPECT().Service("id-api").Return(registry.ServiceSnapshot{
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
			before: func() {
				mockStore.EXPECT().Service("id-unknown").Return(registry.ServiceSnapshot{}, false)
			},
			expectStatus: http.StatusNotFound,
			expectBody:   "service not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.before()

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

func Test_ToServiceSerializer_CarriesRevision(t *testing.T) {
	tests := []struct {
		name     string
		snapshot registry.ServiceSnapshot
		expect   uint64
	}{
		{
			name:     "running service reports its revision",
			snapshot: registry.ServiceSnapshot{ID: "id-api", Status: registry.StatusRunning, LifecycleSeq: 42},
			expect:   42,
		},
		{
			name:     "failed service reports its revision",
			snapshot: registry.ServiceSnapshot{ID: "id-api", Status: registry.StatusFailed, LifecycleSeq: 7},
			expect:   7,
		},
		{
			name:     "service that never transitioned reports zero",
			snapshot: registry.ServiceSnapshot{ID: "id-api", Status: registry.StatusStopped},
			expect:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expect, toServiceSerializer(tt.snapshot).Revision)
		})
	}
}

func Test_HandleGetService_SerializesRevision(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := registry.NewMockStore(ctrl)
	h := &handler{store: mockStore, bus: bus.NewMockBus(ctrl)}

	mockStore.EXPECT().Service("id-api").Return(registry.ServiceSnapshot{
		ID:           "id-api",
		Name:         "api",
		Status:       registry.StatusFailed,
		LifecycleSeq: 11,
	}, true)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/services/{id}", h.handleGetService)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/services/id-api", nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"revision":11`)
}

func Test_HandleStartService(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := registry.NewMockStore(ctrl)
	mockBus := bus.NewMockBus(ctrl)
	h := &handler{store: mockStore, bus: mockBus}

	tests := []struct {
		name         string
		serviceID    string
		before       func()
		expectStatus int
		expectBody   string
	}{
		{
			name:      "start stopped service",
			serviceID: "id-api",
			before: func() {
				mockStore.EXPECT().Phase().Return(string(bus.PhaseRunning))
				mockStore.EXPECT().Service("id-api").Return(registry.ServiceSnapshot{ID: "id-api", Name: "api", Status: registry.StatusStopped}, true)
				mockBus.EXPECT().Publish(gomock.Any()).Do(func(msg bus.Message) {
					assert.Equal(t, bus.CommandStartService, msg.Type)
				})
			},
			expectStatus: http.StatusAccepted,
			expectBody:   "starting",
		},
		{
			name:      "start failed service",
			serviceID: "id-api",
			before: func() {
				mockStore.EXPECT().Phase().Return(string(bus.PhaseRunning))
				mockStore.EXPECT().Service("id-api").Return(registry.ServiceSnapshot{ID: "id-api", Name: "api", Status: registry.StatusFailed}, true)
				mockBus.EXPECT().Publish(gomock.Any())
			},
			expectStatus: http.StatusAccepted,
			expectBody:   "starting",
		},
		{
			name:      "cannot start running service",
			serviceID: "id-api",
			before: func() {
				mockStore.EXPECT().Phase().Return(string(bus.PhaseRunning))
				mockStore.EXPECT().Service("id-api").Return(registry.ServiceSnapshot{ID: "id-api", Name: "api", Status: registry.StatusRunning}, true)
			},
			expectStatus: http.StatusConflict,
			expectBody:   "service cannot be started",
		},
		{
			name:      "service not found",
			serviceID: "id-unknown",
			before: func() {
				mockStore.EXPECT().Phase().Return(string(bus.PhaseRunning))
				mockStore.EXPECT().Service("id-unknown").Return(registry.ServiceSnapshot{}, false)
			},
			expectStatus: http.StatusNotFound,
			expectBody:   "service not found",
		},
		{
			name:      "instance not accepting actions",
			serviceID: "id-api",
			before: func() {
				mockStore.EXPECT().Phase().Return("startup")
			},
			expectStatus: http.StatusConflict,
			expectBody:   "instance is not accepting actions",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.before()

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
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := registry.NewMockStore(ctrl)
	mockBus := bus.NewMockBus(ctrl)
	h := &handler{store: mockStore, bus: mockBus}

	tests := []struct {
		name         string
		serviceID    string
		before       func()
		expectStatus int
		expectBody   string
	}{
		{
			name:      "stop running service",
			serviceID: "id-api",
			before: func() {
				mockStore.EXPECT().Phase().Return(string(bus.PhaseRunning))
				mockStore.EXPECT().Service("id-api").Return(registry.ServiceSnapshot{ID: "id-api", Name: "api", Status: registry.StatusRunning}, true)
				mockBus.EXPECT().Publish(gomock.Any()).Do(func(msg bus.Message) {
					assert.Equal(t, bus.CommandStopService, msg.Type)
				})
			},
			expectStatus: http.StatusAccepted,
			expectBody:   "stopping",
		},
		{
			name:      "cannot stop stopped service",
			serviceID: "id-api",
			before: func() {
				mockStore.EXPECT().Phase().Return(string(bus.PhaseRunning))
				mockStore.EXPECT().Service("id-api").Return(registry.ServiceSnapshot{ID: "id-api", Name: "api", Status: registry.StatusStopped}, true)
			},
			expectStatus: http.StatusConflict,
			expectBody:   "service is not running",
		},
		{
			name:      "service not found",
			serviceID: "id-unknown",
			before: func() {
				mockStore.EXPECT().Phase().Return(string(bus.PhaseRunning))
				mockStore.EXPECT().Service("id-unknown").Return(registry.ServiceSnapshot{}, false)
			},
			expectStatus: http.StatusNotFound,
			expectBody:   "service not found",
		},
		{
			name:      "instance not accepting actions",
			serviceID: "id-api",
			before: func() {
				mockStore.EXPECT().Phase().Return(string(bus.PhaseStopping))
			},
			expectStatus: http.StatusConflict,
			expectBody:   "instance is not accepting actions",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.before()

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
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := registry.NewMockStore(ctrl)
	mockBus := bus.NewMockBus(ctrl)
	h := &handler{store: mockStore, bus: mockBus}

	tests := []struct {
		name         string
		serviceID    string
		before       func()
		expectStatus int
		expectBody   string
	}{
		{
			name:      "restart running service",
			serviceID: "id-api",
			before: func() {
				mockStore.EXPECT().Phase().Return(string(bus.PhaseRunning))
				mockStore.EXPECT().Service("id-api").Return(registry.ServiceSnapshot{ID: "id-api", Name: "api", Status: registry.StatusRunning}, true)
				mockBus.EXPECT().Publish(gomock.Any()).Do(func(msg bus.Message) {
					assert.Equal(t, bus.CommandRestartService, msg.Type)
				})
			},
			expectStatus: http.StatusAccepted,
			expectBody:   "restarting",
		},
		{
			name:      "restart stopped service",
			serviceID: "id-api",
			before: func() {
				mockStore.EXPECT().Phase().Return(string(bus.PhaseRunning))
				mockStore.EXPECT().Service("id-api").Return(registry.ServiceSnapshot{ID: "id-api", Name: "api", Status: registry.StatusStopped}, true)
				mockBus.EXPECT().Publish(gomock.Any()).Do(func(msg bus.Message) {
					assert.Equal(t, bus.CommandRestartService, msg.Type)
				})
			},
			expectStatus: http.StatusAccepted,
			expectBody:   "restarting",
		},
		{
			name:      "restart failed service",
			serviceID: "id-api",
			before: func() {
				mockStore.EXPECT().Phase().Return(string(bus.PhaseRunning))
				mockStore.EXPECT().Service("id-api").Return(registry.ServiceSnapshot{ID: "id-api", Name: "api", Status: registry.StatusFailed}, true)
				mockBus.EXPECT().Publish(gomock.Any()).Do(func(msg bus.Message) {
					assert.Equal(t, bus.CommandRestartService, msg.Type)
				})
			},
			expectStatus: http.StatusAccepted,
			expectBody:   "restarting",
		},
		{
			name:      "cannot restart starting service",
			serviceID: "id-api",
			before: func() {
				mockStore.EXPECT().Phase().Return(string(bus.PhaseRunning))
				mockStore.EXPECT().Service("id-api").Return(registry.ServiceSnapshot{ID: "id-api", Name: "api", Status: registry.StatusStarting}, true)
			},
			expectStatus: http.StatusConflict,
			expectBody:   "service cannot be restarted",
		},
		{
			name:      "service not found",
			serviceID: "id-unknown",
			before: func() {
				mockStore.EXPECT().Phase().Return(string(bus.PhaseRunning))
				mockStore.EXPECT().Service("id-unknown").Return(registry.ServiceSnapshot{}, false)
			},
			expectStatus: http.StatusNotFound,
			expectBody:   "service not found",
		},
		{
			name:      "instance not accepting actions",
			serviceID: "id-api",
			before: func() {
				mockStore.EXPECT().Phase().Return("startup")
			},
			expectStatus: http.StatusConflict,
			expectBody:   "instance is not accepting actions",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.before()

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

func Test_HandleLogs(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockJournal := NewMockJournal(ctrl)
	mockStore := registry.NewMockStore(ctrl)
	h := &handler{store: mockStore, journal: mockJournal}

	stamp := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name       string
		target     string
		before     func()
		statusCode int
		assertBody func(t *testing.T, body []byte)
	}{
		{
			name:   "returns buffered lines with the default tail",
			target: "/api/v1/logs",
			before: func() {
				mockJournal.EXPECT().
					History(relay.HistoryQuery{Tail: config.APILogsDefaultTail}).
					Return([]relay.LogMessage{{Service: "api", Message: "ready", Timestamp: stamp}})
			},
			statusCode: http.StatusOK,
			assertBody: func(t *testing.T, body []byte) {
				t.Helper()

				var payload LogListSerializer
				require.NoError(t, json.Unmarshal(body, &payload))
				require.Len(t, payload.Lines, 1)
				assert.Equal(t, "api", payload.Lines[0].Service)
				assert.Equal(t, "ready", payload.Lines[0].Message)
				assert.Equal(t, "2026-08-24T12:00:00Z", payload.Lines[0].Timestamp)
				assert.Equal(t, config.APILogsDefaultTail, payload.Tail)
			},
		},
		{
			name:   "honours an explicit tail",
			target: "/api/v1/logs?tail=5",
			before: func() {
				mockJournal.EXPECT().
					History(relay.HistoryQuery{Tail: 5}).
					Return(nil)
			},
			statusCode: http.StatusOK,
			assertBody: func(t *testing.T, body []byte) {
				t.Helper()

				var payload LogListSerializer
				require.NoError(t, json.Unmarshal(body, &payload))
				assert.Empty(t, payload.Lines)
			},
		},
		{
			name:       "rejects a tail that is not a positive number",
			target:     "/api/v1/logs?tail=0",
			before:     func() {},
			statusCode: http.StatusBadRequest,
			assertBody: func(t *testing.T, body []byte) {
				t.Helper()

				var payload ErrorSerializer
				require.NoError(t, json.Unmarshal(body, &payload))
				assert.Equal(t, errors.ErrAPIInvalidTail.Error(), payload.Error)
			},
		},
		{
			name:       "rejects an unparsable since",
			target:     "/api/v1/logs?since=yesterday",
			before:     func() {},
			statusCode: http.StatusBadRequest,
			assertBody: func(t *testing.T, body []byte) {
				t.Helper()

				var payload ErrorSerializer
				require.NoError(t, json.Unmarshal(body, &payload))
				assert.Equal(t, errors.ErrAPIInvalidSince.Error(), payload.Error)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.before()

			req := httptest.NewRequest(http.MethodGet, tt.target, nil)
			w := httptest.NewRecorder()

			h.handleLogs(w, req)

			assert.Equal(t, tt.statusCode, w.Code)
			tt.assertBody(t, w.Body.Bytes())
		})
	}
}

func Test_HandleLogs_FiltersByServiceID(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockStore := registry.NewMockStore(ctrl)
	mockJournal := NewMockJournal(ctrl)
	h := &handler{store: mockStore, journal: mockJournal}

	tests := []struct {
		name       string
		target     string
		before     func()
		statusCode int
	}{
		{
			name:   "maps every service id to its name",
			target: "/api/v1/logs?service=id-a&service=id-b",
			before: func() {
				mockStore.EXPECT().Service("id-a").Return(registry.ServiceSnapshot{Name: "api"}, true)
				mockStore.EXPECT().Service("id-b").Return(registry.ServiceSnapshot{Name: "web"}, true)
				mockJournal.EXPECT().
					History(relay.HistoryQuery{Services: []string{"api", "web"}, Tail: config.APILogsDefaultTail}).
					Return(nil)
			},
			statusCode: http.StatusOK,
		},
		{
			name:   "rejects an unknown service id",
			target: "/api/v1/logs?service=missing",
			before: func() {
				mockStore.EXPECT().Service("missing").Return(registry.ServiceSnapshot{}, false)
			},
			statusCode: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.before()

			req := httptest.NewRequest(http.MethodGet, tt.target, nil)
			w := httptest.NewRecorder()

			h.handleLogs(w, req)

			assert.Equal(t, tt.statusCode, w.Code)
		})
	}
}

func Test_HandleLogs_CapsTailAtTheBufferSize(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockJournal := NewMockJournal(ctrl)
	h := &handler{journal: mockJournal}

	mockJournal.EXPECT().
		History(relay.HistoryQuery{Tail: config.SocketLogsHistorySize}).
		Return(nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/logs?tail=999999", nil)
	w := httptest.NewRecorder()

	h.handleLogs(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func Test_HandleServiceLogs(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockStore := registry.NewMockStore(ctrl)
	mockJournal := NewMockJournal(ctrl)
	h := &handler{store: mockStore, journal: mockJournal}

	tests := []struct {
		name       string
		id         string
		before     func()
		statusCode int
	}{
		{
			name: "reads only the named service",
			id:   "service-id",
			before: func() {
				mockStore.EXPECT().Service("service-id").Return(registry.ServiceSnapshot{Name: "api"}, true)
				mockJournal.EXPECT().
					History(relay.HistoryQuery{Services: []string{"api"}, Tail: config.APILogsDefaultTail}).
					Return(nil)
			},
			statusCode: http.StatusOK,
		},
		{
			name: "reports an unknown service id",
			id:   "missing",
			before: func() {
				mockStore.EXPECT().Service("missing").Return(registry.ServiceSnapshot{}, false)
			},
			statusCode: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.before()

			req := httptest.NewRequest(http.MethodGet, "/api/v1/services/"+tt.id+"/logs", nil)
			req.SetPathValue("id", tt.id)

			w := httptest.NewRecorder()

			h.handleServiceLogs(w, req)

			assert.Equal(t, tt.statusCode, w.Code)
		})
	}
}
