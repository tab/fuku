package services

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"fuku/internal/contracts"
	"fuku/internal/model"
)

// queue is a Subscription over a plain channel
type queue chan contracts.Message

func (q queue) Messages() <-chan contracts.Message {
	return q
}

func Test_Runtime_Subscribe(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSubscriber := NewMockSubscriber(ctrl)

	log := slog.New(slog.DiscardHandler)

	options := contracts.SubscribeOptions{Name: "services", Required: true, Types: commandTypes}

	runtime := NewRuntime(RuntimeParams{Subscriber: mockSubscriber, Logger: log})

	tests := []struct {
		name     string
		before   func()
		expected error
	}{
		{
			name: "registers the required command subscription",
			before: func() {
				messages := make(queue)
				close(messages)

				mockSubscriber.EXPECT().Subscribe(gomock.Any(), options).Return(messages, nil)
			},
		},
		{
			name: "returns the subscribe error",
			before: func() {
				mockSubscriber.EXPECT().Subscribe(gomock.Any(), options).Return(nil, contracts.ErrBusClosed)
			},
			expected: contracts.ErrBusClosed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.before()

			err := runtime.Subscribe(t.Context())

			require.ErrorIs(t, err, tt.expected)
		})
	}
}

func Test_Runtime_Drain(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSubscriber := NewMockSubscriber(ctrl)

	log := slog.New(slog.DiscardHandler)

	tests := []struct {
		name   string
		before func() *Runtime
	}{
		{
			name: "a runtime that never subscribed has nothing to drain",
			before: func() *Runtime {
				return NewRuntime(RuntimeParams{Subscriber: mockSubscriber, Logger: log})
			},
		},
		{
			name: "a subscribed runtime drains once its closed queue ends the loop",
			before: func() *Runtime {
				runtime := NewRuntime(RuntimeParams{Subscriber: mockSubscriber, Logger: log})

				messages := make(queue)
				close(messages)

				mockSubscriber.EXPECT().Subscribe(gomock.Any(), gomock.Any()).Return(messages, nil)

				require.NoError(t, runtime.Subscribe(t.Context()))

				return runtime
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runtime := tt.before()

			err := runtime.Drain(t.Context())

			require.NoError(t, err)
		})
	}
}

func Test_Runtime_Publish(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPublisher := NewMockPublisher(ctrl)
	mockReporter := NewMockReporter(ctrl)

	log := slog.New(slog.DiscardHandler)

	msg := contracts.Message{Type: contracts.EventPhaseChanged, Data: contracts.PhaseChanged{Phase: model.PhaseRunning}}

	runtime := NewRuntime(RuntimeParams{Publisher: mockPublisher, Reporter: mockReporter, Logger: log})

	tests := []struct {
		name   string
		before func()
	}{
		{
			name: "an accepted publish reports nothing",
			before: func() {
				mockPublisher.EXPECT().Publish(msg).Return(nil)
			},
		},
		{
			name: "a rejected publish is a runtime failure",
			before: func() {
				mockPublisher.EXPECT().Publish(msg).Return(contracts.ErrBusOverloaded)
				mockReporter.EXPECT().Fail(contracts.ErrBusOverloaded)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.before()

			runtime.publish(msg)

			assert.True(t, ctrl.Satisfied())
		})
	}
}

func Test_Runtime_Handle(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLauncher := NewMockLauncher(ctrl)
	mockTracker := NewMockTracker(ctrl)
	mockPool := NewMockPool(ctrl)
	mockPublisher := NewMockPublisher(ctrl)
	mockProcess := NewMockProcess(ctrl)
	mockExited := NewMockProcess(ctrl)

	log := slog.New(slog.DiscardHandler)
	emptyStream := func() io.Reader { return strings.NewReader("") }

	svc := model.Service{ID: "test-id-api", Name: "api", Tier: "platform"}
	tiers := []model.Tier{{Name: "platform", Services: []*model.Service{&svc}}}
	running := make(chan struct{})
	exited := make(chan struct{})

	close(exited)

	isType := func(msgType contracts.MessageType) gomock.Matcher {
		return gomock.Cond(func(msg contracts.Message) bool {
			return msg.Type == msgType
		})
	}

	runtime := NewRuntime(RuntimeParams{
		Options:   Options{RetryAttempts: 1},
		Launcher:  mockLauncher,
		Tracker:   mockTracker,
		Pool:      mockPool,
		Guard:     NewGuard(mockTracker),
		Publisher: mockPublisher,
		Logger:    log,
	})

	mockProcess.EXPECT().PID().Return(42).AnyTimes()
	mockProcess.EXPECT().Service().Return(svc).AnyTimes()
	mockProcess.EXPECT().Stdout().DoAndReturn(emptyStream).AnyTimes()
	mockProcess.EXPECT().Stderr().DoAndReturn(emptyStream).AnyTimes()
	mockProcess.EXPECT().Done().Return(running).AnyTimes()
	mockExited.EXPECT().Service().Return(svc).AnyTimes()
	mockExited.EXPECT().Done().Return(exited).AnyTimes()

	tests := []struct {
		name   string
		before func() context.Context
		msg    contracts.Message
		held   bool
	}{
		{
			name: "ignores a command with a foreign payload",
			before: func() context.Context {
				ctx := runtime.begin(t.Context())
				runtime.guard.resolve(tiers)
				runtime.guard.dispatch(svc.ID)
				runtime.guard.release(svc.ID)

				return ctx
			},
			msg: contracts.Message{Type: contracts.CommandStartService, Data: "api"},
		},
		{
			name: "drops a command for a service whose token is held",
			before: func() context.Context {
				ctx := runtime.begin(t.Context())
				runtime.guard.resolve(tiers)
				runtime.guard.dispatch(svc.ID)

				return ctx
			},
			msg:  contracts.Message{Type: contracts.CommandRestartService, Data: svc},
			held: true,
		},
		{
			name: "drops a command nobody admitted",
			before: func() context.Context {
				ctx := runtime.begin(t.Context())
				runtime.guard.resolve(tiers)
				runtime.guard.dispatch(svc.ID)
				runtime.guard.release(svc.ID)

				return ctx
			},
			msg: contracts.Message{Type: contracts.CommandStartService, Data: svc},
		},
		{
			name: "stops the service on an admitted stop command and releases the token",
			before: func() context.Context {
				ctx := runtime.begin(t.Context())
				runtime.guard.resolve(tiers)
				runtime.guard.dispatch(svc.ID)
				runtime.guard.release(svc.ID)

				gomock.InOrder(
					mockTracker.EXPECT().Get(svc.ID).Return(mockProcess, true),
					mockTracker.EXPECT().Get(svc.ID).Return(mockExited, true),
				)

				_, err := runtime.guard.admit(svc.ID, contracts.ActionStop)
				require.NoError(t, err)

				mockTracker.EXPECT().Detach(svc.ID)
				mockExited.EXPECT().Terminate().Return(nil)
				mockTracker.EXPECT().Untrack(svc.ID, mockExited).Return(false)
				gomock.InOrder(
					mockPublisher.EXPECT().Publish(isType(contracts.EventServiceStopping)).Return(nil),
					mockPublisher.EXPECT().Publish(isType(contracts.EventServiceStopped)).Return(nil),
				)

				return ctx
			},
			msg: contracts.Message{Type: contracts.CommandStopService, Data: svc},
		},
		{
			name: "starts the service on a worker on an admitted start command",
			before: func() context.Context {
				ctx := runtime.begin(t.Context())
				runtime.guard.resolve(tiers)
				runtime.guard.dispatch(svc.ID)
				runtime.guard.release(svc.ID)

				mockTracker.EXPECT().Get(svc.ID).Return(nil, false).Times(2)

				_, err := runtime.guard.admit(svc.ID, contracts.ActionStart)
				require.NoError(t, err)

				gomock.InOrder(
					mockPool.EXPECT().Acquire(gomock.Any()).Return(nil),
					mockLauncher.EXPECT().Start(svc).Return(mockProcess, nil),
					mockPublisher.EXPECT().Publish(isType(contracts.EventServiceStarting)).Return(nil),
					mockPublisher.EXPECT().Publish(isType(contracts.EventServiceReady)).Return(nil),
					mockPool.EXPECT().Release(),
				)

				return ctx
			},
			msg: contracts.Message{Type: contracts.CommandStartService, Data: svc},
		},
		{
			name: "restarts the service on a worker on an admitted restart command",
			before: func() context.Context {
				ctx := runtime.begin(t.Context())
				runtime.guard.resolve(tiers)
				runtime.guard.dispatch(svc.ID)
				runtime.guard.release(svc.ID)

				mockTracker.EXPECT().Get(svc.ID).Return(nil, false).Times(2)

				_, err := runtime.guard.admit(svc.ID, contracts.ActionRestart)
				require.NoError(t, err)

				gomock.InOrder(
					mockPool.EXPECT().Acquire(gomock.Any()).Return(nil),
					mockPublisher.EXPECT().Publish(isType(contracts.EventServiceRestarting)).Return(nil),
					mockLauncher.EXPECT().Start(svc).Return(mockProcess, nil),
					mockPublisher.EXPECT().Publish(isType(contracts.EventServiceStarting)).Return(nil),
					mockPublisher.EXPECT().Publish(isType(contracts.EventServiceReady)).Return(nil),
					mockPool.EXPECT().Release(),
				)

				return ctx
			},
			msg: contracts.Message{Type: contracts.CommandRestartService, Data: svc},
		},
		{
			name: "a failed worker acquisition releases the token",
			before: func() context.Context {
				ctx := runtime.begin(t.Context())
				runtime.guard.resolve(tiers)
				runtime.guard.dispatch(svc.ID)
				runtime.guard.release(svc.ID)

				mockTracker.EXPECT().Get(svc.ID).Return(nil, false)

				_, err := runtime.guard.admit(svc.ID, contracts.ActionStart)
				require.NoError(t, err)

				mockPool.EXPECT().Acquire(gomock.Any()).Return(context.Canceled)

				return ctx
			},
			msg: contracts.Message{Type: contracts.CommandStartService, Data: svc},
		},
		{
			name: "restarts the changed service on a file change",
			before: func() context.Context {
				ctx := runtime.begin(t.Context())
				runtime.guard.resolve(tiers)
				runtime.guard.dispatch(svc.ID)
				runtime.guard.release(svc.ID)

				mockTracker.EXPECT().Get(svc.ID).Return(nil, false)
				gomock.InOrder(
					mockPool.EXPECT().Acquire(gomock.Any()).Return(nil),
					mockPublisher.EXPECT().Publish(isType(contracts.EventServiceRestarting)).Return(nil),
					mockLauncher.EXPECT().Start(svc).Return(mockProcess, nil),
					mockPublisher.EXPECT().Publish(isType(contracts.EventServiceStarting)).Return(nil),
					mockPublisher.EXPECT().Publish(isType(contracts.EventServiceReady)).Return(nil),
					mockPool.EXPECT().Release(),
				)

				return ctx
			},
			msg: contracts.Message{Type: contracts.EventWatchTriggered, Data: contracts.WatchTriggered{Service: svc, ChangedFiles: []string{"main.go"}}},
		},
		{
			name: "drops a file change for a service whose token is held",
			before: func() context.Context {
				ctx := runtime.begin(t.Context())
				runtime.guard.resolve(tiers)
				runtime.guard.dispatch(svc.ID)

				return ctx
			},
			msg:  contracts.Message{Type: contracts.EventWatchTriggered, Data: contracts.WatchTriggered{Service: svc}},
			held: true,
		},
		{
			name: "drops a file change for a service that was admitted for a command",
			before: func() context.Context {
				ctx := runtime.begin(t.Context())
				runtime.guard.resolve(tiers)
				runtime.guard.dispatch(svc.ID)
				runtime.guard.release(svc.ID)

				mockTracker.EXPECT().Get(svc.ID).Return(nil, false)

				_, err := runtime.guard.admit(svc.ID, contracts.ActionRestart)
				require.NoError(t, err)

				return ctx
			},
			msg:  contracts.Message{Type: contracts.EventWatchTriggered, Data: contracts.WatchTriggered{Service: svc}},
			held: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.before()

			runtime.handle(tt.msg)
			runtime.wg.Wait()

			assert.True(t, ctrl.Satisfied())
			assert.Equal(t, tt.held, runtime.guard.services[svc.ID].reserved)
		})
	}
}

func Test_Runtime_Handle_StopAll(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTracker := NewMockTracker(ctrl)

	log := slog.New(slog.DiscardHandler)

	tiers := []model.Tier{{Name: "platform", Services: []*model.Service{{ID: "test-id-api", Name: "api"}}}}

	runtime := NewRuntime(RuntimeParams{Guard: NewGuard(mockTracker), Logger: log})

	ctx := runtime.begin(t.Context())
	runtime.guard.resolve(tiers)

	runtime.handle(contracts.Message{Type: contracts.CommandStopAll})

	require.ErrorIs(t, context.Cause(ctx), errStopAll)
}

func Test_Runtime_Handle_WithoutRun(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTracker := NewMockTracker(ctrl)

	log := slog.New(slog.DiscardHandler)

	svc := model.Service{ID: "test-id-api", Name: "api"}
	tiers := []model.Tier{{Name: "platform", Services: []*model.Service{&svc}}}

	runtime := NewRuntime(RuntimeParams{Guard: NewGuard(mockTracker), Logger: log})

	runtime.begin(t.Context())
	runtime.guard.resolve(tiers)
	runtime.guard.dispatch(svc.ID)
	runtime.guard.release(svc.ID)

	mockTracker.EXPECT().Get(svc.ID).Return(nil, false)

	_, err := runtime.guard.admit(svc.ID, contracts.ActionStart)
	require.NoError(t, err)

	runtime.end()

	runtime.handle(contracts.Message{Type: contracts.CommandStartService, Data: svc})
	runtime.wg.Wait()

	assert.True(t, runtime.guard.reserve(svc.ID))
}
