package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"
	"go.uber.org/mock/gomock"

	"fuku/internal/app/cli"
	"fuku/internal/config/sentry"
)

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

	application := NewApp(mockTUI, mockSentry, &noopShutdowner{})

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
	app := NewApp(mockTUI, mockSentry, shutdowner)

	app.Run(t.Context())

	assert.True(t, shutdowner.called)
}

func Test_Register(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTUI := cli.NewMockTUI(ctrl)
	mockSentry := sentry.NewMockSentry(ctrl)
	app := NewApp(mockTUI, mockSentry, &noopShutdowner{})

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
	app := NewApp(mockTUI, mockSentry, &noopShutdowner{})

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
	app := NewApp(mockTUI, mockSentry, &noopShutdowner{})

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
