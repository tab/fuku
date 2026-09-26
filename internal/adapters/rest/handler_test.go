package rest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"go.yaml.in/yaml/v3"

	"fuku/internal/adapters/instance"
	"fuku/internal/app/services"
	"fuku/internal/contracts"
	"fuku/internal/model"
	"fuku/internal/platform/buildinfo"
)

const testProject = "/Users/dev/projects/shop"

// readFrom stands in for Registry.Read and runs the callback on the fixture snapshot
func readFrom(snapshot *model.Snapshot) func(func(*model.Snapshot)) {
	return func(fn func(*model.Snapshot)) {
		fn(snapshot)
	}
}

func Test_HandleLive(t *testing.T) {
	identity := model.Instance{
		ID:          "1f0c6e4a-2b8d-4c3e-9a7f-5d6b8c0e1a24",
		Project:     testProject,
		Fingerprint: instance.Fingerprint(testProject),
	}

	h := &handler{identity: identity}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/live", nil)
	w := httptest.NewRecorder()

	h.handleLive(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotContains(t, w.Body.String(), testProject)

	var body LiveSerializer
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "alive", body.Status)
	assert.Equal(t, buildinfo.AppName, body.Product)
	assert.Equal(t, identity.ID, body.Instance)
	assert.Equal(t, identity.Fingerprint, body.Fingerprint)
	assert.Len(t, body.Fingerprint, instance.FingerprintLength)
}

func Test_HandleReady(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRegistry := NewMockRegistry(ctrl)

	h := &handler{registry: mockRegistry}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/ready", nil)

	tests := []struct {
		name       string
		before     func()
		recorder   *httptest.ResponseRecorder
		statusCode int
		body       string
	}{
		{
			name: "ready when store is resolved",
			before: func() {
				mockRegistry.EXPECT().Read(gomock.Any()).Do(readFrom(&model.Snapshot{Resolved: true}))
			},
			recorder:   httptest.NewRecorder(),
			statusCode: http.StatusOK,
			body:       `{"status":"ready"}`,
		},
		{
			name: "not ready when store is not resolved",
			before: func() {
				mockRegistry.EXPECT().Read(gomock.Any()).Do(readFrom(&model.Snapshot{Resolved: false}))
			},
			recorder:   httptest.NewRecorder(),
			statusCode: http.StatusServiceUnavailable,
			body:       `{"status":"not ready"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.before()

			h.handleReady(tt.recorder, req)

			assert.Equal(t, tt.statusCode, tt.recorder.Code)
			assert.JSONEq(t, tt.body, tt.recorder.Body.String())
		})
	}
}

func Test_HandleStatus(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRegistry := NewMockRegistry(ctrl)

	identity := model.Instance{
		ID:          "1f0c6e4a-2b8d-4c3e-9a7f-5d6b8c0e1a24",
		Project:     testProject,
		Fingerprint: instance.Fingerprint(testProject),
	}

	h := &handler{registry: mockRegistry, identity: identity}

	snapshot := &model.Snapshot{
		Phase:     model.PhaseRunning,
		Profile:   "default",
		StartedAt: time.Now().Add(-time.Hour),
		Services: map[string]*model.Service{
			"id-api":    {Status: model.StatusRunning},
			"id-web":    {Status: model.StatusRunning},
			"id-worker": {Status: model.StatusStopped},
			"id-db":     {Status: model.StatusFailed},
		},
	}

	mockRegistry.EXPECT().Read(gomock.Any()).Do(readFrom(snapshot))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/status", nil)
	w := httptest.NewRecorder()

	h.handleStatus(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var body StatusSerializer
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, identity.ID, body.Instance)
	assert.Equal(t, testProject, body.Project)
	assert.Equal(t, "default", body.Profile)
	assert.Equal(t, string(model.PhaseRunning), body.Phase)
	assert.Equal(t, int64(3600), body.Uptime)
	assert.Equal(t, 4, body.Services.Total)
	assert.Equal(t, 2, body.Services.Running)
	assert.Equal(t, 1, body.Services.Stopped)
	assert.Equal(t, 1, body.Services.Failed)
}

func Test_HandleListServices(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRegistry := NewMockRegistry(ctrl)

	h := &handler{registry: mockRegistry}

	now := time.Now()
	db := &model.Service{ID: "id-1", Name: "db", Tier: "foundation", Status: model.StatusRunning, Process: model.Process{PID: 100, CPU: 1.5, Memory: 1024, StartedAt: now}}
	api := &model.Service{ID: "id-2", Name: "api", Tier: "application", Status: model.StatusStopped}
	worker := &model.Service{ID: "id-3", Name: "worker", Tier: "application", Status: model.StatusStarting, Process: model.Process{PID: 200, CPU: 0.5, Memory: 512, StartedAt: now}}
	snapshot := &model.Snapshot{
		Tiers: []*model.Tier{
			{Name: "foundation", Services: []*model.Service{db}},
			{Name: "application", Services: []*model.Service{api, worker}},
		},
		Services: map[string]*model.Service{"id-1": db, "id-2": api, "id-3": worker},
	}

	mockRegistry.EXPECT().Read(gomock.Any()).Do(readFrom(snapshot))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/services", nil)
	w := httptest.NewRecorder()

	h.handleListServices(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var body ServiceListSerializer
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Len(t, body.Services, 3)
	assert.Equal(t, "db", body.Services[0].Name)
	assert.Equal(t, model.StatusRunning, body.Services[0].Status)
	assert.Equal(t, 100, body.Services[0].PID)
	assert.InDelta(t, 1.5, body.Services[0].CPU, 0.01)
	assert.Equal(t, uint64(1024), body.Services[0].Memory)

	assert.Equal(t, "api", body.Services[1].Name)
	assert.Equal(t, model.StatusStopped, body.Services[1].Status)
	assert.Equal(t, 0, body.Services[1].PID)
	assert.Equal(t, int64(0), body.Services[1].Uptime)

	assert.Equal(t, "worker", body.Services[2].Name)
	assert.Equal(t, model.StatusStarting, body.Services[2].Status)
	assert.Equal(t, 0, body.Services[2].PID)
	assert.InDelta(t, 0, body.Services[2].CPU, 0.01)
	assert.Equal(t, uint64(0), body.Services[2].Memory)
	assert.Equal(t, int64(0), body.Services[2].Uptime)
}

func Test_HandleGetService(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRegistry := NewMockRegistry(ctrl)

	h := &handler{registry: mockRegistry}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/services/{id}", h.handleGetService)

	tests := []struct {
		name         string
		before       func()
		request      *http.Request
		recorder     *httptest.ResponseRecorder
		expectStatus int
		expectBody   string
	}{
		{
			name: "service found",
			before: func() {
				mockRegistry.EXPECT().Read(gomock.Any()).Do(readFrom(&model.Snapshot{Services: map[string]*model.Service{"id-api": {
					ID:     "id-api",
					Name:   "api",
					Tier:   "foundation",
					Status: model.StatusRunning,
					Process: model.Process{
						PID: 1234,
					},
				}}}))
			},
			request:      httptest.NewRequest(http.MethodGet, "/api/v1/services/id-api", nil),
			recorder:     httptest.NewRecorder(),
			expectStatus: http.StatusOK,
			expectBody:   "api",
		},
		{
			name: "service not found",
			before: func() {
				mockRegistry.EXPECT().Read(gomock.Any()).Do(readFrom(&model.Snapshot{}))
			},
			request:      httptest.NewRequest(http.MethodGet, "/api/v1/services/id-unknown", nil),
			recorder:     httptest.NewRecorder(),
			expectStatus: http.StatusNotFound,
			expectBody:   "service not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.before()

			mux.ServeHTTP(tt.recorder, tt.request)

			assert.Equal(t, tt.expectStatus, tt.recorder.Code)
			assert.Contains(t, tt.recorder.Body.String(), tt.expectBody)
		})
	}
}

func Test_HandleAction(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockControl := NewMockControl(ctrl)

	h := &handler{control: mockControl}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/services/{id}/start", h.handleStartService)
	mux.HandleFunc("POST /api/v1/services/{id}/stop", h.handleStopService)
	mux.HandleFunc("POST /api/v1/services/{id}/restart", h.handleRestartService)

	api := model.Service{ID: "id-api", Name: "api"}

	tests := []struct {
		name         string
		before       func()
		request      *http.Request
		recorder     *httptest.ResponseRecorder
		expectStatus int
		expectBody   string
	}{
		{
			name: "start admitted",
			before: func() {
				mockControl.EXPECT().Start("id-api").Return(services.Admission{Service: api, Action: contracts.ActionStart, Status: model.StatusStarting}, nil)
			},
			request:      httptest.NewRequest(http.MethodPost, "/api/v1/services/id-api/start", nil),
			recorder:     httptest.NewRecorder(),
			expectStatus: http.StatusAccepted,
			expectBody:   `{"id":"id-api","name":"api","action":"start","status":"starting"}`,
		},
		{
			name: "stop admitted",
			before: func() {
				mockControl.EXPECT().Stop("id-api").Return(services.Admission{Service: api, Action: contracts.ActionStop, Status: model.StatusStopping}, nil)
			},
			request:      httptest.NewRequest(http.MethodPost, "/api/v1/services/id-api/stop", nil),
			recorder:     httptest.NewRecorder(),
			expectStatus: http.StatusAccepted,
			expectBody:   `{"id":"id-api","name":"api","action":"stop","status":"stopping"}`,
		},
		{
			name: "restart admitted",
			before: func() {
				mockControl.EXPECT().Restart("id-api").Return(services.Admission{Service: api, Action: contracts.ActionRestart, Status: model.StatusRestarting}, nil)
			},
			request:      httptest.NewRequest(http.MethodPost, "/api/v1/services/id-api/restart", nil),
			recorder:     httptest.NewRecorder(),
			expectStatus: http.StatusAccepted,
			expectBody:   `{"id":"id-api","name":"api","action":"restart","status":"restarting"}`,
		},
		{
			name: "start not allowed",
			before: func() {
				mockControl.EXPECT().Start("id-api").Return(services.Admission{}, fmt.Errorf("%w: %s", contracts.ErrActionNotAllowed, contracts.ActionStart))
			},
			request:      httptest.NewRequest(http.MethodPost, "/api/v1/services/id-api/start", nil),
			recorder:     httptest.NewRecorder(),
			expectStatus: http.StatusConflict,
			expectBody:   `{"error":"service cannot be started"}`,
		},
		{
			name: "stop not allowed",
			before: func() {
				mockControl.EXPECT().Stop("id-api").Return(services.Admission{}, fmt.Errorf("%w: %s", contracts.ErrActionNotAllowed, contracts.ActionStop))
			},
			request:      httptest.NewRequest(http.MethodPost, "/api/v1/services/id-api/stop", nil),
			recorder:     httptest.NewRecorder(),
			expectStatus: http.StatusConflict,
			expectBody:   `{"error":"service is not running"}`,
		},
		{
			name: "restart not allowed",
			before: func() {
				mockControl.EXPECT().Restart("id-api").Return(services.Admission{}, fmt.Errorf("%w: %s", contracts.ErrActionNotAllowed, contracts.ActionRestart))
			},
			request:      httptest.NewRequest(http.MethodPost, "/api/v1/services/id-api/restart", nil),
			recorder:     httptest.NewRecorder(),
			expectStatus: http.StatusConflict,
			expectBody:   `{"error":"service cannot be restarted"}`,
		},
		{
			name: "busy service answers with the action's conflict",
			before: func() {
				mockControl.EXPECT().Restart("id-api").Return(services.Admission{}, contracts.ErrServiceBusy)
			},
			request:      httptest.NewRequest(http.MethodPost, "/api/v1/services/id-api/restart", nil),
			recorder:     httptest.NewRecorder(),
			expectStatus: http.StatusConflict,
			expectBody:   `{"error":"service cannot be restarted"}`,
		},
		{
			name: "service not found",
			before: func() {
				mockControl.EXPECT().Start("id-unknown").Return(services.Admission{}, contracts.ErrServiceNotFound)
			},
			request:      httptest.NewRequest(http.MethodPost, "/api/v1/services/id-unknown/start", nil),
			recorder:     httptest.NewRecorder(),
			expectStatus: http.StatusNotFound,
			expectBody:   `{"error":"service not found"}`,
		},
		{
			name: "instance not accepting actions",
			before: func() {
				mockControl.EXPECT().Stop("id-api").Return(services.Admission{}, contracts.ErrNotAccepting)
			},
			request:      httptest.NewRequest(http.MethodPost, "/api/v1/services/id-api/stop", nil),
			recorder:     httptest.NewRecorder(),
			expectStatus: http.StatusConflict,
			expectBody:   `{"error":"instance is not accepting actions"}`,
		},
		{
			name: "bus closed",
			before: func() {
				mockControl.EXPECT().Start("id-api").Return(services.Admission{}, contracts.ErrBusClosed)
			},
			request:      httptest.NewRequest(http.MethodPost, "/api/v1/services/id-api/start", nil),
			recorder:     httptest.NewRecorder(),
			expectStatus: http.StatusConflict,
			expectBody:   `{"error":"instance is not accepting actions"}`,
		},
		{
			name: "bus overloaded",
			before: func() {
				mockControl.EXPECT().Restart("id-api").Return(services.Admission{}, contracts.ErrBusOverloaded)
			},
			request:      httptest.NewRequest(http.MethodPost, "/api/v1/services/id-api/restart", nil),
			recorder:     httptest.NewRecorder(),
			expectStatus: http.StatusInternalServerError,
			expectBody:   `{"error":"instance is overloaded"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.before()

			mux.ServeHTTP(tt.recorder, tt.request)

			assert.Equal(t, tt.expectStatus, tt.recorder.Code)
			assert.JSONEq(t, tt.expectBody, tt.recorder.Body.String())
		})
	}
}

const schemaRefPrefix = "#/components/schemas/"
const responseRefPrefix = "#/components/responses/"

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
	Ref     string                  `yaml:"$ref"`
	Content map[string]openapiMedia `yaml:"content"`
}

type openapiOperation struct {
	Responses map[string]openapiResponse `yaml:"responses"`
}

type openapiPath struct {
	Get  openapiOperation `yaml:"get"`
	Post openapiOperation `yaml:"post"`
}

type openapiSpec struct {
	Paths      map[string]openapiPath `yaml:"paths"`
	Components struct {
		Schemas   map[string]openapiSchema   `yaml:"schemas"`
		Responses map[string]openapiResponse `yaml:"responses"`
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

// responseRef reports the schema reference a path's method declares for a status's application/json response
func (s openapiSpec) responseRef(t *testing.T, method, path, status string) string {
	t.Helper()

	route, found := s.Paths[path]
	require.True(t, found, "path %s is missing from the spec", path)

	operation := route.Get
	if method == http.MethodPost {
		operation = route.Post
	}

	response, found := operation.Responses[status]
	require.True(t, found, "%s %s declares no %s response", method, path, status)

	if response.Ref != "" {
		name := strings.TrimPrefix(response.Ref, responseRefPrefix)

		resolved, found := s.Components.Responses[name]
		require.True(t, found, "response %s referenced but not defined", name)

		response = resolved
	}

	media, found := response.Content["application/json"]
	require.True(t, found, "%s %s declares no application/json %s response", method, path, status)

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
	case reflect.Slice:
		return "array"
	default:
		t.Fatalf("no OpenAPI type mapped for Go kind %s", typ.Kind())

		return ""
	}
}

func Test_OpenAPI_ResponseSchemas(t *testing.T) {
	spec := loadOpenAPISpec(t)

	tests := []struct {
		name       string
		method     string
		path       string
		status     string
		schema     string
		serializer any
	}{
		{
			name:       "live response",
			method:     http.MethodGet,
			path:       "/live",
			status:     "200",
			schema:     "Live",
			serializer: LiveSerializer{},
		},
		{
			name:       "status response",
			method:     http.MethodGet,
			path:       "/status",
			status:     "200",
			schema:     "Status",
			serializer: StatusSerializer{},
		},
		{
			name:       "service response",
			method:     http.MethodGet,
			path:       "/services/{id}",
			status:     "200",
			schema:     "Service",
			serializer: ServiceSerializer{},
		},
		{
			name:       "service list response",
			method:     http.MethodGet,
			path:       "/services",
			status:     "200",
			schema:     "ServiceList",
			serializer: ServiceListSerializer{},
		},
		{
			name:       "service action accepted response",
			method:     http.MethodPost,
			path:       "/services/{id}/start",
			status:     "202",
			schema:     "ServiceActionAccepted",
			serializer: ActionSerializer{},
		},
		{
			name:       "probe response",
			method:     http.MethodGet,
			path:       "/ready",
			status:     "200",
			schema:     "Probe",
			serializer: ProbeSerializer{},
		},
		{
			name:       "error response",
			method:     http.MethodGet,
			path:       "/services/{id}",
			status:     "404",
			schema:     "Error",
			serializer: ErrorSerializer{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, schemaRefPrefix+tt.schema, spec.responseRef(t, tt.method, tt.path, tt.status), "%s %s does not respond with %s", tt.method, tt.path, tt.schema)

			schema, found := spec.Components.Schemas[tt.schema]
			require.True(t, found, "schema %s is missing from the spec", tt.schema)

			serializer := reflect.TypeOf(tt.serializer)

			for field := range serializer.Fields() {
				tag, opts, _ := strings.Cut(field.Tag.Get("json"), ",")

				property, found := schema.Properties[tag]
				require.True(t, found, "%s.%s is missing from the spec", tt.schema, tag)

				assert.Equal(t, opts != "omitempty", slices.Contains(schema.Required, tag), "%s.%s is required exactly when its field is not omitempty", tt.schema, tag)
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
			enum:        []string{buildinfo.AppName},
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
