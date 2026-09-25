package lifecycle

import (
	"context"
	"errors"
	"log/slog"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"
	"go.uber.org/mock/gomock"

	"fuku/internal/contracts"
)

func Test_Register(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLifecycle := NewMockLifecycle(ctrl)

	coordinator := NewCoordinator(nil, nil, nil, nil, Participants{}, nil)

	bothHooks := func(hook fx.Hook) bool {
		return hook.OnStart != nil && hook.OnStop != nil
	}

	mockLifecycle.EXPECT().Append(gomock.Cond(bothHooks))

	Register(mockLifecycle, coordinator)
}

func Test_Coordinator_Start(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockGuard := NewMockGuard(ctrl)
	mockFirst := NewMockConsumer(ctrl)
	mockSecond := NewMockConsumer(ctrl)
	mockLeading := NewMockProducer(ctrl)
	mockTrailing := NewMockProducer(ctrl)
	mockCommand := NewMockCommand(ctrl)
	mockTelemetry := NewMockTelemetry(ctrl)
	mockCloser := NewMockCloser(ctrl)
	mockShutdowner := NewMockShutdowner(ctrl)
	mockLog := NewMockLogger(ctrl)

	log := slog.New(slog.DiscardHandler)
	refused := errors.New("fuku is already running for this project")
	subscribeErr := errors.New("bus closed")
	startErr := errors.New("bind failed")
	stopErr := errors.New("stop failed")
	commandErr := errors.New("profile not found: nope")

	participants := Participants{
		Guard:     mockGuard,
		Consumers: []Consumer{mockFirst, mockSecond},
		Producers: []Producer{mockLeading, mockTrailing},
		Command:   mockCommand,
	}

	tests := []struct {
		name        string
		before      func(t *testing.T) *Coordinator
		expectedErr error
	}{
		{
			name: "guard, telemetry, consumers, producers and the command run in that order and the outcome stops the container",
			before: func(t *testing.T) *Coordinator {
				coordinator := NewCoordinator(NewArbiter(mockShutdowner), nil, nil, mockTelemetry, participants, log)
				runContext := func(ctx context.Context) bool { return ctx == coordinator.ctx }
				completed := make(chan struct{})
				stopped := func(...any) error {
					close(completed)

					return nil
				}

				gomock.InOrder(
					mockGuard.EXPECT().Check(gomock.Any()).Return(nil),
					mockTelemetry.EXPECT().Start(),
					mockFirst.EXPECT().Subscribe(gomock.Cond(runContext)).Return(nil),
					mockSecond.EXPECT().Subscribe(gomock.Cond(runContext)).Return(nil),
					mockLeading.EXPECT().Start(gomock.Cond(runContext)).Return(nil),
					mockTrailing.EXPECT().Start(gomock.Cond(runContext)).Return(nil),
					mockCommand.EXPECT().Run(gomock.Cond(runContext)).Return(2, nil),
					mockShutdowner.EXPECT().Shutdown(gomock.Len(1)).DoAndReturn(stopped),
				)

				t.Cleanup(func() { <-completed })

				return coordinator
			},
		},
		{
			name: "the command's error is recorded as the cause",
			before: func(t *testing.T) *Coordinator {
				arbiter := NewArbiter(mockShutdowner)
				coordinator := NewCoordinator(arbiter, nil, nil, mockTelemetry, participants, log)
				runContext := func(ctx context.Context) bool { return ctx == coordinator.ctx }
				completed := make(chan struct{})
				stopped := func(...any) error {
					close(completed)

					return nil
				}

				gomock.InOrder(
					mockGuard.EXPECT().Check(gomock.Any()).Return(nil),
					mockTelemetry.EXPECT().Start(),
					mockFirst.EXPECT().Subscribe(gomock.Cond(runContext)).Return(nil),
					mockSecond.EXPECT().Subscribe(gomock.Cond(runContext)).Return(nil),
					mockLeading.EXPECT().Start(gomock.Cond(runContext)).Return(nil),
					mockTrailing.EXPECT().Start(gomock.Cond(runContext)).Return(nil),
					mockCommand.EXPECT().Run(gomock.Cond(runContext)).Return(1, commandErr),
					mockShutdowner.EXPECT().Shutdown(gomock.Len(1)).DoAndReturn(stopped),
				)

				t.Cleanup(func() {
					<-completed

					code, cause := arbiter.outcome()

					assert.Equal(t, 1, code)
					assert.ErrorIs(t, cause, commandErr)
				})

				return coordinator
			},
		},
		{
			name: "a refused guard starts nothing and closes the bus",
			before: func(t *testing.T) *Coordinator {
				gomock.InOrder(
					mockGuard.EXPECT().Check(gomock.Any()).Return(refused),
					mockTelemetry.EXPECT().Stop(),
					mockCloser.EXPECT().Close(),
				)

				coordinator := NewCoordinator(NewArbiter(mockShutdowner), nil, mockCloser, mockTelemetry, participants, log)

				t.Cleanup(func() { assert.ErrorIs(t, coordinator.ctx.Err(), context.Canceled) })

				return coordinator
			},
			expectedErr: refused,
		},
		{
			name: "a consumer that cannot subscribe keeps every producer down, cancels the context and closes the bus",
			before: func(t *testing.T) *Coordinator {
				coordinator := NewCoordinator(NewArbiter(mockShutdowner), nil, mockCloser, mockTelemetry, participants, log)
				runContext := func(ctx context.Context) bool { return ctx == coordinator.ctx }

				gomock.InOrder(
					mockGuard.EXPECT().Check(gomock.Any()).Return(nil),
					mockTelemetry.EXPECT().Start(),
					mockFirst.EXPECT().Subscribe(gomock.Cond(runContext)).Return(nil),
					mockSecond.EXPECT().Subscribe(gomock.Cond(runContext)).Return(subscribeErr),
					mockTelemetry.EXPECT().Stop(),
					mockCloser.EXPECT().Close(),
				)

				t.Cleanup(func() { assert.ErrorIs(t, coordinator.ctx.Err(), context.Canceled) })

				return coordinator
			},
			expectedErr: subscribeErr,
		},
		{
			name: "a producer that cannot start unwinds the producers started before it and closes the bus",
			before: func(t *testing.T) *Coordinator {
				coordinator := NewCoordinator(NewArbiter(mockShutdowner), nil, mockCloser, mockTelemetry, participants, mockLog)
				runContext := func(ctx context.Context) bool { return ctx == coordinator.ctx }

				gomock.InOrder(
					mockGuard.EXPECT().Check(gomock.Any()).Return(nil),
					mockTelemetry.EXPECT().Start(),
					mockFirst.EXPECT().Subscribe(gomock.Cond(runContext)).Return(nil),
					mockSecond.EXPECT().Subscribe(gomock.Cond(runContext)).Return(nil),
					mockLeading.EXPECT().Start(gomock.Cond(runContext)).Return(nil),
					mockTrailing.EXPECT().Start(gomock.Cond(runContext)).Return(startErr),
					mockLeading.EXPECT().Stop(gomock.Any()).Return(stopErr),
					mockLog.EXPECT().Error("Failed to stop a producer while unwinding the start", "error", stopErr),
					mockTelemetry.EXPECT().Stop(),
					mockCloser.EXPECT().Close(),
				)

				t.Cleanup(func() { assert.ErrorIs(t, coordinator.ctx.Err(), context.Canceled) })

				return coordinator
			},
			expectedErr: startErr,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			coordinator := tt.before(t)

			err := coordinator.Start(t.Context())

			require.ErrorIs(t, err, tt.expectedErr)
		})
	}
}

func Test_Coordinator_Start_WithoutGuard(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCommand := NewMockCommand(ctrl)
	mockTelemetry := NewMockTelemetry(ctrl)
	mockShutdowner := NewMockShutdowner(ctrl)

	log := slog.New(slog.DiscardHandler)
	completed := make(chan struct{})
	coordinator := NewCoordinator(NewArbiter(mockShutdowner), nil, nil, mockTelemetry, Participants{Command: mockCommand}, log)
	runContext := func(ctx context.Context) bool { return ctx == coordinator.ctx }
	stopped := func(...any) error {
		close(completed)

		return nil
	}

	gomock.InOrder(
		mockTelemetry.EXPECT().Start(),
		mockCommand.EXPECT().Run(gomock.Cond(runContext)).Return(0, nil),
		mockShutdowner.EXPECT().Shutdown(gomock.Len(1)).DoAndReturn(stopped),
	)

	err := coordinator.Start(t.Context())

	require.NoError(t, err)

	<-completed
}

func Test_Coordinator_Stop(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockFirst := NewMockConsumer(ctrl)
	mockSecond := NewMockConsumer(ctrl)
	mockLeading := NewMockProducer(ctrl)
	mockTrailing := NewMockProducer(ctrl)
	mockCommand := NewMockCommand(ctrl)
	mockPublisher := NewMockPublisher(ctrl)
	mockTelemetry := NewMockTelemetry(ctrl)
	mockCloser := NewMockCloser(ctrl)
	mockShutdowner := NewMockShutdowner(ctrl)
	mockLog := NewMockLogger(ctrl)

	log := slog.New(slog.DiscardHandler)
	signal := contracts.Message{Type: contracts.EventSignalReceived, Data: contracts.SignalReceived{Name: "terminated"}}
	drainErr := errors.New("drain timed out")

	participants := Participants{
		Consumers: []Consumer{mockFirst, mockSecond},
		Producers: []Producer{mockLeading, mockTrailing},
		Command:   mockCommand,
	}

	mockShutdowner.EXPECT().Shutdown(gomock.Any()).Return(nil).AnyTimes()

	waitForCancel := func(ctx context.Context) (int, error) {
		<-ctx.Done()

		return 0, nil
	}
	waitForDeadline := func(ctx context.Context) error {
		<-ctx.Done()

		return ctx.Err()
	}

	started := func(t *testing.T, coordinator *Coordinator) *Coordinator {
		mockTelemetry.EXPECT().Start()
		mockFirst.EXPECT().Subscribe(gomock.Any()).Return(nil)
		mockSecond.EXPECT().Subscribe(gomock.Any()).Return(nil)
		mockLeading.EXPECT().Start(gomock.Any()).Return(nil)
		mockTrailing.EXPECT().Start(gomock.Any()).Return(nil)
		mockCommand.EXPECT().Run(gomock.Any()).DoAndReturn(waitForCancel)

		require.NoError(t, coordinator.Start(t.Context()))

		return coordinator
	}

	tests := []struct {
		name        string
		before      func(t *testing.T) (*Coordinator, context.Context)
		expectedErr error
	}{
		{
			name: "announces the signal, stops the producers newest first, drains the consumers twice, ends the command and stops telemetry",
			before: func(t *testing.T) (*Coordinator, context.Context) {
				arbiter := NewArbiter(mockShutdowner)
				arbiter.observe(syscall.SIGTERM)

				gomock.InOrder(
					mockLog.EXPECT().Info("Received signal terminated, shutting down services..."),
					mockPublisher.EXPECT().Publish(signal).Return(nil),
					mockTrailing.EXPECT().Stop(gomock.Any()).Return(nil),
					mockLeading.EXPECT().Stop(gomock.Any()).Return(nil),
					mockFirst.EXPECT().Drain(gomock.Any()).Return(nil),
					mockSecond.EXPECT().Drain(gomock.Any()).Return(nil),
					mockFirst.EXPECT().Drain(gomock.Any()).Return(nil),
					mockSecond.EXPECT().Drain(gomock.Any()).Return(nil),
					mockTelemetry.EXPECT().Stop(),
					mockCloser.EXPECT().Close(),
				)

				return started(t, NewCoordinator(arbiter, mockPublisher, mockCloser, mockTelemetry, participants, mockLog)), t.Context()
			},
		},
		{
			name: "a shutdown decided from inside announces no signal",
			before: func(t *testing.T) (*Coordinator, context.Context) {
				arbiter := NewArbiter(mockShutdowner)
				arbiter.decide(0, nil)
				arbiter.observe(syscall.SIGTERM)

				mockTrailing.EXPECT().Stop(gomock.Any()).Return(nil)
				mockLeading.EXPECT().Stop(gomock.Any()).Return(nil)
				mockFirst.EXPECT().Drain(gomock.Any()).Return(nil).Times(2)
				mockSecond.EXPECT().Drain(gomock.Any()).Return(nil).Times(2)
				mockTelemetry.EXPECT().Stop()
				mockCloser.EXPECT().Close()

				return started(t, NewCoordinator(arbiter, mockPublisher, mockCloser, mockTelemetry, participants, log)), t.Context()
			},
		},
		{
			name: "a failed drain is reported and the context still ends",
			before: func(t *testing.T) (*Coordinator, context.Context) {
				mockTrailing.EXPECT().Stop(gomock.Any()).Return(nil)
				mockLeading.EXPECT().Stop(gomock.Any()).Return(nil)
				mockFirst.EXPECT().Drain(gomock.Any()).Return(drainErr).Times(2)
				mockTelemetry.EXPECT().Stop()
				mockCloser.EXPECT().Close()

				return started(t, NewCoordinator(NewArbiter(mockShutdowner), mockPublisher, mockCloser, mockTelemetry, participants, log)), t.Context()
			},
			expectedErr: drainErr,
		},
		{
			name: "an expired stop deadline is reported while the command is still running",
			before: func(t *testing.T) (*Coordinator, context.Context) {
				ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
				t.Cleanup(cancel)

				mockTrailing.EXPECT().Stop(gomock.Any()).Return(nil)
				mockLeading.EXPECT().Stop(gomock.Any()).Return(nil)
				mockFirst.EXPECT().Drain(gomock.Any()).Return(nil).Times(2)
				mockSecond.EXPECT().Drain(gomock.Any()).DoAndReturn(waitForDeadline).Times(2)
				mockTelemetry.EXPECT().Stop()
				mockCloser.EXPECT().Close()

				return started(t, NewCoordinator(NewArbiter(mockShutdowner), mockPublisher, mockCloser, mockTelemetry, participants, log)), ctx
			},
			expectedErr: context.DeadlineExceeded,
		},
		{
			name: "a producer that overruns the stop deadline is reported and the consumers still drain",
			before: func(t *testing.T) (*Coordinator, context.Context) {
				ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
				t.Cleanup(cancel)

				gomock.InOrder(
					mockTrailing.EXPECT().Stop(gomock.Any()).DoAndReturn(waitForDeadline),
					mockLeading.EXPECT().Stop(gomock.Any()).Return(nil),
					mockFirst.EXPECT().Drain(gomock.Any()).Return(nil),
					mockSecond.EXPECT().Drain(gomock.Any()).Return(nil),
					mockFirst.EXPECT().Drain(gomock.Any()).Return(nil),
					mockSecond.EXPECT().Drain(gomock.Any()).Return(nil),
					mockTelemetry.EXPECT().Stop(),
					mockCloser.EXPECT().Close(),
				)

				return started(t, NewCoordinator(NewArbiter(mockShutdowner), mockPublisher, mockCloser, mockTelemetry, participants, log)), ctx
			},
			expectedErr: context.DeadlineExceeded,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			coordinator, ctx := tt.before(t)

			err := coordinator.Stop(ctx)

			require.ErrorIs(t, err, tt.expectedErr)
			assert.ErrorIs(t, coordinator.ctx.Err(), context.Canceled)
		})
	}
}

func Test_Coordinator_Stop_WaitsForTheCommand(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCommand := NewMockCommand(ctrl)
	mockTelemetry := NewMockTelemetry(ctrl)
	mockCloser := NewMockCloser(ctrl)
	mockShutdowner := NewMockShutdowner(ctrl)

	log := slog.New(slog.DiscardHandler)
	coordinator := NewCoordinator(NewArbiter(mockShutdowner), nil, mockCloser, mockTelemetry, Participants{Command: mockCommand}, log)
	runContext := func(ctx context.Context) bool { return ctx == coordinator.ctx }
	finished := make(chan struct{})
	waitForCancel := func(ctx context.Context) (int, error) {
		<-ctx.Done()
		close(finished)

		return 0, nil
	}

	mockTelemetry.EXPECT().Start()
	mockCommand.EXPECT().Run(gomock.Cond(runContext)).DoAndReturn(waitForCancel)
	mockTelemetry.EXPECT().Stop()
	mockCloser.EXPECT().Close()
	mockShutdowner.EXPECT().Shutdown(gomock.Any()).Return(nil)

	require.NoError(t, coordinator.Start(t.Context()))

	err := coordinator.Stop(t.Context())

	require.NoError(t, err)

	select {
	case <-finished:
	default:
		t.Fatal("Stop returned before the command finished")
	}
}

func Test_Coordinator_Stop_ClosesTheBusLast(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConsumer := NewMockConsumer(ctrl)
	mockProducer := NewMockProducer(ctrl)
	mockCommand := NewMockCommand(ctrl)
	mockShutdowner := NewMockShutdowner(ctrl)
	mockTelemetry := NewMockTelemetry(ctrl)
	mockCloser := NewMockCloser(ctrl)

	log := slog.New(slog.DiscardHandler)
	participants := Participants{Consumers: []Consumer{mockConsumer}, Producers: []Producer{mockProducer}, Command: mockCommand}
	coordinator := NewCoordinator(NewArbiter(mockShutdowner), nil, mockCloser, mockTelemetry, participants, log)
	runContext := func(ctx context.Context) bool { return ctx == coordinator.ctx }
	waitForCancel := func(ctx context.Context) (int, error) {
		<-ctx.Done()

		return 0, nil
	}

	mockTelemetry.EXPECT().Start()
	mockConsumer.EXPECT().Subscribe(gomock.Cond(runContext)).Return(nil)
	mockProducer.EXPECT().Start(gomock.Cond(runContext)).Return(nil)
	mockCommand.EXPECT().Run(gomock.Cond(runContext)).DoAndReturn(waitForCancel)
	gomock.InOrder(
		mockProducer.EXPECT().Stop(gomock.Any()).Return(nil),
		mockConsumer.EXPECT().Drain(gomock.Any()).Return(nil).Times(2),
		mockShutdowner.EXPECT().Shutdown(gomock.Len(1)).Return(nil),
		mockTelemetry.EXPECT().Stop(),
		mockCloser.EXPECT().Close(),
	)

	require.NoError(t, coordinator.Start(t.Context()))

	err := coordinator.Stop(t.Context())

	require.NoError(t, err)
}
