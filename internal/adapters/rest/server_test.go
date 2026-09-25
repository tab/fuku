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
}
