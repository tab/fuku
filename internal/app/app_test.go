package app

import (
	"context"
	"os"
	"testing"
	"time"

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

	app := NewApp(mockCLI, mockLogger)

	assert.NotNil(t, app)
	assert.Equal(t, mockCLI, app.cli)
	assert.Equal(t, mockLogger, app.log)
}

func Test_Run_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCLI := cli.NewMockCLI(ctrl)
	mockLogger := logger.NewMockLogger(ctrl)

	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()
	os.Args = []string{"fuku", "help"}

	mockCLI.EXPECT().Run([]string{"help"}).Return(nil)

	app := NewApp(mockCLI, mockLogger)
	app.Run()
}

func Test_Register(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCLI := cli.NewMockCLI(ctrl)
	mockLogger := logger.NewMockLogger(ctrl)

	app := NewApp(mockCLI, mockLogger)

	var registered bool
	testLifecycle := testLifecycleImpl{
		onAppend: func(hook fx.Hook) {
			registered = true
			assert.NotNil(t, hook.OnStart)
			assert.NotNil(t, hook.OnStop)
		},
	}

	Register(&testLifecycle, app)
	assert.True(t, registered)
}

type testLifecycleImpl struct {
	onAppend func(fx.Hook)
}

func (t *testLifecycleImpl) Append(hook fx.Hook) {
	if t.onAppend != nil {
		t.onAppend(hook)
	}
}

func Test_Register_HooksRegistered(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCLI := cli.NewMockCLI(ctrl)
	mockLogger := logger.NewMockLogger(ctrl)

	app := NewApp(mockCLI, mockLogger)

	var capturedHook fx.Hook
	testLifecycle := testLifecycleImpl{
		onAppend: func(hook fx.Hook) {
			capturedHook = hook
		},
	}

	Register(&testLifecycle, app)

	assert.NotNil(t, capturedHook.OnStart)
	assert.NotNil(t, capturedHook.OnStop)

	assert.IsType(t, func(context.Context) error { return nil }, capturedHook.OnStart)
	assert.IsType(t, func(context.Context) error { return nil }, capturedHook.OnStop)
}

func Test_Register_OnStopHook(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCLI := cli.NewMockCLI(ctrl)
	mockLogger := logger.NewMockLogger(ctrl)

	app := NewApp(mockCLI, mockLogger)

	var capturedHook fx.Hook
	testLifecycle := testLifecycleImpl{
		onAppend: func(hook fx.Hook) {
			capturedHook = hook
		},
	}

	Register(&testLifecycle, app)

	assert.NotNil(t, capturedHook.OnStop)

	err := capturedHook.OnStop(context.Background())
	assert.NoError(t, err)
}

func Test_Register_OnStartHookExecution(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCLI := cli.NewMockCLI(ctrl)
	mockLogger := logger.NewMockLogger(ctrl)

	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()
	os.Args = []string{"fuku", "version"}

	done := make(chan bool, 1)
	mockCLI.EXPECT().Run([]string{"version"}).Do(func(args []string) {
		done <- true
	}).Return(nil)

	app := NewApp(mockCLI, mockLogger)

	var capturedHook fx.Hook
	testLifecycle := testLifecycleImpl{
		onAppend: func(hook fx.Hook) {
			capturedHook = hook
		},
	}

	Register(&testLifecycle, app)

	assert.NotNil(t, capturedHook.OnStart)

	err := capturedHook.OnStart(context.Background())
	assert.NoError(t, err)

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("OnStart hook did not execute app.Run() within timeout")
	}
}
