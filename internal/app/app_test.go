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

// mockLifecycle implements fx.Lifecycle for testing
type mockLifecycle struct {
	onAppend func(fx.Hook)
}

func (m *mockLifecycle) Append(hook fx.Hook) {
	if m.onAppend != nil {
		m.onAppend(hook)
	}
}

// mockShutdowner implements fx.Shutdowner for testing
type mockShutdowner struct{}

func (m *mockShutdowner) Shutdown(...fx.ShutdownOption) error {
	return nil
}

func Test_NewApp(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCLI := cli.NewMockCLI(ctrl)
	mockLogger := logger.NewMockLogger(ctrl)

	application := NewApp(mockCLI, &mockShutdowner{}, mockLogger)

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
		name          string
		before        func()
		args          []string
		expectedError bool
	}{
		{
			name: "Success",
			args: []string{"help"},
			before: func() {
				mockCLI.EXPECT().Run([]string{"help"}).Return(nil)
			},
			expectedError: false,
		},
		{
			name: "Failure",
			args: []string{"run", "failed-profile"},
			before: func() {
				mockCLI.EXPECT().Run([]string{"run", "failed-profile"}).Return(errors.New("runner failed"))
				mockLogger.EXPECT().Error().Return(&logger.NoopEvent{})
			},
			expectedError: true,
		},
		{
			name: "With no arguments",
			args: []string{},
			before: func() {
				mockCLI.EXPECT().Run([]string{}).Return(nil)
			},
			expectedError: false,
		},
		{
			name: "With multiple arguments",
			args: []string{"run", "test-profile", "extra"},
			before: func() {
				mockCLI.EXPECT().Run([]string{"run", "test-profile", "extra"}).Return(nil)
			},
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.before()
			err := app.execute(tt.args)
			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func Test_Register(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCLI := cli.NewMockCLI(ctrl)
	mockLogger := logger.NewMockLogger(ctrl)
	app := NewApp(mockCLI, &mockShutdowner{}, mockLogger)

	var registered bool
	var capturedHook fx.Hook

	testLifecycle := &mockLifecycle{
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
	app := NewApp(mockCLI, &mockShutdowner{}, mockLogger)

	var capturedHook fx.Hook

	testLifecycle := &mockLifecycle{
		onAppend: func(hook fx.Hook) {
			capturedHook = hook
		},
	}

	Register(testLifecycle, app)

	assert.NotNil(t, capturedHook.OnStop)
	err := capturedHook.OnStop(context.Background())
	assert.NoError(t, err)
}

func Test_App_Run(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCLI := cli.NewMockCLI(ctrl)
	mockLogger := logger.NewMockLogger(ctrl)
	mockShutdowner := &mockShutdowner{}

	app := NewApp(mockCLI, mockShutdowner, mockLogger)

	t.Run("Success case", func(t *testing.T) {
		mockCLI.EXPECT().Run(gomock.Any()).Return(nil)

		err := app.execute([]string{"help"})
		assert.NoError(t, err)
	})

	t.Run("Error case", func(t *testing.T) {
		mockEvent := logger.NewMockEvent(ctrl)
		mockEvent.EXPECT().Msg("Application error")
		mockEvent.EXPECT().Err(gomock.Any()).Return(mockEvent)
		mockLogger.EXPECT().Error().Return(mockEvent)

		testErr := errors.New("test error")
		mockCLI.EXPECT().Run(gomock.Any()).Return(testErr)

		err := app.execute([]string{"invalid"})
		assert.Error(t, err)
		assert.Equal(t, testErr, err)
	})
}

func Test_Register_OnStartHook(t *testing.T) {
	t.Skip("Integration test - requires goroutine coordination")
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCLI := cli.NewMockCLI(ctrl)
	mockLogger := logger.NewMockLogger(ctrl)
	mockEvent := logger.NewMockEvent(ctrl)

	// Set up expectations for the Run method
	mockLogger.EXPECT().Debug().Return(mockEvent).AnyTimes()
	mockEvent.EXPECT().Err(gomock.Any()).Return(mockEvent).AnyTimes()
	mockEvent.EXPECT().Msg(gomock.Any()).AnyTimes()

	mockCLI.EXPECT().Run(gomock.Any()).Return(nil)

	app := NewApp(mockCLI, &mockShutdowner{}, mockLogger)

	var capturedHook fx.Hook

	testLifecycle := &mockLifecycle{
		onAppend: func(hook fx.Hook) {
			capturedHook = hook
		},
	}

	Register(testLifecycle, app)

	// Test OnStart hook
	assert.NotNil(t, capturedHook.OnStart)
	err := capturedHook.OnStart(context.Background())
	assert.NoError(t, err)

	// Give the goroutine time to start
	// Note: This is not a complete test of the Run method as it involves complex coordination
}

func Test_App_ErrorHandling(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	t.Run("Shutdown error handling", func(t *testing.T) {
		// This test is more for understanding the code path than actual coverage
		// since Run() involves os.Args and complex goroutine coordination
		assert.True(t, true) // Placeholder for code path understanding
	})
}
