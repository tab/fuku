package app

import (
	"context"
	"errors"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"
	"go.uber.org/mock/gomock"

	"fuku/internal/app/bus"
	"fuku/internal/app/cli"
	"fuku/internal/config"
	"fuku/internal/config/logger"
	"fuku/internal/config/sentry"
)

// newTestLogger returns a logger mock that accepts the component split NewApp performs
func newTestLogger(ctrl *gomock.Controller) *logger.MockLogger {
	mockLog := logger.NewMockLogger(ctrl)
	mockLog.EXPECT().WithComponent("APP").Return(mockLog).AnyTimes()
	mockLog.EXPECT().Info().Return(nil).AnyTimes()

	return mockLog
}

func Test_NewRoot(t *testing.T) {
	root := NewRoot()

	assert.NotNil(t, root.Context())

	root.Cancel()
	assert.ErrorIs(t, root.Context().Err(), context.Canceled)
}

func Test_NewApp(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTUI := cli.NewMockTUI(ctrl)
	mockSentry := sentry.NewMockSentry(ctrl)

	application := NewApp(mockTUI, mockSentry, &noopShutdowner{}, bus.NoOp(), NewShutdown(), newTestLogger(ctrl))

	assert.NotNil(t, application)
	assert.Equal(t, mockTUI, application.ui)
	assert.Equal(t, mockSentry, application.sentry)
	assert.NotNil(t, application.done)
}

func Test_execute(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTUI := cli.NewMockTUI(ctrl)
	mockSentry := sentry.NewMockSentry(ctrl)

	app := &App{
		ui:     mockTUI,
		sentry: mockSentry,
		done:   make(chan struct{}),
	}

	tests := []struct {
		name             string
		before           func()
		expectedExitCode int
	}{
		{
			name: "Success",
			before: func() {
				mockTUI.EXPECT().Execute(gomock.Any()).Return(0, nil)
			},
			expectedExitCode: 0,
		},
		{
			name: "Failure",
			before: func() {
				mockTUI.EXPECT().Execute(gomock.Any()).Return(1, errors.New("runner failed"))
			},
			expectedExitCode: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.before()

			exitCode := app.execute(t.Context())
			assert.Equal(t, tt.expectedExitCode, exitCode)
		})
	}
}

func Test_Run_SignalsShutdown(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTUI := cli.NewMockTUI(ctrl)
	mockTUI.EXPECT().Execute(gomock.Any()).Return(0, nil)

	mockSentry := sentry.NewMockSentry(ctrl)
	mockSentry.EXPECT().Flush()

	shutdowner := &recordingShutdowner{}
	shutdown := NewShutdown()
	app := NewApp(mockTUI, mockSentry, shutdowner, bus.NoOp(), shutdown, newTestLogger(ctrl))

	app.Run(t.Context())

	assert.True(t, shutdowner.called)

	shutdown.Observe(syscall.SIGTERM)
	assert.Nil(t, shutdown.Signal())
}

func Test_Register(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTUI := cli.NewMockTUI(ctrl)
	mockSentry := sentry.NewMockSentry(ctrl)
	app := NewApp(mockTUI, mockSentry, &noopShutdowner{}, bus.NoOp(), NewShutdown(), newTestLogger(ctrl))

	var (
		registered   bool
		capturedHook fx.Hook
	)

	testLifecycle := &testLifecycleImpl{
		onAppend: func(hook fx.Hook) {
			registered = true
			capturedHook = hook
		},
	}

	Register(testLifecycle, NewRoot(), app)

	assert.True(t, registered)
	assert.NotNil(t, capturedHook.OnStart)
	assert.NotNil(t, capturedHook.OnStop)
}

func Test_Register_OnStop_CancelsContextAndUnblocksApp(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTUI := cli.NewMockTUI(ctrl)
	mockTUI.EXPECT().Execute(gomock.Any()).DoAndReturn(func(ctx context.Context) (int, error) {
		<-ctx.Done()

		return 0, nil
	})

	mockSentry := sentry.NewMockSentry(ctrl)
	mockSentry.EXPECT().Flush()

	root := NewRoot()
	app := NewApp(mockTUI, mockSentry, &noopShutdowner{}, bus.NoOp(), NewShutdown(), newTestLogger(ctrl))

	var capturedHook fx.Hook

	testLifecycle := &testLifecycleImpl{
		onAppend: func(hook fx.Hook) {
			capturedHook = hook
		},
	}

	Register(testLifecycle, root, app)

	err := capturedHook.OnStart(context.Background())
	require.NoError(t, err)

	done := make(chan error, 1)

	go func() {
		done <- capturedHook.OnStop(context.Background())
	}()

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("OnStop did not return after cancelling root context")
	}
}

func Test_Register_OnStop_RespectsTimeout(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTUI := cli.NewMockTUI(ctrl)
	mockSentry := sentry.NewMockSentry(ctrl)
	app := NewApp(mockTUI, mockSentry, &noopShutdowner{}, bus.NoOp(), NewShutdown(), newTestLogger(ctrl))

	var capturedHook fx.Hook

	testLifecycle := &testLifecycleImpl{
		onAppend: func(hook fx.Hook) {
			capturedHook = hook
		},
	}

	Register(testLifecycle, NewRoot(), app)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := capturedHook.OnStop(ctx)
	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

func Test_Register_OnStop_AnnouncesSignalBeforeCancel(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := config.DefaultConfig()
	cfg.Logs.Buffer = 10

	b := bus.NewBus(cfg, bus.NewFormatter(logger.NewEventLogger()), nil)
	defer b.Close()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	msgChan := b.Subscribe(ctx)

	shutdown := NewShutdown()
	shutdown.Observe(syscall.SIGINT)

	root := NewRoot()
	app := NewApp(cli.NewMockTUI(ctrl), sentry.NewMockSentry(ctrl), &noopShutdowner{}, b, shutdown, newTestLogger(ctrl))

	var capturedHook fx.Hook

	testLifecycle := &testLifecycleImpl{
		onAppend: func(hook fx.Hook) {
			capturedHook = hook
		},
	}

	Register(testLifecycle, root, app)

	stopCtx, stopCancel := context.WithCancel(context.Background())
	stopCancel()

	require.Error(t, capturedHook.OnStop(stopCtx))

	assert.Equal(t, []bus.Message{
		{Type: bus.EventSignal, Data: bus.Signal{Name: "interrupt"}, Critical: true},
	}, drain(msgChan))
	assert.ErrorIs(t, root.Context().Err(), context.Canceled)
}

func Test_PublishSignal(t *testing.T) {
	tests := []struct {
		name     string
		before   func(s *Shutdown)
		expected []bus.Message
	}{
		{
			name:     "No signal to announce",
			before:   func(_ *Shutdown) {},
			expected: nil,
		},
		{
			name: "Signal announced on the bus",
			before: func(s *Shutdown) {
				s.Observe(syscall.SIGTERM)
			},
			expected: []bus.Message{
				{Type: bus.EventSignal, Data: bus.Signal{Name: "terminated"}, Critical: true},
			},
		},
		{
			name: "Application shut itself down",
			before: func(s *Shutdown) {
				s.Self()
				s.Observe(syscall.SIGTERM)
			},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			cfg := config.DefaultConfig()
			cfg.Logs.Buffer = 10

			b := bus.NewBus(cfg, bus.NewFormatter(logger.NewEventLogger()), nil)
			defer b.Close()

			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()

			msgChan := b.Subscribe(ctx)

			shutdown := NewShutdown()
			tt.before(shutdown)

			app := NewApp(cli.NewMockTUI(ctrl), sentry.NewMockSentry(ctrl), &noopShutdowner{}, b, shutdown, newTestLogger(ctrl))
			app.PublishSignal()

			assert.Equal(t, tt.expected, drain(msgChan))
		})
	}
}

// drain collects the messages already buffered on a subscription
func drain(msgChan <-chan bus.Message) []bus.Message {
	var messages []bus.Message

	for {
		select {
		case msg := <-msgChan:
			msg.Seq = 0
			msg.Timestamp = time.Time{}

			messages = append(messages, msg)
		case <-time.After(50 * time.Millisecond):
			return messages
		}
	}
}

// testLifecycleImpl implements fx.Lifecycle for testing
type testLifecycleImpl struct {
	onAppend func(fx.Hook)
}

func (t *testLifecycleImpl) Append(hook fx.Hook) {
	if t.onAppend != nil {
		t.onAppend(hook)
	}
}

// noopShutdowner implements fx.Shutdowner for testing
type noopShutdowner struct{}

func (n *noopShutdowner) Shutdown(_ ...fx.ShutdownOption) error { return nil }

// recordingShutdowner records Shutdown calls for assertions
type recordingShutdowner struct {
	called bool
}

func (r *recordingShutdowner) Shutdown(_ ...fx.ShutdownOption) error {
	r.called = true

	return nil
}
