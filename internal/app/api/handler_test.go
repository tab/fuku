package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"go.yaml.in/yaml/v3"

	"fuku/internal/app/bus"
	"fuku/internal/app/instance"
	"fuku/internal/app/registry"
	"fuku/internal/config"
)

const testProject = "/Users/dev/projects/shop"

func testIdentity() instance.Identity {
	return instance.Identity{
		ID:          "1f0c6e4a-2b8d-4c3e-9a7f-5d6b8c0e1a24",
		Project:     testProject,
		Fingerprint: instance.Fingerprint(testProject),
	}
}

func Test_HandleLive(t *testing.T) {
	identity := testIdentity()
	h := &handler{identity: identity}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/live", nil)
	w := httptest.NewRecorder()

	h.handleLive(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotContains(t, w.Body.String(), testProject)

	var body LiveSerializer
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "alive", body.Status)
	assert.Equal(t, config.AppName, body.Product)
	assert.Equal(t, identity.ID, body.Instance)
	assert.Equal(t, identity.Fingerprint, body.Fingerprint)
	assert.Len(t, body.Fingerprint, instance.FingerprintLength)
}

func Test_HandleReady(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockStore := registry.NewMockStore(ctrl)
	h := &handler{store: mockStore}

	tests := []struct {
		name       string
		before     func()
		statusCode int
		body       string
	}{
		{
			name: "ready when store is resolved",
			before: func() {
				mockStore.EXPECT().IsResolved().Return(true)
			},
			statusCode: http.StatusOK,
			body:       `{"status":"ready"}`,
		},
		{
			name: "not ready when store is not resolved",
			before: func() {
				mockStore.EXPECT().IsResolved().Return(false)
			},
			statusCode: http.StatusServiceUnavailable,
			body:       `{"status":"not ready"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.before()

			req := httptest.NewRequest(http.MethodGet, "/api/v1/ready", nil)
			w := httptest.NewRecorder()

			h.handleReady(w, req)

			assert.Equal(t, tt.statusCode, w.Code)
			assert.JSONEq(t, tt.body, w.Body.String())
		})
	}
}

func Test_HandleStatus(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockStore := registry.NewMockStore(ctrl)
	identity := testIdentity()
	h := &handler{store: mockStore, bus: bus.NewMockBus(ctrl), identity: identity}

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
	assert.Equal(t, identity.ID, body.Instance)
	assert.Equal(t, testProject, body.Project)
	assert.Equal(t, "default", body.Profile)
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

const schemaRefPrefix = "#/components/schemas/"

type openapiSchema struct {
	Ref         string                   `yaml:"$ref"`
	Type        string                   `yaml:"type"`
	Format      string                   `yaml:"format"`
	Pattern     string                   `yaml:"pattern"`
	Enum        []string                 `yaml:"enum"`
	Description string                   `yaml:"description"`
	Required    []string                 `yaml:"required"`
	Properties  map[string]openapiSchema `yaml:"properties"`
}

type openapiMedia struct {
	Schema openapiSchema `yaml:"schema"`
}

type openapiResponse struct {
	Content map[string]openapiMedia `yaml:"content"`
}

type openapiOperation struct {
	Responses map[string]openapiResponse `yaml:"responses"`
}

type openapiPath struct {
	Get openapiOperation `yaml:"get"`
}

type openapiSpec struct {
	Paths      map[string]openapiPath `yaml:"paths"`
	Components struct {
		Schemas map[string]openapiSchema `yaml:"schemas"`
	} `yaml:"components"`
}

func loadOpenAPISpec(t *testing.T) openapiSpec {
	t.Helper()

	data, err := os.ReadFile(filepath.Join("..", "..", "..", "spec", "openapi.yaml"))
	require.NoError(t, err)

	var spec openapiSpec
	require.NoError(t, yaml.Unmarshal(data, &spec))

	return spec
}

// resolveType reports the declared type of a property, following a schema reference when present
func (s openapiSpec) resolveType(t *testing.T, property openapiSchema) string {
	t.Helper()

	if property.Ref == "" {
		return property.Type
	}

	name := strings.TrimPrefix(property.Ref, schemaRefPrefix)

	referenced, found := s.Components.Schemas[name]
	require.True(t, found, "schema %s referenced but not defined", name)

	return referenced.Type
}

// property reports a named property of a named schema
func (s openapiSpec) property(t *testing.T, schema, name string) openapiSchema {
	t.Helper()

	definition, found := s.Components.Schemas[schema]
	require.True(t, found, "schema %s is missing from the spec", schema)

	property, found := definition.Properties[name]
	require.True(t, found, "%s.%s is missing from the spec", schema, name)

	return property
}

// responseRef reports the schema reference the GET on a path declares for its 200 application/json response
func (s openapiSpec) responseRef(t *testing.T, path string) string {
	t.Helper()

	route, found := s.Paths[path]
	require.True(t, found, "path %s is missing from the spec", path)

	response, found := route.Get.Responses["200"]
	require.True(t, found, "path %s declares no 200 response", path)

	media, found := response.Content["application/json"]
	require.True(t, found, "path %s declares no application/json 200 response", path)

	return media.Schema.Ref
}

// openapiType maps the Go type a handler encodes onto the OpenAPI type the spec must declare
func openapiType(t *testing.T, typ reflect.Type) string {
	t.Helper()

	switch typ.Kind() {
	case reflect.String:
		return "string"
	case reflect.Int, reflect.Int64, reflect.Uint64:
		return "integer"
	case reflect.Float32, reflect.Float64:
		return "number"
	case reflect.Bool:
		return "boolean"
	case reflect.Struct:
		return "object"
	default:
		t.Fatalf("no OpenAPI type mapped for Go kind %s", typ.Kind())

		return ""
	}
}

func Test_OpenAPI_ResponseSchemas(t *testing.T) {
	spec := loadOpenAPISpec(t)

	tests := []struct {
		name       string
		path       string
		schema     string
		serializer any
	}{
		{
			name:       "live response",
			path:       "/live",
			schema:     "Live",
			serializer: LiveSerializer{},
		},
		{
			name:       "status response",
			path:       "/status",
			schema:     "Status",
			serializer: StatusSerializer{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, schemaRefPrefix+tt.schema, spec.responseRef(t, tt.path), "GET %s does not respond with %s", tt.path, tt.schema)

			schema, found := spec.Components.Schemas[tt.schema]
			require.True(t, found, "schema %s is missing from the spec", tt.schema)

			serializer := reflect.TypeOf(tt.serializer)

			for i := range serializer.NumField() {
				field := serializer.Field(i)
				tag, _, _ := strings.Cut(field.Tag.Get("json"), ",")

				property, found := schema.Properties[tag]
				require.True(t, found, "%s.%s is missing from the spec", tt.schema, tag)

				assert.Contains(t, schema.Required, tag, "%s.%s is not listed as required", tt.schema, tag)
				assert.Equal(t, openapiType(t, field.Type), spec.resolveType(t, property), "%s.%s declares the wrong type", tt.schema, tag)
			}
		})
	}
}

func Test_OpenAPI_IdentityConstraints(t *testing.T) {
	spec := loadOpenAPISpec(t)

	tests := []struct {
		name        string
		schema      string
		property    string
		format      string
		pattern     string
		enum        []string
		description string
	}{
		{
			name:        "live product is pinned to the product name",
			schema:      "Live",
			property:    "product",
			enum:        []string{config.AppName},
			description: "Always \"fuku\"",
		},
		{
			name:        "live instance is a uuid",
			schema:      "Live",
			property:    "instance",
			format:      "uuid",
			description: "new on every start",
		},
		{
			name:        "live fingerprint is a fixed-length lowercase hex digest",
			schema:      "Live",
			property:    "fingerprint",
			pattern:     fmt.Sprintf("^[0-9a-f]{%d}$", instance.FingerprintLength),
			description: "SHA-256 of the project directory",
		},
		{
			name:        "status instance is a uuid",
			schema:      "Status",
			property:    "instance",
			format:      "uuid",
			description: "new on every start",
		},
		{
			name:        "status project is a canonical absolute path",
			schema:      "Status",
			property:    "project",
			description: "Canonical absolute path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			property := spec.property(t, tt.schema, tt.property)

			assert.Equal(t, tt.format, property.Format, "%s.%s declares the wrong format", tt.schema, tt.property)
			assert.Equal(t, tt.pattern, property.Pattern, "%s.%s declares the wrong pattern", tt.schema, tt.property)
			assert.Equal(t, tt.enum, property.Enum, "%s.%s declares the wrong enum", tt.schema, tt.property)
			assert.Contains(t, property.Description, tt.description, "%s.%s does not document its promised value", tt.schema, tt.property)
		})
	}
}
