package app

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/fx"
	"go.uber.org/mock/gomock"

	"fuku/internal/app/cli"
)

func Test_NewApp(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCLI := cli.NewMockCLI(ctrl)

	application := NewApp(mockCLI)

	assert.NotNil(t, application)
	assert.Equal(t, mockCLI, application.cli)
}

func Test_execute(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCLI := cli.NewMockCLI(ctrl)

	app := &App{
		cli: mockCLI,
	}

	tests := []struct {
		name             string
		before           func()
		expectedExitCode int
	}{
		{
			name: "Success",
			before: func() {
				mockCLI.EXPECT().Execute().Return(0, nil)
			},
			expectedExitCode: 0,
		},
		{
			name: "Failure",
			before: func() {
				mockCLI.EXPECT().Execute().Return(1, errors.New("runner failed"))
			},
			expectedExitCode: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.before()

			exitCode := app.execute()
			assert.Equal(t, tt.expectedExitCode, exitCode)
		})
	}
}

func Test_Register(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCLI := cli.NewMockCLI(ctrl)
	app := NewApp(mockCLI)

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

	Register(testLifecycle, app)

	assert.True(t, registered)
	assert.NotNil(t, capturedHook.OnStart)
	assert.NotNil(t, capturedHook.OnStop)
}

func Test_Register_OnStop_ReturnsNil(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCLI := cli.NewMockCLI(ctrl)
	app := NewApp(mockCLI)

	var capturedHook fx.Hook

	testLifecycle := &testLifecycleImpl{
		onAppend: func(hook fx.Hook) {
			capturedHook = hook
		},
	}

	Register(testLifecycle, app)

	ctx := context.Background()
	err := capturedHook.OnStop(ctx)
	assert.NoError(t, err)
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
