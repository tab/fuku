package services

import (
	"context"
	"errors"
	"log/slog"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"fuku/internal/contracts"
	"fuku/internal/model"
)

func Test_Runtime_Attempt(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLauncher := NewMockLauncher(ctrl)
	mockTracker := NewMockTracker(ctrl)
	mockReadiness := NewMockReadiness(ctrl)
	mockPublisher := NewMockPublisher(ctrl)
	mockProcess := NewMockProcess(ctrl)

	log := slog.New(slog.DiscardHandler)

	svc := model.Service{ID: "test-id-api", Name: "api"}
	http := &model.Readiness{Type: model.ReadinessHTTP, URL: "http://localhost:8080/health"}
	plain := model.Service{ID: svc.ID, Name: "api", Tier: "platform"}
	probed := model.Service{ID: svc.ID, Name: "api", Tier: "platform", Readiness: http}
	launchErr := errors.New("launch failed")
	readinessErr := errors.New("timed out")

	isStarting := func(service model.Service, attempt int) gomock.Matcher {
		return gomock.Cond(func(msg contracts.Message) bool {
			data, ok := msg.Data.(contracts.ServiceStarting)

			return msg.Type == contracts.EventServiceStarting && ok && reflect.DeepEqual(data.Service, service) && data.Tier == "platform" && data.PID == 42 && data.Attempt == attempt
		})
	}
	isReady := func(service model.Service) gomock.Matcher {
		return gomock.Cond(func(msg contracts.Message) bool {
			data, ok := msg.Data.(contracts.ServiceReady)

			return msg.Type == contracts.EventServiceReady && ok && reflect.DeepEqual(data.Service, service) && data.Tier == "platform" && data.PID == 42
		})
	}

	runtime := NewRuntime(RuntimeParams{
		Launcher:  mockLauncher,
		Tracker:   mockTracker,
		Readiness: mockReadiness,
		Publisher: mockPublisher,
		Logger:    log,
	})

	tests := []struct {
		name     string
		before   func() context.Context
		service  model.Service
		attempt  int
		expected contracts.Process
		err      error
	}{
		{
			name: "a cancelled run launches nothing",
			before: func() context.Context {
				ctx, cancel := context.WithCancel(t.Context())
				cancel()

				return ctx
			},
			service: plain,
			attempt: 1,
			err:     context.Canceled,
		},
		{
			name: "a busy port fails before any launch",
			before: func() context.Context {
				mockReadiness.EXPECT().ProbePort(*http).Return(model.Port{Address: "localhost:8080", InUse: true})

				return t.Context()
			},
			service: probed,
			attempt: 1,
			err:     contracts.ErrPortAlreadyInUse,
		},
		{
			name: "a failed launch returns its error",
			before: func() context.Context {
				mockLauncher.EXPECT().Start(plain).Return(nil, launchErr)

				return t.Context()
			},
			service: plain,
			attempt: 1,
			err:     launchErr,
		},
		{
			name: "a service without readiness is ready once started and publishes the real attempt",
			before: func() context.Context {
				mockLauncher.EXPECT().Start(plain).Return(mockProcess, nil)
				mockProcess.EXPECT().PID().Return(42).Times(2)
				gomock.InOrder(
					mockPublisher.EXPECT().Publish(isStarting(plain, 3)).Return(nil),
					mockPublisher.EXPECT().Publish(isReady(plain)).Return(nil),
				)

				return t.Context()
			},
			service:  plain,
			attempt:  3,
			expected: mockProcess,
		},
		{
			name: "a run cancelled after the port probe launches nothing",
			before: func() context.Context {
				ctx, cancel := context.WithCancel(t.Context())
				cancelling := func(model.Readiness) model.Port {
					cancel()

					return model.Port{Address: "localhost:8080"}
				}

				mockReadiness.EXPECT().ProbePort(*http).DoAndReturn(cancelling)

				return ctx
			},
			service: probed,
			attempt: 1,
			err:     context.Canceled,
		},
		{
			name: "a ready check passes the started process",
			before: func() context.Context {
				mockReadiness.EXPECT().ProbePort(*http).Return(model.Port{Address: "localhost:8080"})
				mockLauncher.EXPECT().Start(probed).Return(mockProcess, nil)
				mockProcess.EXPECT().PID().Return(42).Times(2)
				mockReadiness.EXPECT().Check(gomock.Any(), *http, mockProcess).Return(nil)
				gomock.InOrder(
					mockPublisher.EXPECT().Publish(isStarting(probed, 1)).Return(nil),
					mockPublisher.EXPECT().Publish(isReady(probed)).Return(nil),
				)

				return t.Context()
			},
			service:  probed,
			attempt:  1,
			expected: mockProcess,
		},
		{
			name: "a failed readiness check terminates and untracks the child",
			before: func() context.Context {
				mockReadiness.EXPECT().ProbePort(*http).Return(model.Port{Address: "localhost:8080"})
				mockLauncher.EXPECT().Start(probed).Return(mockProcess, nil)
				mockProcess.EXPECT().PID().Return(42)
				mockPublisher.EXPECT().Publish(isStarting(probed, 1)).Return(nil)
				mockReadiness.EXPECT().Check(gomock.Any(), *http, mockProcess).Return(readinessErr)
				gomock.InOrder(
					mockProcess.EXPECT().Terminate().Return(nil),
					mockTracker.EXPECT().Untrack(svc.ID, mockProcess).Return(true),
				)

				return t.Context()
			},
			service: probed,
			attempt: 1,
			err:     readinessErr,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := tt.before()

			proc, err := runtime.attempt(ctx, tt.service, tt.attempt)

			require.ErrorIs(t, err, tt.err)
			assert.Equal(t, tt.expected, proc)
		})
	}
}

func Test_Runtime_Launch(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLauncher := NewMockLauncher(ctrl)
	mockProcess := NewMockProcess(ctrl)

	log := slog.New(slog.DiscardHandler)

	svc := model.Service{ID: "test-id-api", Name: "api", Tier: "platform"}

	var underLock bool

	runtime := NewRuntime(RuntimeParams{Launcher: mockLauncher, Logger: log})

	recordTheLock := func(model.Service) {
		underLock = holdsRunLock(runtime)
	}

	tests := []struct {
		name      string
		before    func() context.Context
		expected  contracts.Process
		err       error
		underLock bool
	}{
		{
			name: "starts the child under the run lock",
			before: func() context.Context {
				mockLauncher.EXPECT().Start(svc).Do(recordTheLock).Return(mockProcess, nil)

				return t.Context()
			},
			expected:  mockProcess,
			underLock: true,
		},
		{
			name: "a cancelled run starts nothing",
			before: func() context.Context {
				ctx, cancel := context.WithCancel(t.Context())
				cancel()

				return ctx
			},
			err: context.Canceled,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			underLock = false
			ctx := tt.before()

			proc, err := runtime.launch(ctx, svc)

			require.ErrorIs(t, err, tt.err)
			assert.Equal(t, tt.expected, proc)
			assert.Equal(t, tt.underLock, underLock)
		})
	}
}

func Test_Runtime_WaitForReady(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockReadiness := NewMockReadiness(ctrl)
	mockProcess := NewMockProcess(ctrl)

	log := slog.New(slog.DiscardHandler)

	http := &model.Readiness{Type: model.ReadinessHTTP, URL: "http://localhost:8080/health"}
	logReadiness := &model.Readiness{Type: model.ReadinessLog, Pattern: "ready"}
	checkErr := errors.New("timed out")

	runtime := NewRuntime(RuntimeParams{Readiness: mockReadiness, Logger: log})

	tests := []struct {
		name      string
		before    func()
		readiness *model.Readiness
		expected  error
	}{
		{
			name:   "no readiness is ready at once",
			before: func() {},
		},
		{
			name: "an HTTP check runs against the process",
			before: func() {
				mockReadiness.EXPECT().Check(gomock.Any(), *http, mockProcess).Return(nil)
			},
			readiness: http,
		},
		{
			name: "a log check runs against the process",
			before: func() {
				mockReadiness.EXPECT().Check(gomock.Any(), *logReadiness, mockProcess).Return(nil)
			},
			readiness: logReadiness,
		},
		{
			name: "a failed check is named",
			before: func() {
				mockReadiness.EXPECT().Check(gomock.Any(), *http, mockProcess).Return(checkErr)
			},
			readiness: http,
			expected:  checkErr,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.before()

			err := runtime.waitForReady(t.Context(), tt.readiness, mockProcess)

			require.ErrorIs(t, err, tt.expected)
		})
	}
}

func Test_Runtime_Stop(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTracker := NewMockTracker(ctrl)
	mockPublisher := NewMockPublisher(ctrl)
	mockProcess := NewMockProcess(ctrl)

	log := slog.New(slog.DiscardHandler)

	svc := model.Service{ID: "test-id-api", Name: "api", Tier: "platform"}
	event := contracts.ServiceEvent{Service: svc, Tier: "platform"}

	runtime := NewRuntime(RuntimeParams{Tracker: mockTracker, Publisher: mockPublisher, Logger: log})

	tests := []struct {
		name   string
		before func()
	}{
		{
			name: "a service without a child is left alone",
			before: func() {
				mockTracker.EXPECT().Get(svc.ID).Return(nil, false)
			},
		},
		{
			name: "a tracked child is detached, terminated and untracked between the stopping and stopped events",
			before: func() {
				done := make(chan struct{})
				terminate := func() error {
					close(done)

					return nil
				}

				mockTracker.EXPECT().Get(svc.ID).Return(mockProcess, true)
				mockProcess.EXPECT().Service().Return(svc).Times(2)
				mockProcess.EXPECT().Done().Return(done)
				gomock.InOrder(
					mockPublisher.EXPECT().Publish(contracts.Message{Type: contracts.EventServiceStopping, Data: contracts.ServiceStopping{ServiceEvent: event}}).Return(nil),
					mockTracker.EXPECT().Detach(svc.ID),
					mockProcess.EXPECT().Terminate().DoAndReturn(terminate),
					mockTracker.EXPECT().Untrack(svc.ID, mockProcess).Return(false),
					mockPublisher.EXPECT().Publish(contracts.Message{Type: contracts.EventServiceStopped, Data: contracts.ServiceStopped{ServiceEvent: event}}).Return(nil),
				)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.before()

			runtime.stop(svc.ID)

			assert.True(t, ctrl.Satisfied())
		})
	}
}

func Test_Runtime_Restart(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLauncher := NewMockLauncher(ctrl)
	mockTracker := NewMockTracker(ctrl)
	mockPublisher := NewMockPublisher(ctrl)
	mockOld := NewMockProcess(ctrl)
	mockNew := NewMockProcess(ctrl)

	log := slog.New(slog.DiscardHandler)

	svc := model.Service{ID: "test-id-api", Name: "api", Tier: "platform"}
	event := contracts.ServiceEvent{Service: svc, Tier: "platform"}
	launchErr := errors.New("launch failed")
	running := make(chan struct{})

	isRestarting := gomock.Cond(func(msg contracts.Message) bool {
		return msg.Type == contracts.EventServiceRestarting && reflect.DeepEqual(msg.Data, contracts.ServiceRestarting{ServiceEvent: event})
	})
	isStarting := gomock.Cond(func(msg contracts.Message) bool {
		data, ok := msg.Data.(contracts.ServiceStarting)

		return msg.Type == contracts.EventServiceStarting && ok && reflect.DeepEqual(data.Service, svc) && data.Attempt == 1
	})
	isReady := gomock.Cond(func(msg contracts.Message) bool {
		return msg.Type == contracts.EventServiceReady
	})
	isFailed := gomock.Cond(func(msg contracts.Message) bool {
		data, ok := msg.Data.(contracts.ServiceFailed)

		return msg.Type == contracts.EventServiceFailed && ok && reflect.DeepEqual(data.Service, svc) && errors.Is(data.Error, launchErr)
	})
	isStopped := gomock.Cond(func(msg contracts.Message) bool {
		return msg.Type == contracts.EventServiceStopped && reflect.DeepEqual(msg.Data, contracts.ServiceStopped{ServiceEvent: event})
	})

	runtime := NewRuntime(RuntimeParams{Launcher: mockLauncher, Tracker: mockTracker, Publisher: mockPublisher, Logger: log})

	mockNew.EXPECT().PID().Return(43).AnyTimes()
	mockNew.EXPECT().Done().Return(running).AnyTimes()

	tests := []struct {
		name   string
		before func() context.Context
	}{
		{
			name: "a stopped service is launched again under the same ID",
			before: func() context.Context {
				mockTracker.EXPECT().Get(svc.ID).Return(nil, false)
				mockLauncher.EXPECT().Start(svc).Return(mockNew, nil)
				gomock.InOrder(
					mockPublisher.EXPECT().Publish(isRestarting).Return(nil),
					mockPublisher.EXPECT().Publish(isStarting).Return(nil),
					mockPublisher.EXPECT().Publish(isReady).Return(nil),
				)

				return t.Context()
			},
		},
		{
			name: "a running service is stopped before the launch",
			before: func() context.Context {
				done := make(chan struct{})
				terminate := func() error {
					close(done)

					return nil
				}

				mockTracker.EXPECT().Get(svc.ID).Return(mockOld, true)
				mockOld.EXPECT().Service().Return(svc)
				mockOld.EXPECT().Done().Return(done)
				gomock.InOrder(
					mockPublisher.EXPECT().Publish(isRestarting).Return(nil),
					mockTracker.EXPECT().Detach(svc.ID),
					mockOld.EXPECT().Terminate().DoAndReturn(terminate),
					mockTracker.EXPECT().Untrack(svc.ID, mockOld).Return(false),
					mockLauncher.EXPECT().Start(svc).Return(mockNew, nil),
					mockPublisher.EXPECT().Publish(isStarting).Return(nil),
					mockPublisher.EXPECT().Publish(isReady).Return(nil),
				)

				return t.Context()
			},
		},
		{
			name: "a failed launch is published as a failure",
			before: func() context.Context {
				mockTracker.EXPECT().Get(svc.ID).Return(nil, false)
				mockLauncher.EXPECT().Start(svc).Return(nil, launchErr)
				gomock.InOrder(
					mockPublisher.EXPECT().Publish(isRestarting).Return(nil),
					mockPublisher.EXPECT().Publish(isFailed).Return(nil),
				)

				return t.Context()
			},
		},
		{
			name: "a restart the end of the run cuts short ends the service as stopped, not failed",
			before: func() context.Context {
				ctx, cancel := context.WithCancel(t.Context())
				cancel()

				mockTracker.EXPECT().Get(svc.ID).Return(nil, false)
				gomock.InOrder(
					mockPublisher.EXPECT().Publish(isRestarting).Return(nil),
					mockPublisher.EXPECT().Publish(isStopped).Return(nil),
				)

				return ctx
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := tt.before()

			runtime.restart(ctx, svc)

			assert.True(t, ctrl.Satisfied())
		})
	}
}

func Test_Runtime_Start(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLauncher := NewMockLauncher(ctrl)
	mockTracker := NewMockTracker(ctrl)
	mockPublisher := NewMockPublisher(ctrl)
	mockProcess := NewMockProcess(ctrl)
	mockExited := NewMockProcess(ctrl)

	log := slog.New(slog.DiscardHandler)

	svc := model.Service{ID: "test-id-api", Name: "api", Tier: "platform"}
	running := make(chan struct{})
	exited := make(chan struct{})

	close(exited)

	runtime := NewRuntime(RuntimeParams{Options: Options{RetryAttempts: 1}, Launcher: mockLauncher, Tracker: mockTracker, Publisher: mockPublisher, Logger: log})

	mockProcess.EXPECT().PID().Return(42).AnyTimes()
	mockProcess.EXPECT().Done().Return(running).AnyTimes()
	mockExited.EXPECT().Service().Return(svc).AnyTimes()
	mockExited.EXPECT().Done().Return(exited).AnyTimes()

	tests := []struct {
		name   string
		before func()
	}{
		{
			name: "a service whose child exited but is still tracked is reaped and started",
			before: func() {
				mockTracker.EXPECT().Get(svc.ID).Return(mockExited, true)
				gomock.InOrder(
					mockTracker.EXPECT().Detach(svc.ID),
					mockExited.EXPECT().Terminate().Return(nil),
					mockTracker.EXPECT().Untrack(svc.ID, mockExited).Return(false),
					mockLauncher.EXPECT().Start(svc).Return(mockProcess, nil),
				)
				mockPublisher.EXPECT().Publish(gomock.Any()).Return(nil).Times(2)
			},
		},
		{
			name: "a service without a child is started",
			before: func() {
				mockTracker.EXPECT().Get(svc.ID).Return(nil, false)
				mockLauncher.EXPECT().Start(svc).Return(mockProcess, nil)
				mockPublisher.EXPECT().Publish(gomock.Any()).Return(nil).Times(2)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.before()

			runtime.start(t.Context(), svc)

			assert.True(t, ctrl.Satisfied())
		})
	}
}

func Test_Runtime_WatchForExit(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTracker := NewMockTracker(ctrl)
	mockPublisher := NewMockPublisher(ctrl)
	mockProcess := NewMockProcess(ctrl)

	log := slog.New(slog.DiscardHandler)

	svc := model.Service{ID: "test-id-api", Name: "api", Tier: "platform", Watch: &model.Watch{Include: []string{"**/*.go"}}}
	worker := model.Service{ID: "test-id-worker", Name: "worker", Tier: "platform"}
	exited := make(chan struct{})

	close(exited)

	runtime := NewRuntime(RuntimeParams{Tracker: mockTracker, Publisher: mockPublisher, Logger: log})

	mockProcess.EXPECT().Done().Return(exited).AnyTimes()

	tests := []struct {
		name   string
		before func() chan struct{}
	}{
		{
			name: "an expected exit publishes nothing",
			before: func() chan struct{} {
				observed := make(chan struct{})
				untracked := func(string, contracts.Process) bool {
					close(observed)

					return false
				}

				mockProcess.EXPECT().Service().Return(svc)
				mockTracker.EXPECT().Untrack(svc.ID, mockProcess).DoAndReturn(untracked)

				return observed
			},
		},
		{
			name: "an unexpected exit of a watched service is a failure",
			before: func() chan struct{} {
				observed := make(chan struct{})
				published := func(contracts.Message) error {
					close(observed)

					return nil
				}

				mockProcess.EXPECT().Service().Return(svc)
				mockTracker.EXPECT().Untrack(svc.ID, mockProcess).Return(true)
				mockPublisher.EXPECT().Publish(contracts.Message{
					Type: contracts.EventServiceFailed,
					Data: contracts.ServiceFailed{ServiceEvent: contracts.ServiceEvent{Service: svc, Tier: "platform"}, Error: contracts.ErrUnexpectedExit},
				}).DoAndReturn(published)

				return observed
			},
		},
		{
			name: "an unexpected exit of an unwatched service is an unexpected stop",
			before: func() chan struct{} {
				observed := make(chan struct{})
				published := func(contracts.Message) error {
					close(observed)

					return nil
				}

				mockProcess.EXPECT().Service().Return(worker)
				mockTracker.EXPECT().Untrack(worker.ID, mockProcess).Return(true)
				mockPublisher.EXPECT().Publish(contracts.Message{
					Type: contracts.EventServiceStopped,
					Data: contracts.ServiceStopped{ServiceEvent: contracts.ServiceEvent{Service: worker, Tier: "platform"}, Unexpected: true},
				}).DoAndReturn(published)

				return observed
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			observed := tt.before()

			runtime.watchForExit(mockProcess)

			<-observed
			assert.True(t, ctrl.Satisfied())
		})
	}
}

func Test_Runtime_ProbePort(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockReadiness := NewMockReadiness(ctrl)

	log := slog.New(slog.DiscardHandler)

	tcp := &model.Readiness{Type: model.ReadinessTCP, Address: "localhost:5432"}

	runtime := NewRuntime(RuntimeParams{Readiness: mockReadiness, Logger: log})

	tests := []struct {
		name      string
		before    func()
		readiness *model.Readiness
		expected  error
	}{
		{
			name:   "a service without readiness is not probed",
			before: func() {},
		},
		{
			name: "a free address passes",
			before: func() {
				mockReadiness.EXPECT().ProbePort(*tcp).Return(model.Port{Address: "localhost:5432"})
			},
			readiness: tcp,
		},
		{
			name: "an address in use fails the attempt",
			before: func() {
				mockReadiness.EXPECT().ProbePort(*tcp).Return(model.Port{Address: "localhost:5432", InUse: true})
			},
			readiness: tcp,
			expected:  contracts.ErrPortAlreadyInUse,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.before()

			err := runtime.probePort("api", tt.readiness)

			require.ErrorIs(t, err, tt.expected)
		})
	}
}
