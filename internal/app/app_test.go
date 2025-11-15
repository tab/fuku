package app

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/fx"
	"go.uber.org/mock/gomock"

	"fuku/internal/app/cli"
	"fuku/internal/config/logger"
)

func Test_NewApp(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCLI := cli.NewMockCLI(ctrl)
	mockLogger := logger.NewMockLogger(ctrl)

	application := NewApp(mockCLI, mockLogger)

	assert.NotNil(t, application)
	assert.Equal(t, mockCLI, application.cli)
	assert.Equal(t, mockLogger, application.log)
}

func Test_execute(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCLI := cli.NewMockCLI(ctrl)
	mockLogger := logger.NewMockLogger(ctrl)

	app := &App{
		cli: mockCLI,
		log: mockLogger,
	}

	tests := []struct {
		name             string
		before           func()
		args             []string
		expectedExitCode int
	}{
		{
			name: "Success",
			args: []string{"help"},
			before: func() {
				mockCLI.EXPECT().Run([]string{"help"}).Return(0, nil)
			},
			expectedExitCode: 0,
		},
		{
			name: "Failure",
			args: []string{"run", "failed-profile"},
			before: func() {
				mockCLI.EXPECT().Run([]string{"run", "failed-profile"}).Return(1, errors.New("runner failed"))
				mockLogger.EXPECT().Error().Return(nil)
			},
			expectedExitCode: 1,
		},
		{
			name: "With no arguments",
			args: []string{},
			before: func() {
				mockCLI.EXPECT().Run([]string{}).Return(0, nil)
			},
			expectedExitCode: 0,
		},
		{
			name: "With multiple arguments",
			args: []string{"run", "test-profile", "extra"},
			before: func() {
				mockCLI.EXPECT().Run([]string{"run", "test-profile", "extra"}).Return(0, nil)
			},
			expectedExitCode: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.before()
			exitCode := app.execute(tt.args)
			assert.Equal(t, tt.expectedExitCode, exitCode)
		})
	}
}

func Test_Register(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCLI := cli.NewMockCLI(ctrl)
	mockLogger := logger.NewMockLogger(ctrl)
	app := NewApp(mockCLI, mockLogger)

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

func Test_Register_OnStopHook(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCLI := cli.NewMockCLI(ctrl)
	mockLogger := logger.NewMockLogger(ctrl)
	app := NewApp(mockCLI, mockLogger)

	var capturedHook fx.Hook

	testLifecycle := &testLifecycleImpl{
		onAppend: func(hook fx.Hook) {
			capturedHook = hook
		},
	}

	Register(testLifecycle, app)

	assert.NotNil(t, capturedHook.OnStop)
	err := capturedHook.OnStop(context.Background())
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
