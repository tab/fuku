package rest

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"fuku/internal/adapters/instance"
	"fuku/internal/app/services"
	"fuku/internal/contracts"
	"fuku/internal/model"
)

func Test_Server_StartAndShutdown(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPublisher := NewMockPublisher(ctrl)
	mockRegistry := NewMockRegistry(ctrl)

	var address string

	isStarted := gomock.Cond(func(msg contracts.Message) bool {
		return msg.Type == contracts.EventAPIStarted
	})
	recordAddress := func(msg contracts.Message) error {
		address = msg.Data.(contracts.APIStarted).Listen

		return nil
	}

	mockPublisher.EXPECT().Publish(isStarted).DoAndReturn(recordAddress)
	mockPublisher.EXPECT().Publish(gomock.Any()).Return(nil).AnyTimes()
	mockRegistry.EXPECT().Read(gomock.Any()).Do(readFrom(&model.Snapshot{})).AnyTimes()

	log := slog.New(slog.DiscardHandler)

	options := Options{Listen: "127.0.0.1:0"}

	identity := model.Instance{
		ID:          "1f0c6e4a-2b8d-4c3e-9a7f-5d6b8c0e1a24",
		Project:     testProject,
		Fingerprint: instance.Fingerprint(testProject),
	}

	s := NewServer(options, mockRegistry, nil, mockPublisher, identity, log)
	s.Start(t.Context())

	require.NotNil(t, s.httpServer)
	assert.NotEmpty(t, address)
	assert.NotContains(t, address, ":0")
	assert.Equal(t, address, s.Address(t.Context()))

	resp, err := http.Get("http://" + address + "/api/v1/live")
	require.NoError(t, err)

	var live LiveSerializer

	err = json.NewDecoder(resp.Body).Decode(&live)
	resp.Body.Close()
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Empty(t, resp.Header.Get("Access-Control-Allow-Origin"))
	assert.Equal(t, identity.ID, live.Instance)
	assert.Equal(t, identity.Fingerprint, live.Fingerprint)

	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()

	s.Stop(ctx)
}

func Test_Server_Start_FallsBackWhenBasePortIsOccupied(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPublisher := NewMockPublisher(ctrl)

	var address string

	isStarted := gomock.Cond(func(msg contracts.Message) bool {
		return msg.Type == contracts.EventAPIStarted
	})
	recordAddress := func(msg contracts.Message) error {
		address = msg.Data.(contracts.APIStarted).Listen

		return nil
	}

	mockPublisher.EXPECT().Publish(isStarted).DoAndReturn(recordAddress)
	mockPublisher.EXPECT().Publish(gomock.Any()).Return(nil).AnyTimes()

	log := slog.New(slog.DiscardHandler)

	occupied, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	defer occupied.Close()

	s := NewServer(Options{Listen: occupied.Addr().String()}, nil, nil, mockPublisher, model.Instance{}, log)
	s.Start(t.Context())

	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()

	defer s.Stop(ctx)

	require.NotNil(t, s.httpServer)
	assert.NotEqual(t, occupied.Addr().String(), address)
	assert.NotEmpty(t, address)
}

func Test_Server_Start_GuardAndToken(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPublisher := NewMockPublisher(ctrl)
	mockRegistry := NewMockRegistry(ctrl)

	requested := make(chan int, 1)

	isRequested := gomock.Cond(func(msg contracts.Message) bool {
		return msg.Type == contracts.EventAPIRequested
	})
	recordStatus := func(msg contracts.Message) error {
		requested <- msg.Data.(contracts.APIRequested).Status

		return nil
	}

	mockPublisher.EXPECT().Publish(isRequested).DoAndReturn(recordStatus).AnyTimes()
	mockPublisher.EXPECT().Publish(gomock.Any()).Return(nil).AnyTimes()
	mockRegistry.EXPECT().Read(gomock.Any()).Do(readFrom(&model.Snapshot{Resolved: true})).AnyTimes()

	log := slog.New(slog.DiscardHandler)

	s := NewServer(Options{Listen: "127.0.0.1:0", Token: "test-token"}, mockRegistry, nil, mockPublisher, model.Instance{}, log)
	s.Start(t.Context())

	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()

	defer s.Stop(ctx)

	address := s.Address(t.Context())
	valid := []string{"Bearer test-token"}

	tests := []struct {
		name          string
		path          string
		host          string
		origin        []string
		authorization []string
		expectStatus  int
	}{
		{
			name:          "a valid bearer passes",
			path:          "/api/v1/status",
			authorization: valid,
			expectStatus:  http.StatusOK,
		},
		{
			name:         "a missing bearer is unauthorized",
			path:         "/api/v1/status",
			expectStatus: http.StatusUnauthorized,
		},
		{
			name:          "a wrong bearer is unauthorized",
			path:          "/api/v1/status",
			authorization: []string{"Bearer wrong-token"},
			expectStatus:  http.StatusUnauthorized,
		},
		{
			name:         "a domain host is forbidden on the live probe",
			path:         "/api/v1/live",
			host:         "example.com:3858",
			expectStatus: http.StatusForbidden,
		},
		{
			name:         "a private host is forbidden on the ready probe",
			path:         "/api/v1/ready",
			host:         "10.0.0.1:3858",
			expectStatus: http.StatusForbidden,
		},
		{
			name:          "a domain host is forbidden with a valid bearer",
			path:          "/api/v1/status",
			host:          "example.com:3858",
			authorization: valid,
			expectStatus:  http.StatusForbidden,
		},
		{
			name:          "a private host is forbidden with a valid bearer",
			path:          "/api/v1/status",
			host:          "10.0.0.1:3858",
			authorization: valid,
			expectStatus:  http.StatusForbidden,
		},
		{
			name:          "a loopback origin is forbidden with a valid bearer",
			path:          "/api/v1/status",
			origin:        []string{"http://localhost:3858"},
			authorization: valid,
			expectStatus:  http.StatusForbidden,
		},
		{
			name:          "a null origin is forbidden with a valid bearer",
			path:          "/api/v1/status",
			origin:        []string{"null"},
			authorization: valid,
			expectStatus:  http.StatusForbidden,
		},
		{
			name:          "an empty origin is forbidden with a valid bearer",
			path:          "/api/v1/status",
			origin:        []string{""},
			authorization: valid,
			expectStatus:  http.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://"+address+tt.path, nil)
			require.NoError(t, err)

			req.Host = tt.host
			req.Header["Origin"] = tt.origin
			req.Header["Authorization"] = tt.authorization

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			resp.Body.Close()

			assert.Equal(t, tt.expectStatus, resp.StatusCode)
			assert.Equal(t, tt.expectStatus, <-requested)
		})
	}
}

func Test_Server_Start_WithoutToken(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPublisher := NewMockPublisher(ctrl)
	mockRegistry := NewMockRegistry(ctrl)
	mockControl := NewMockControl(ctrl)

	api := model.Service{ID: "id-api", Name: "api"}
	snapshot := &model.Snapshot{Services: map[string]*model.Service{"id-api": &api}}

	mockPublisher.EXPECT().Publish(gomock.Any()).Return(nil).AnyTimes()
	mockRegistry.EXPECT().Read(gomock.Any()).Do(readFrom(snapshot)).AnyTimes()

	log := slog.New(slog.DiscardHandler)

	s := NewServer(Options{Listen: "127.0.0.1:0"}, mockRegistry, mockControl, mockPublisher, model.Instance{}, log)
	s.Start(t.Context())

	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()

	defer s.Stop(ctx)

	address := s.Address(t.Context())

	tests := []struct {
		name         string
		before       func()
		method       string
		path         string
		expectStatus int
	}{
		{
			name:         "status",
			before:       func() {},
			method:       http.MethodGet,
			path:         "/api/v1/status",
			expectStatus: http.StatusOK,
		},
		{
			name:         "service list",
			before:       func() {},
			method:       http.MethodGet,
			path:         "/api/v1/services",
			expectStatus: http.StatusOK,
		},
		{
			name:         "one service",
			before:       func() {},
			method:       http.MethodGet,
			path:         "/api/v1/services/id-api",
			expectStatus: http.StatusOK,
		},
		{
			name: "start",
			before: func() {
				mockControl.EXPECT().Start("id-api").Return(services.Admission{Service: api, Action: contracts.ActionStart, Status: model.StatusStarting}, nil)
			},
			method:       http.MethodPost,
			path:         "/api/v1/services/id-api/start",
			expectStatus: http.StatusAccepted,
		},
		{
			name: "stop",
			before: func() {
				mockControl.EXPECT().Stop("id-api").Return(services.Admission{Service: api, Action: contracts.ActionStop, Status: model.StatusStopping}, nil)
			},
			method:       http.MethodPost,
			path:         "/api/v1/services/id-api/stop",
			expectStatus: http.StatusAccepted,
		},
		{
			name: "restart",
			before: func() {
				mockControl.EXPECT().Restart("id-api").Return(services.Admission{Service: api, Action: contracts.ActionRestart, Status: model.StatusRestarting}, nil)
			},
			method:       http.MethodPost,
			path:         "/api/v1/services/id-api/restart",
			expectStatus: http.StatusAccepted,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.before()

			req, err := http.NewRequestWithContext(t.Context(), tt.method, "http://"+address+tt.path, nil)
			require.NoError(t, err)

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			resp.Body.Close()

			assert.Equal(t, tt.expectStatus, resp.StatusCode)
		})
	}
}

func Test_Server_Shutdown_NilServer(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	log := slog.New(slog.DiscardHandler)

	s := NewServer(Options{}, nil, nil, nil, model.Instance{}, log)

	err := s.Stop(t.Context())

	require.NoError(t, err)
}

func Test_Server_Stop_PublishRejected(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLog := NewMockLogger(ctrl)
	mockPublisher := NewMockPublisher(ctrl)

	stopped := contracts.Message{Type: contracts.EventAPIStopped, Data: contracts.APIStopped{}}

	s := NewServer(Options{}, nil, nil, mockPublisher, model.Instance{}, mockLog)
	s.httpServer = &http.Server{}

	gomock.InOrder(
		mockLog.EXPECT().Debug("API server shutting down"),
		mockPublisher.EXPECT().Publish(stopped).Return(contracts.ErrBusClosed),
		mockLog.EXPECT().Error(fmt.Sprintf("Failed to publish %s", contracts.EventAPIStopped), "error", contracts.ErrBusClosed),
		mockLog.EXPECT().Debug("API server stopped"),
	)

	err := s.Stop(t.Context())

	require.NoError(t, err)
}

func Test_Server_Start_PortBusy(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPublisher := NewMockPublisher(ctrl)
	mockRegistry := NewMockRegistry(ctrl)

	log := slog.New(slog.DiscardHandler)

	occupied, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	defer occupied.Close()

	base := occupied.Addr().(*net.TCPAddr).Port

	for port := base + 1; port < base+PortRetries; port++ {
		listener, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
		if err == nil {
			t.Cleanup(func() { listener.Close() })
		}
	}

	s := NewServer(Options{Listen: occupied.Addr().String()}, mockRegistry, nil, mockPublisher, model.Instance{}, log)

	s.Start(t.Context())
	defer s.Stop(t.Context())

	assert.Nil(t, s.httpServer)
	assert.Empty(t, s.Address(t.Context()))
}

func Test_Server_Address_BeforeStart(t *testing.T) {
	s := NewServer(Options{}, nil, nil, nil, model.Instance{}, slog.New(slog.DiscardHandler))

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	assert.Empty(t, s.Address(ctx))
}
