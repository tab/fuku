package services

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"fuku/internal/contracts"
	"fuku/internal/model"
)

func Test_NewRuntime(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockProfiles := NewMockProfileResolver(ctrl)
	mockPreflight := NewMockPreflight(ctrl)
	mockLauncher := NewMockLauncher(ctrl)
	mockTracker := NewMockTracker(ctrl)
	mockReadiness := NewMockReadiness(ctrl)
	mockPool := NewMockPool(ctrl)
	mockPublisher := NewMockPublisher(ctrl)
	mockSubscriber := NewMockSubscriber(ctrl)
	mockReporter := NewMockReporter(ctrl)

	log := slog.New(slog.DiscardHandler)
	guard := NewGuard(mockTracker)

	options := Options{RetryAttempts: 3}

	runtime := NewRuntime(RuntimeParams{
		Options:    options,
		Profiles:   mockProfiles,
		Preflight:  mockPreflight,
		Launcher:   mockLauncher,
		Tracker:    mockTracker,
		Readiness:  mockReadiness,
		Pool:       mockPool,
		Guard:      guard,
		Publisher:  mockPublisher,
		Subscriber: mockSubscriber,
		Reporter:   mockReporter,
		Logger:     log,
	})

	assert.NotNil(t, runtime)
	assert.Equal(t, options, runtime.options)
	assert.Equal(t, mockProfiles, runtime.profiles)
	assert.Equal(t, mockPreflight, runtime.preflight)
	assert.Equal(t, mockLauncher, runtime.launcher)
	assert.Equal(t, mockTracker, runtime.tracker)
	assert.Equal(t, mockReadiness, runtime.readiness)
	assert.Equal(t, mockPool, runtime.pool)
	assert.Equal(t, guard, runtime.guard)
	assert.Equal(t, mockPublisher, runtime.publisher)
	assert.Equal(t, mockSubscriber, runtime.subscriber)
	assert.Equal(t, mockReporter, runtime.reporter)
	assert.Equal(t, log, runtime.log)
}

func Test_Runtime_Run(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockProfiles := NewMockProfileResolver(ctrl)
	mockPreflight := NewMockPreflight(ctrl)
	mockLauncher := NewMockLauncher(ctrl)
	mockTracker := NewMockTracker(ctrl)
	mockReadiness := NewMockReadiness(ctrl)
	mockPool := NewMockPool(ctrl)
	mockPublisher := NewMockPublisher(ctrl)
	mockProcess := NewMockProcess(ctrl)

	log := slog.New(slog.DiscardHandler)
	emptyStream := func() io.Reader { return strings.NewReader("") }

	http := &model.Readiness{Type: model.ReadinessHTTP, URL: "http://localhost:8080/health"}
	svc := model.Service{ID: "test-id-api", Name: "api", Directory: "api", Tier: "platform", Readiness: http}
	tiers := []model.Tier{{Name: "platform", Services: []*model.Service{&svc}}}
	scanErr := errors.New("scan failed")
	running := make(chan struct{})

	var (
		mu       sync.Mutex
		recorded []contracts.Message
	)

	record := func(msg contracts.Message) error {
		mu.Lock()
		defer mu.Unlock()

		recorded = append(recorded, msg)

		if data, ok := msg.Data.(contracts.PhaseChanged); ok && data.Phase == model.PhaseRunning {
			close(running)
		}

		return nil
	}
	types := func() []contracts.MessageType {
		mu.Lock()
		defer mu.Unlock()

		result := make([]contracts.MessageType, len(recorded))
		for i, msg := range recorded {
			result[i] = msg.Type
		}

		return result
	}
	phases := func() []model.Phase {
		mu.Lock()
		defer mu.Unlock()

		var result []model.Phase

		for _, msg := range recorded {
			if data, ok := msg.Data.(contracts.PhaseChanged); ok {
				result = append(result, data.Phase)
			}
		}

		return result
	}

	runtime := NewRuntime(RuntimeParams{
		Options:   Options{RetryAttempts: 1, Profile: "missing"},
		Profiles:  mockProfiles,
		Preflight: mockPreflight,
		Launcher:  mockLauncher,
		Tracker:   mockTracker,
		Readiness: mockReadiness,
		Pool:      mockPool,
		Guard:     NewGuard(mockTracker),
		Publisher: mockPublisher,
		Logger:    log,
	})

	mockPublisher.EXPECT().Publish(gomock.Any()).DoAndReturn(record).AnyTimes()
	mockReadiness.EXPECT().ProbePort(*http).Return(model.Port{Address: "localhost:8080"}).AnyTimes()
	mockProcess.EXPECT().PID().Return(42).AnyTimes()
	mockProcess.EXPECT().Service().Return(svc).AnyTimes()
	mockProcess.EXPECT().Stdout().DoAndReturn(emptyStream).AnyTimes()
	mockProcess.EXPECT().Stderr().DoAndReturn(emptyStream).AnyTimes()
	mockTracker.EXPECT().Untrack(svc.ID, mockProcess).Return(false).AnyTimes()

	tests := []struct {
		name     string
		before   func() context.Context
		types    []contracts.MessageType
		phases   []model.Phase
		expected error
	}{
		{
			name: "returns the profile error after announcing the startup",
			before: func() context.Context {
				mockProfiles.EXPECT().Resolve("missing").Return(nil, contracts.ErrProfileNotFound)
				mockTracker.EXPECT().Reverse().Return(nil)

				return t.Context()
			},
			types:    []contracts.MessageType{contracts.EventPhaseChanged, contracts.EventPhaseChanged, contracts.EventPhaseChanged},
			phases:   []model.Phase{model.PhaseStartup, model.PhaseStopping, model.PhaseStopped},
			expected: contracts.ErrProfileNotFound,
		},
		{
			name: "stops at once when the profile has no services",
			before: func() context.Context {
				mockProfiles.EXPECT().Resolve("missing").Return([]model.Tier{{Name: "platform"}}, nil)
				mockTracker.EXPECT().Reverse().Return(nil)

				return t.Context()
			},
			types:  []contracts.MessageType{contracts.EventPhaseChanged, contracts.EventProfileResolved, contracts.EventPhaseChanged, contracts.EventPhaseChanged},
			phases: []model.Phase{model.PhaseStartup, model.PhaseStopping, model.PhaseStopped},
		},
		{
			name: "starts the profile after a failed cleanup and stops it when the run is cancelled",
			before: func() context.Context {
				ctx, cancel := context.WithCancel(t.Context())
				done := make(chan struct{})
				terminate := func() error {
					close(done)

					return nil
				}

				go func() {
					<-running
					cancel()
				}()

				mockProfiles.EXPECT().Resolve("missing").Return(tiers, nil)
				mockPreflight.EXPECT().Cleanup(gomock.Any(), gomock.Any()).Return(scanErr)
				mockPool.EXPECT().Acquire(gomock.Any()).Return(nil)
				mockPool.EXPECT().Release()
				mockLauncher.EXPECT().Start(svc).Return(mockProcess, nil)
				mockReadiness.EXPECT().Check(gomock.Any(), *http, mockProcess).Return(nil)
				mockProcess.EXPECT().Done().Return(done).AnyTimes()
				mockProcess.EXPECT().Terminate().DoAndReturn(terminate)
				mockTracker.EXPECT().Reverse().Return([]contracts.Process{mockProcess})
				mockTracker.EXPECT().Detach(svc.ID).Times(2)
				mockTracker.EXPECT().Get(svc.ID).Return(mockProcess, true)

				return ctx
			},
			types: []contracts.MessageType{
				contracts.EventPhaseChanged,
				contracts.EventProfileResolved,
				contracts.EventTierStarting,
				contracts.EventServiceStarting,
				contracts.EventServiceReady,
				contracts.EventTierReady,
				contracts.EventPhaseChanged,
				contracts.EventPhaseChanged,
				contracts.EventServiceStopping,
				contracts.EventServiceStopped,
				contracts.EventPhaseChanged,
			},
			phases: []model.Phase{model.PhaseStartup, model.PhaseRunning, model.PhaseStopping, model.PhaseStopped},
		},
		{
			name: "a StopAll while the preflight runs interrupts the startup before any service starts",
			before: func() context.Context {
				stopping := func(ctx context.Context, _ map[string]string) error {
					runtime.handle(contracts.Message{Type: contracts.CommandStopAll})

					return ctx.Err()
				}

				mockProfiles.EXPECT().Resolve("missing").Return(tiers, nil)
				mockPreflight.EXPECT().Cleanup(gomock.Any(), gomock.Any()).DoAndReturn(stopping)
				mockTracker.EXPECT().Reverse().Return(nil)

				return t.Context()
			},
			types: []contracts.MessageType{
				contracts.EventPhaseChanged,
				contracts.EventProfileResolved,
				contracts.EventPhaseChanged,
				contracts.EventPhaseChanged,
			},
			phases:   []model.Phase{model.PhaseStartup, model.PhaseStopping, model.PhaseStopped},
			expected: errStopAll,
		},
		{
			name: "a StopAll during startup interrupts the startup and stops what started",
			before: func() context.Context {
				stopping := func(ctx context.Context, _ model.Readiness, _ contracts.Process) error {
					runtime.handle(contracts.Message{Type: contracts.CommandStopAll})

					return ctx.Err()
				}

				mockProfiles.EXPECT().Resolve("missing").Return(tiers, nil)
				mockPreflight.EXPECT().Cleanup(gomock.Any(), gomock.Any()).Return(nil)
				mockPool.EXPECT().Acquire(gomock.Any()).Return(nil)
				mockPool.EXPECT().Release()
				mockLauncher.EXPECT().Start(svc).Return(mockProcess, nil)
				mockReadiness.EXPECT().Check(gomock.Any(), *http, mockProcess).DoAndReturn(stopping)
				mockProcess.EXPECT().Terminate().Return(nil)
				mockTracker.EXPECT().Reverse().Return(nil)

				return t.Context()
			},
			types: []contracts.MessageType{
				contracts.EventPhaseChanged,
				contracts.EventProfileResolved,
				contracts.EventTierStarting,
				contracts.EventServiceStarting,
				contracts.EventServiceStopped,
				contracts.EventPhaseChanged,
				contracts.EventPhaseChanged,
			},
			phases:   []model.Phase{model.PhaseStartup, model.PhaseStopping, model.PhaseStopped},
			expected: errStartupInterrupted,
		},
		{
			name: "a cancellation during startup interrupts the startup with the context error",
			before: func() context.Context {
				ctx, cancel := context.WithCancel(t.Context())
				cancelling := func(ctx context.Context, _ model.Readiness, _ contracts.Process) error {
					cancel()

					return ctx.Err()
				}

				mockProfiles.EXPECT().Resolve("missing").Return(tiers, nil)
				mockPreflight.EXPECT().Cleanup(gomock.Any(), gomock.Any()).Return(nil)
				mockPool.EXPECT().Acquire(gomock.Any()).Return(nil)
				mockPool.EXPECT().Release()
				mockLauncher.EXPECT().Start(svc).Return(mockProcess, nil)
				mockReadiness.EXPECT().Check(gomock.Any(), *http, mockProcess).DoAndReturn(cancelling)
				mockProcess.EXPECT().Terminate().Return(nil)
				mockTracker.EXPECT().Reverse().Return(nil)

				return ctx
			},
			types: []contracts.MessageType{
				contracts.EventPhaseChanged,
				contracts.EventProfileResolved,
				contracts.EventTierStarting,
				contracts.EventServiceStarting,
				contracts.EventServiceStopped,
				contracts.EventPhaseChanged,
				contracts.EventPhaseChanged,
			},
			phases:   []model.Phase{model.PhaseStartup, model.PhaseStopping, model.PhaseStopped},
			expected: context.Canceled,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorded = nil
			ctx := tt.before()

			err := runtime.run(runtime.begin(ctx))

			require.ErrorIs(t, err, tt.expected)
			assert.Equal(t, tt.types, types())
			assert.Equal(t, tt.phases, phases())
			assert.True(t, ctrl.Satisfied())
		})
	}
}

func Test_Runtime_Run_StopAllInterruption(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockProfiles := NewMockProfileResolver(ctrl)
	mockPreflight := NewMockPreflight(ctrl)
	mockPool := NewMockPool(ctrl)
	mockTracker := NewMockTracker(ctrl)
	mockPublisher := NewMockPublisher(ctrl)

	log := slog.New(slog.DiscardHandler)

	svc := model.Service{ID: "test-id-api", Name: "api"}
	tiers := []model.Tier{{Name: "platform", Services: []*model.Service{&svc}}}

	runtime := NewRuntime(RuntimeParams{
		Options:   Options{Profile: "default"},
		Profiles:  mockProfiles,
		Preflight: mockPreflight,
		Tracker:   mockTracker,
		Pool:      mockPool,
		Guard:     NewGuard(mockTracker),
		Publisher: mockPublisher,
		Logger:    log,
	})

	stopping := func(ctx context.Context) error {
		runtime.handle(contracts.Message{Type: contracts.CommandStopAll})

		return ctx.Err()
	}

	mockPublisher.EXPECT().Publish(gomock.Any()).Return(nil).AnyTimes()
	mockProfiles.EXPECT().Resolve("default").Return(tiers, nil)
	mockPreflight.EXPECT().Cleanup(gomock.Any(), gomock.Any()).Return(nil)
	mockPool.EXPECT().Acquire(gomock.Any()).DoAndReturn(stopping)
	mockTracker.EXPECT().Reverse().Return(nil)

	err := runtime.run(runtime.begin(t.Context()))

	require.EqualError(t, err, "startup interrupted: StopAll command")
	assert.Equal(t, model.PhaseStopped, runtime.guard.phase)
}

func Test_Runtime_StartTier(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLauncher := NewMockLauncher(ctrl)
	mockTracker := NewMockTracker(ctrl)
	mockPool := NewMockPool(ctrl)
	mockPublisher := NewMockPublisher(ctrl)
	mockProcess := NewMockProcess(ctrl)

	log := slog.New(slog.DiscardHandler)
	emptyStream := func() io.Reader { return strings.NewReader("") }

	api := model.Service{ID: "test-id-api", Name: "api", Tier: "platform"}
	worker := model.Service{ID: "test-id-worker", Name: "worker", Tier: "platform"}
	tiers := []model.Tier{{Name: "platform", Services: []*model.Service{&api, &worker}}}
	launchErr := errors.New("launch failed")
	running := make(chan struct{})

	isFailed := func(svc model.Service, cause error) gomock.Matcher {
		return gomock.Cond(func(msg contracts.Message) bool {
			data, ok := msg.Data.(contracts.ServiceFailed)

			return msg.Type == contracts.EventServiceFailed && ok && reflect.DeepEqual(data.Service, svc) && data.Tier == "platform" && errors.Is(data.Error, cause)
		})
	}
	isStopped := func(svc model.Service) gomock.Matcher {
		return gomock.Cond(func(msg contracts.Message) bool {
			data, ok := msg.Data.(contracts.ServiceStopped)

			return msg.Type == contracts.EventServiceStopped && ok && reflect.DeepEqual(data.Service, svc) && data.Tier == "platform"
		})
	}
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
	mockProcess.EXPECT().Stdout().DoAndReturn(emptyStream).AnyTimes()
	mockProcess.EXPECT().Stderr().DoAndReturn(emptyStream).AnyTimes()
	mockProcess.EXPECT().Done().Return(running).AnyTimes()

	tests := []struct {
		name     string
		before   func() context.Context
		services []*model.Service
		expected []string
	}{
		{
			name: "a service takes a worker before its launch and releases it once ready",
			before: func() context.Context {
				runtime.guard.open()
				runtime.guard.resolve(tiers)

				gomock.InOrder(
					mockPool.EXPECT().Acquire(gomock.Any()).Return(nil),
					mockLauncher.EXPECT().Start(api).Return(mockProcess, nil),
					mockPublisher.EXPECT().Publish(isType(contracts.EventServiceStarting)).Return(nil),
					mockPublisher.EXPECT().Publish(isType(contracts.EventServiceReady)).Return(nil),
					mockPool.EXPECT().Release(),
				)

				return t.Context()
			},
			services: []*model.Service{&api},
			expected: []string{},
		},
		{
			name: "a service without a worker fails without a launch",
			before: func() context.Context {
				runtime.guard.open()
				runtime.guard.resolve(tiers)

				mockPool.EXPECT().Acquire(gomock.Any()).Return(context.Canceled).Times(2)
				mockPublisher.EXPECT().Publish(isFailed(api, contracts.ErrFailedToAcquireWorker)).Return(nil)
				mockPublisher.EXPECT().Publish(isFailed(worker, contracts.ErrFailedToAcquireWorker)).Return(nil)

				return t.Context()
			},
			services: []*model.Service{&api, &worker},
			expected: []string{"api", "worker"},
		},
		{
			name: "a run that ends while the service waits for a worker ends it as stopped",
			before: func() context.Context {
				ctx, cancel := context.WithCancel(t.Context())
				cancel()

				runtime.guard.open()
				runtime.guard.resolve(tiers)

				mockPool.EXPECT().Acquire(ctx).Return(context.Canceled)
				mockPublisher.EXPECT().Publish(isStopped(api)).Return(nil)

				return ctx
			},
			services: []*model.Service{&api},
			expected: []string{"api"},
		},
		{
			name: "a failed start names the service and leaves the others alone",
			before: func() context.Context {
				runtime.guard.open()
				runtime.guard.resolve(tiers)

				mockPool.EXPECT().Acquire(gomock.Any()).Return(nil).Times(2)
				mockPool.EXPECT().Release().Times(2)
				mockLauncher.EXPECT().Start(api).Return(mockProcess, nil)
				mockLauncher.EXPECT().Start(worker).Return(nil, launchErr)
				mockPublisher.EXPECT().Publish(isType(contracts.EventServiceStarting)).Return(nil)
				mockPublisher.EXPECT().Publish(isType(contracts.EventServiceReady)).Return(nil)
				mockPublisher.EXPECT().Publish(isFailed(worker, launchErr)).Return(nil)

				return t.Context()
			},
			services: []*model.Service{&api, &worker},
			expected: []string{"worker"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := tt.before()

			failed := runtime.startTier(ctx, tt.services)

			assert.ElementsMatch(t, tt.expected, failed)
			assert.True(t, ctrl.Satisfied())

			for _, svc := range tt.services {
				assert.True(t, runtime.guard.reserve(svc.ID), svc.Name)
			}
		})
	}
}

func Test_Runtime_StartAllTiers(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLauncher := NewMockLauncher(ctrl)
	mockTracker := NewMockTracker(ctrl)
	mockPool := NewMockPool(ctrl)
	mockPublisher := NewMockPublisher(ctrl)
	mockProcess := NewMockProcess(ctrl)

	log := slog.New(slog.DiscardHandler)
	emptyStream := func() io.Reader { return strings.NewReader("") }

	postgres := model.Service{ID: "test-id-postgres", Name: "postgres", Tier: "foundation"}
	api := model.Service{ID: "test-id-api", Name: "api", Tier: "platform"}
	tiers := []model.Tier{
		{Name: "foundation", Services: []*model.Service{&postgres}},
		{Name: "empty"},
		{Name: "platform", Services: []*model.Service{&api}},
	}
	launchErr := errors.New("launch failed")
	running := make(chan struct{})

	var (
		mu       sync.Mutex
		recorded []contracts.Message
	)

	record := func(msg contracts.Message) error {
		mu.Lock()
		defer mu.Unlock()

		recorded = append(recorded, msg)

		return nil
	}
	tierEvents := func() []contracts.Message {
		mu.Lock()
		defer mu.Unlock()

		var result []contracts.Message

		for _, msg := range recorded {
			if msg.Type == contracts.EventTierStarting || msg.Type == contracts.EventTierReady {
				result = append(result, msg)
			}
		}

		return result
	}
	isTier := func(msgType contracts.MessageType, name string) gomock.Matcher {
		return gomock.Cond(func(msg contracts.Message) bool {
			switch data := msg.Data.(type) {
			case contracts.TierStarting:
				return msgType == contracts.EventTierStarting && data.Name == name
			case contracts.TierReady:
				return msgType == contracts.EventTierReady && data.Name == name
			default:
				return false
			}
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

	runtime.guard.open()
	runtime.guard.resolve(tiers)

	mockPublisher.EXPECT().Publish(gomock.Any()).DoAndReturn(record).AnyTimes()
	mockPool.EXPECT().Acquire(gomock.Any()).Return(nil).Times(2)
	mockPool.EXPECT().Release().Times(2)
	mockProcess.EXPECT().PID().Return(42).AnyTimes()
	mockProcess.EXPECT().Stdout().DoAndReturn(emptyStream).AnyTimes()
	mockProcess.EXPECT().Stderr().DoAndReturn(emptyStream).AnyTimes()
	mockProcess.EXPECT().Done().Return(running).AnyTimes()
	gomock.InOrder(
		mockLauncher.EXPECT().Start(postgres).Return(nil, launchErr),
		mockLauncher.EXPECT().Start(api).Return(mockProcess, nil),
	)

	runtime.startAllTiers(t.Context(), tiers)

	events := tierEvents()
	require.Len(t, events, 3)
	assert.True(t, isTier(contracts.EventTierStarting, "foundation").Matches(events[0]))
	assert.True(t, isTier(contracts.EventTierStarting, "platform").Matches(events[1]))
	assert.True(t, isTier(contracts.EventTierReady, "platform").Matches(events[2]))
}

func Test_Runtime_Start_Producer(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockProfiles := NewMockProfileResolver(ctrl)
	mockTracker := NewMockTracker(ctrl)
	mockPublisher := NewMockPublisher(ctrl)
	mockReporter := NewMockReporter(ctrl)

	log := slog.New(slog.DiscardHandler)
	options := Options{Profile: "backend"}
	resolveErr := errors.New("profile not found")
	isType := func(msgType contracts.MessageType) gomock.Matcher {
		return gomock.Cond(func(msg contracts.Message) bool { return msg.Type == msgType })
	}

	mockPublisher.EXPECT().Publish(isType(contracts.EventPhaseChanged)).Return(nil).AnyTimes()

	tests := []struct {
		name   string
		before func() *Runtime
	}{
		{
			name: "a failed run is reported to the arbiter",
			before: func() *Runtime {
				runtime := NewRuntime(RuntimeParams{Options: options, Profiles: mockProfiles, Tracker: mockTracker, Guard: NewGuard(mockTracker), Publisher: mockPublisher, Reporter: mockReporter, Logger: log})
				reported := make(chan error, 1)
				report := func(err error) { reported <- err }
				assertReported := func() {
					assert.EqualError(t, <-reported, "failed to run profile 'backend': failed to resolve profile: profile not found")
				}

				mockProfiles.EXPECT().Resolve("backend").Return(nil, resolveErr)
				mockTracker.EXPECT().Reverse().Return(nil)
				mockReporter.EXPECT().Fail(gomock.Any()).Do(report)

				t.Cleanup(assertReported)

				return runtime
			},
		},
		{
			name: "a profile without services finishes cleanly",
			before: func() *Runtime {
				runtime := NewRuntime(RuntimeParams{Options: options, Profiles: mockProfiles, Tracker: mockTracker, Guard: NewGuard(mockTracker), Publisher: mockPublisher, Reporter: mockReporter, Logger: log})

				mockProfiles.EXPECT().Resolve("backend").Return(nil, nil)
				mockTracker.EXPECT().Reverse().Return(nil)
				mockPublisher.EXPECT().Publish(isType(contracts.EventProfileResolved)).Return(nil)

				return runtime
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runtime := tt.before()

			err := runtime.Start(t.Context())

			require.NoError(t, err)

			<-runtime.Done()

			require.NoError(t, runtime.Stop(t.Context()))
		})
	}
}

func Test_Runtime_Stop_Producer(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockProfiles := NewMockProfileResolver(ctrl)
	mockPreflight := NewMockPreflight(ctrl)
	mockTracker := NewMockTracker(ctrl)
	mockPool := NewMockPool(ctrl)
	mockPublisher := NewMockPublisher(ctrl)

	log := slog.New(slog.DiscardHandler)
	svc := model.Service{ID: "test-id-api", Name: "api"}
	tiers := []model.Tier{{Name: "platform", Services: []*model.Service{&svc}}}

	tests := []struct {
		name   string
		before func() *Runtime
	}{
		{
			name: "a runtime that never started has nothing to stop",
			before: func() *Runtime {
				return NewRuntime(RuntimeParams{Options: Options{Profile: "backend"}, Logger: log})
			},
		},
		{
			name: "a started run is cancelled and waited for",
			before: func() *Runtime {
				runtime := NewRuntime(RuntimeParams{Options: Options{Profile: "backend"}, Profiles: mockProfiles, Preflight: mockPreflight, Tracker: mockTracker, Pool: mockPool, Guard: NewGuard(mockTracker), Publisher: mockPublisher, Logger: log})
				interrupted := make(chan struct{})
				blockUntilCancelled := func(ctx context.Context, _ map[string]string) error {
					close(interrupted)
					<-ctx.Done()

					return nil
				}

				mockProfiles.EXPECT().Resolve("backend").Return(tiers, nil)
				mockPreflight.EXPECT().Cleanup(gomock.Any(), gomock.Any()).DoAndReturn(blockUntilCancelled)
				mockPool.EXPECT().Acquire(gomock.Any()).Return(context.Canceled).AnyTimes()
				mockTracker.EXPECT().Reverse().Return(nil)
				mockPublisher.EXPECT().Publish(gomock.Any()).Return(nil).AnyTimes()

				require.NoError(t, runtime.Start(t.Context()))

				<-interrupted

				return runtime
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runtime := tt.before()

			err := runtime.Stop(t.Context())

			require.NoError(t, err)
		})
	}
}

func Test_Runtime_Stop_Deadline(t *testing.T) {
	log := slog.New(slog.DiscardHandler)

	ignoreHalt := func() {}

	runtime := NewRuntime(RuntimeParams{Logger: log})
	runtime.halt = ignoreHalt

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	err := runtime.Stop(ctx)

	require.ErrorIs(t, err, context.Canceled)
}

func Test_Runtime_Start_AcceptsStopAllWhileTheProfileResolves(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockProfiles := NewMockProfileResolver(ctrl)
	mockPreflight := NewMockPreflight(ctrl)
	mockTracker := NewMockTracker(ctrl)
	mockPublisher := NewMockPublisher(ctrl)
	mockReporter := NewMockReporter(ctrl)

	log := slog.New(slog.DiscardHandler)
	svc := model.Service{ID: "test-id-api", Name: "api"}
	tiers := []model.Tier{{Name: "platform", Services: []*model.Service{&svc}}}
	release := make(chan struct{})

	runtime := NewRuntime(RuntimeParams{Options: Options{Profile: "backend"}, Profiles: mockProfiles, Preflight: mockPreflight, Tracker: mockTracker, Guard: NewGuard(mockTracker), Publisher: mockPublisher, Reporter: mockReporter, Logger: log})
	control := NewControl(runtime.guard, mockPublisher)

	resolve := func(string) ([]model.Tier, error) {
		<-release

		return tiers, nil
	}
	forward := func(msg contracts.Message) error {
		runtime.handle(msg)

		return nil
	}
	isStopAll := gomock.Cond(func(msg contracts.Message) bool { return msg.Type == contracts.CommandStopAll })
	isEvent := gomock.Cond(func(msg contracts.Message) bool { return msg.Type != contracts.CommandStopAll })

	mockProfiles.EXPECT().Resolve("backend").DoAndReturn(resolve)
	mockPublisher.EXPECT().Publish(isStopAll).DoAndReturn(forward)
	mockPublisher.EXPECT().Publish(isEvent).Return(nil).AnyTimes()
	mockPreflight.EXPECT().Cleanup(gomock.Any(), gomock.Any()).Return(nil)
	mockTracker.EXPECT().Reverse().Return(nil)

	require.NoError(t, runtime.Start(t.Context()))

	err := control.StopAll()

	close(release)
	<-runtime.Done()

	require.NoError(t, err)
	assert.Equal(t, model.PhaseStopped, runtime.guard.phase)
	require.NoError(t, runtime.Stop(t.Context()))
}

func Test_Runtime_Stop_CancelsUnderTheLaunchLock(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockProfiles := NewMockProfileResolver(ctrl)
	mockPreflight := NewMockPreflight(ctrl)
	mockLauncher := NewMockLauncher(ctrl)
	mockTracker := NewMockTracker(ctrl)
	mockReadiness := NewMockReadiness(ctrl)
	mockPool := NewMockPool(ctrl)
	mockPublisher := NewMockPublisher(ctrl)

	log := slog.New(slog.DiscardHandler)
	http := &model.Readiness{Type: model.ReadinessHTTP, URL: "http://localhost:8080/health"}
	svc := model.Service{ID: "test-id-api", Name: "api", Directory: "api", Tier: "platform", Readiness: http}
	tiers := []model.Tier{{Name: "platform", Services: []*model.Service{&svc}}}
	probing := make(chan struct{})

	var (
		cancelled  <-chan struct{}
		underLock  bool
		haltingRun context.CancelFunc
	)

	runtime := NewRuntime(RuntimeParams{Options: Options{RetryAttempts: 1, Profile: "backend"}, Profiles: mockProfiles, Preflight: mockPreflight, Launcher: mockLauncher, Tracker: mockTracker, Readiness: mockReadiness, Pool: mockPool, Guard: NewGuard(mockTracker), Publisher: mockPublisher, Logger: log})

	capture := func(ctx context.Context, _ map[string]string) error {
		cancelled = ctx.Done()

		return nil
	}
	waitForTheStop := func(model.Readiness) model.Port {
		close(probing)
		<-cancelled

		return model.Port{Address: "localhost:8080"}
	}
	haltUnderTheLock := func() {
		underLock = holdsRunLock(runtime)

		haltingRun()
	}

	mockProfiles.EXPECT().Resolve("backend").Return(tiers, nil)
	mockPreflight.EXPECT().Cleanup(gomock.Any(), gomock.Any()).DoAndReturn(capture)
	mockPool.EXPECT().Acquire(gomock.Any()).Return(nil)
	mockPool.EXPECT().Release()
	mockReadiness.EXPECT().ProbePort(*http).DoAndReturn(waitForTheStop)
	mockTracker.EXPECT().Reverse().Return(nil)
	mockPublisher.EXPECT().Publish(gomock.Any()).Return(nil).AnyTimes()

	require.NoError(t, runtime.Start(t.Context()))

	haltingRun = runtime.halt
	runtime.halt = haltUnderTheLock

	<-probing

	err := runtime.Stop(t.Context())

	require.NoError(t, err)
	assert.True(t, underLock)
	assert.True(t, ctrl.Satisfied())
}

// holdsRunLock reports whether the caller holds the runtime's lock (a TryLock that succeeds found it free and lets go)
func holdsRunLock(r *Runtime) bool {
	if r.mu.TryLock() {
		r.mu.Unlock()

		return false
	}

	return true
}
