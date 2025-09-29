package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"fuku/internal/app/runner"
	"fuku/internal/config"
	"fuku/internal/config/logger"
)

func Test_NewCLI(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := config.DefaultConfig()
	mockRunner := runner.NewMockRunner(ctrl)
	mockLogger := logger.NewMockLogger(ctrl)

	cliInstance := NewCLI(cfg, mockRunner, mockLogger)
	assert.NotNil(t, cliInstance)

	instance, ok := cliInstance.(*cli)
	assert.True(t, ok)
	assert.NotNil(t, instance)
	assert.Equal(t, cfg, instance.cfg)
	assert.Equal(t, mockRunner, instance.runner)
	assert.Equal(t, mockLogger, instance.log)
}

func Test_Run(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRunner := runner.NewMockRunner(ctrl)
	mockLogger := logger.NewMockLogger(ctrl)
	cfg := config.DefaultConfig()

	c := &cli{
		cfg:    cfg,
		runner: mockRunner,
		log:    mockLogger,
		phases: NewPhaseTracker(),
	}

	tests := []struct {
		name          string
		before        func()
		args          []string
		expectedError bool
	}{
		{
			name: "No arguments - default profile",
			args: []string{},
			before: func() {
				mockLogger.EXPECT().Debug().Return(&logger.NoopEvent{})
				mockRunner.EXPECT().Run(gomock.AssignableToTypeOf(context.Background()), config.DefaultProfile).Return(nil)
			},
			expectedError: false,
		},
		{
			name: "Help command",
			args: []string{"help"},
			before: func() {
				mockLogger.EXPECT().Debug().Return(&logger.NoopEvent{})
			},
			expectedError: false,
		},
		{
			name: "Version command",
			args: []string{"version"},
			before: func() {
				mockLogger.EXPECT().Debug().Return(&logger.NoopEvent{})
			},
			expectedError: false,
		},
		{
			name: "Run command with profile",
			args: []string{"run", "test-profile"},
			before: func() {
				mockLogger.EXPECT().Debug().Return(&logger.NoopEvent{})
				mockRunner.EXPECT().Run(gomock.AssignableToTypeOf(context.Background()), "test-profile").Return(nil)
			},
			expectedError: false,
		},
		{
			name: "Run command with --run=profile",
			args: []string{"--run=test-profile"},
			before: func() {
				mockLogger.EXPECT().Debug().Return(&logger.NoopEvent{})
				mockRunner.EXPECT().Run(gomock.AssignableToTypeOf(context.Background()), "test-profile").Return(nil)
			},
			expectedError: false,
		},
		{
			name: "Run command with --run= (empty profile defaults to default profile)",
			args: []string{"--run="},
			before: func() {
				mockLogger.EXPECT().Debug().Return(&logger.NoopEvent{})
				mockRunner.EXPECT().Run(gomock.AssignableToTypeOf(context.Background()), config.DefaultProfile).Return(nil)
			},
			expectedError: false,
		},
		{
			name: "Run command failure",
			args: []string{"run", "failed-profile"},
			before: func() {
				mockLogger.EXPECT().Debug().Return(&logger.NoopEvent{})
				mockRunner.EXPECT().Run(gomock.AssignableToTypeOf(context.Background()), "failed-profile").Return(errors.New("runner failed"))
				mockLogger.EXPECT().Error().Return(&logger.NoopEvent{})
			},
			expectedError: true,
		},
		{
			name: "Unknown command",
			args: []string{"unknown"},
			before: func() {
				mockLogger.EXPECT().Debug().Return(&logger.NoopEvent{})
			},
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			tt.before()
			err := c.Run(tt.args)

			_ = w.Close()
			os.Stdout = oldStdout

			var buf bytes.Buffer
			_, _ = io.Copy(&buf, r)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func Test_handleRun(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRunner := runner.NewMockRunner(ctrl)
	mockLogger := logger.NewMockLogger(ctrl)

	c := &cli{
		runner: mockRunner,
		log:    mockLogger,
		phases: NewPhaseTracker(),
	}

	tests := []struct {
		name          string
		before        func()
		profile       string
		expectedError bool
	}{
		{
			name:    "Success",
			profile: "test-profile",
			before: func() {
				mockLogger.EXPECT().Debug().Return(&logger.NoopEvent{})
				mockRunner.EXPECT().Run(gomock.AssignableToTypeOf(context.Background()), "test-profile").Return(nil)
			},
			expectedError: false,
		},
		{
			name:    "Failure",
			profile: "failed-profile",
			before: func() {
				mockLogger.EXPECT().Debug().Return(&logger.NoopEvent{})
				mockRunner.EXPECT().Run(gomock.AssignableToTypeOf(context.Background()), "failed-profile").Return(errors.New("runner failed"))
				mockLogger.EXPECT().Error().Return(&logger.NoopEvent{})
			},
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			tt.before()
			err := c.handleRun(tt.profile)

			_ = w.Close()
			os.Stdout = oldStdout

			var buf bytes.Buffer
			_, _ = io.Copy(&buf, r)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func Test_handleHelp(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().Debug().Return(&logger.NoopEvent{}).AnyTimes()

	c := &cli{log: mockLogger}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	_ = c.handleHelp()

	_ = w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	// Since we now have colorized help output, just verify key elements are present
	assert.Contains(t, output, "fuku")
	assert.Contains(t, output, "USAGE")
	assert.Contains(t, output, "COMMANDS")
	assert.Contains(t, output, "help")
	assert.Contains(t, output, "version")
	assert.Contains(t, output, "run")
	assert.Contains(t, output, "ui")
}

func Test_handleHelp_DebugLog(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRunner := runner.NewMockRunner(ctrl)
	mockLogger := logger.NewMockLogger(ctrl)
	cfg := config.DefaultConfig()

	mockEvent := &logger.NoopEvent{}
	mockLogger.EXPECT().Debug().Return(mockEvent)

	c := &cli{
		cfg:    cfg,
		runner: mockRunner,
		log:    mockLogger,
		phases: NewPhaseTracker(),
	}

	err := c.handleHelp()
	assert.NoError(t, err)
}

func Test_handleVersion(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().Debug().Return(&logger.NoopEvent{}).AnyTimes()

	c := &cli{log: mockLogger}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	_ = c.handleVersion()

	_ = w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	// Since we now have colorized version output, just verify key elements are present
	assert.Contains(t, output, "fuku")
	assert.Contains(t, output, config.Version)
	assert.Contains(t, output, "lightweight CLI orchestrator")
}

func Test_handleVersion_DebugLog(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRunner := runner.NewMockRunner(ctrl)
	mockLogger := logger.NewMockLogger(ctrl)
	cfg := config.DefaultConfig()

	mockEvent := &logger.NoopEvent{}
	mockLogger.EXPECT().Debug().Return(mockEvent)

	c := &cli{
		cfg:    cfg,
		runner: mockRunner,
		log:    mockLogger,
		phases: NewPhaseTracker(),
	}

	err := c.handleVersion()
	assert.NoError(t, err)
}

func Test_handleUnknown(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().Debug().Return(&logger.NoopEvent{}).AnyTimes()

	c := &cli{log: mockLogger}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	_ = c.handleUnknown()

	_ = w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	// Since we now have colorized error output, just verify key elements are present
	assert.Contains(t, output, "Error:")
	assert.Contains(t, output, "Unknown command")
	assert.Contains(t, output, "fuku help")
}

func Test_handleUnknown_DebugLog(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRunner := runner.NewMockRunner(ctrl)
	mockLogger := logger.NewMockLogger(ctrl)
	cfg := config.DefaultConfig()

	mockEvent := &logger.NoopEvent{}
	mockLogger.EXPECT().Debug().Return(mockEvent)

	c := &cli{
		cfg:    cfg,
		runner: mockRunner,
		log:    mockLogger,
		phases: NewPhaseTracker(),
	}

	err := c.handleUnknown()
	assert.Error(t, err)
}
