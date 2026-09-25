package services

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"fuku/internal/contracts"
	"fuku/internal/model"
)

func Test_Runtime_StartWithRetry(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLauncher := NewMockLauncher(ctrl)
	mockTracker := NewMockTracker(ctrl)
	mockReadiness := NewMockReadiness(ctrl)
	mockPublisher := NewMockPublisher(ctrl)
	mockProcess := NewMockProcess(ctrl)

	log := slog.New(slog.DiscardHandler)
	emptyStream := func() io.Reader { return strings.NewReader("") }

	http := &model.Readiness{Type: model.ReadinessHTTP, URL: "http://localhost:8080/health"}
	svc := model.Service{ID: "test-id-api", Name: "api", Tier: "platform", Readiness: http}
	launchErr := errors.New("launch failed")
	readinessErr := errors.New("timed out")
	running := make(chan struct{})

	isStarting := func(attempt int) gomock.Matcher {
		return gomock.Cond(func(msg contracts.Message) bool {
			data, ok := msg.Data.(contracts.ServiceStarting)

			return msg.Type == contracts.EventServiceStarting && ok && reflect.DeepEqual(data.Service, svc) && data.Attempt == attempt
		})
	}
	isReady := gomock.Cond(func(msg contracts.Message) bool {
		data, ok := msg.Data.(contracts.ServiceReady)

		return msg.Type == contracts.EventServiceReady && ok && reflect.DeepEqual(data.Service, svc)
	})
	isFailed := func(cause error) gomock.Matcher {
		return gomock.Cond(func(msg contracts.Message) bool {
			data, ok := msg.Data.(contracts.ServiceFailed)

			return msg.Type == contracts.EventServiceFailed && ok && reflect.DeepEqual(data.Service, svc) && data.Tier == "platform" &&
				errors.Is(data.Error, contracts.ErrMaxRetriesExceeded) && errors.Is(data.Error, cause)
		})
	}
	isStopped := gomock.Cond(func(msg contracts.Message) bool {
		data, ok := msg.Data.(contracts.ServiceStopped)

		return msg.Type == contracts.EventServiceStopped && ok && reflect.DeepEqual(data.Service, svc) && data.Tier == "platform"
	})

	runtime := NewRuntime(RuntimeParams{
		Options:   Options{RetryAttempts: 2},
		Launcher:  mockLauncher,
		Tracker:   mockTracker,
		Readiness: mockReadiness,
		Publisher: mockPublisher,
		Logger:    log,
	})

	mockReadiness.EXPECT().ProbePort(*http).Return(model.Port{Address: "localhost:8080"}).AnyTimes()
	mockProcess.EXPECT().PID().Return(42).AnyTimes()
	mockProcess.EXPECT().Stdout().DoAndReturn(emptyStream).AnyTimes()
	mockProcess.EXPECT().Stderr().DoAndReturn(emptyStream).AnyTimes()
	mockProcess.EXPECT().Done().Return(running).AnyTimes()

	tests := []struct {
		name     string
		before   func() context.Context
		expected error
	}{
		{
			name: "a ready first attempt needs no retry",
			before: func() context.Context {
				mockLauncher.EXPECT().Start(svc).Return(mockProcess, nil)
				mockReadiness.EXPECT().Check(gomock.Any(), *http, mockProcess).Return(nil)
				gomock.InOrder(
					mockPublisher.EXPECT().Publish(isStarting(1)).Return(nil),
					mockPublisher.EXPECT().Publish(isReady).Return(nil),
				)

				return t.Context()
			},
		},
		{
			name: "a child that never became ready is cleaned up before the retry keeps the same service ID",
			before: func() context.Context {
				runtime.options.RetryBackoff = 0

				gomock.InOrder(
					mockLauncher.EXPECT().Start(svc).Return(mockProcess, nil),
					mockPublisher.EXPECT().Publish(isStarting(1)).Return(nil),
					mockReadiness.EXPECT().Check(gomock.Any(), *http, mockProcess).Return(readinessErr),
					mockProcess.EXPECT().Terminate().Return(nil),
					mockTracker.EXPECT().Untrack(svc.ID, mockProcess).Return(true),
					mockLauncher.EXPECT().Start(svc).Return(mockProcess, nil),
					mockPublisher.EXPECT().Publish(isStarting(2)).Return(nil),
					mockReadiness.EXPECT().Check(gomock.Any(), *http, mockProcess).Return(nil),
					mockPublisher.EXPECT().Publish(isReady).Return(nil),
				)

				return t.Context()
			},
		},
		{
			name: "the failure is published once the attempts run out",
			before: func() context.Context {
				runtime.options.RetryBackoff = 0

				mockLauncher.EXPECT().Start(svc).Return(nil, launchErr).Times(2)
				mockPublisher.EXPECT().Publish(isFailed(launchErr)).Return(nil)

				return t.Context()
			},
			expected: contracts.ErrMaxRetriesExceeded,
		},
		{
			name: "a run cancelled by a failed attempt retries nothing and ends the service as stopped",
			before: func() context.Context {
				ctx, cancel := context.WithCancel(t.Context())
				cancelling := func(model.Service) (contracts.Process, error) {
					cancel()

					return nil, launchErr
				}

				runtime.options.RetryBackoff = time.Hour

				mockLauncher.EXPECT().Start(svc).DoAndReturn(cancelling)
				mockPublisher.EXPECT().Publish(isStopped).Return(nil)

				return ctx
			},
			expected: context.Canceled,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := tt.before()

			err := runtime.startWithRetry(ctx, svc)

			require.ErrorIs(t, err, tt.expected)
			assert.True(t, ctrl.Satisfied())
		})
	}
}

func Test_Runtime_StartWithRetry_CancelledDuringBackoff(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLauncher := NewMockLauncher(ctrl)
	mockLog := NewMockLogger(ctrl)
	mockPublisher := NewMockPublisher(ctrl)

	svc := model.Service{ID: "test-id-api", Name: "api", Tier: "platform"}
	launchErr := errors.New("launch failed")
	stopped := contracts.Message{
		Type: contracts.EventServiceStopped,
		Data: contracts.ServiceStopped{ServiceEvent: contracts.ServiceEvent{Service: svc, Tier: "platform"}},
	}

	ctx, cancel := context.WithCancel(t.Context())
	cancelling := func(string, ...any) {
		cancel()
	}

	runtime := NewRuntime(RuntimeParams{
		Options:   Options{RetryAttempts: 2, RetryBackoff: time.Hour},
		Launcher:  mockLauncher,
		Publisher: mockPublisher,
		Logger:    mockLog,
	})

	gomock.InOrder(
		mockLauncher.EXPECT().Start(svc).Return(nil, launchErr),
		mockLog.EXPECT().Info("Retrying service 'api' (attempt 2/2)").Do(cancelling),
		mockPublisher.EXPECT().Publish(stopped).Return(nil),
	)

	err := runtime.startWithRetry(ctx, svc)

	require.ErrorIs(t, err, context.Canceled)
}
