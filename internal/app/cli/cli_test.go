package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"fuku/internal/app/runner"
	"fuku/internal/app/runtime"
	"fuku/internal/config"
	"fuku/internal/config/logger"
)

func Test_NewCLI(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := config.DefaultConfig()
	mockRunner := runner.NewMockRunner(ctrl)
	mockLogger := logger.NewMockLogger(ctrl)
	mockEvent := runtime.NewNoOpEventBus()
	mockCommand := runtime.NewNoOpCommandBus()

	cliInstance := NewCLI(cfg, mockRunner, mockLogger, mockEvent, mockCommand)
	assert.NotNil(t, cliInstance)

	instance, ok := cliInstance.(*cli)
	assert.True(t, ok)
	assert.NotNil(t, instance)
	assert.Equal(t, cfg, instance.cfg)
	assert.Equal(t, mockRunner, instance.runner)
	assert.Equal(t, mockLogger, instance.log)
	assert.Equal(t, mockEvent, instance.event)
	assert.Equal(t, mockCommand, instance.command)
}

func Test_Run(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRunner := runner.NewMockRunner(ctrl)
	mockLogger := logger.NewMockLogger(ctrl)
	cfg := config.DefaultConfig()

	c := &cli{
		cfg:     cfg,
		runner:  mockRunner,
		log:     mockLogger,
		event:   runtime.NewNoOpEventBus(),
		command: runtime.NewNoOpCommandBus(),
	}

	tests := []struct {
		name          string
		before        func()
		args          []string
		expectedExit  int
		expectedError bool
	}{
		{
			name: "No arguments - default profile with --no-ui",
			args: []string{"--no-ui"},
			before: func() {
				mockLogger.EXPECT().Debug().Return(nil)
				mockRunner.EXPECT().Run(gomock.AssignableToTypeOf(context.Background()), config.DefaultProfile).Return(nil)
			},
			expectedExit:  0,
			expectedError: false,
		},
		{
			name: "Help command",
			args: []string{"help"},
			before: func() {
				mockLogger.EXPECT().Debug().Return(nil)
			},
			expectedExit:  0,
			expectedError: false,
		},
		{
			name: "Version command",
			args: []string{"version"},
			before: func() {
				mockLogger.EXPECT().Debug().Return(nil)
			},
			expectedExit:  0,
			expectedError: false,
		},
		{
			name: "Run command with profile and --no-ui",
			args: []string{"run", "test-profile", "--no-ui"},
			before: func() {
				mockLogger.EXPECT().Debug().Return(nil)
				mockRunner.EXPECT().Run(gomock.AssignableToTypeOf(context.Background()), "test-profile").Return(nil)
			},
			expectedExit:  0,
			expectedError: false,
		},
		{
			name: "Run command with --run=profile and --no-ui",
			args: []string{"--run=test-profile", "--no-ui"},
			before: func() {
				mockLogger.EXPECT().Debug().Return(nil)
				mockRunner.EXPECT().Run(gomock.AssignableToTypeOf(context.Background()), "test-profile").Return(nil)
			},
			expectedExit:  0,
			expectedError: false,
		},
		{
			name: "Run command with --run= (empty profile defaults to default profile) and --no-ui",
			args: []string{"--run=", "--no-ui"},
			before: func() {
				mockLogger.EXPECT().Debug().Return(nil)
				mockRunner.EXPECT().Run(gomock.AssignableToTypeOf(context.Background()), config.DefaultProfile).Return(nil)
			},
			expectedExit:  0,
			expectedError: false,
		},
		{
			name: "Run command failure with --no-ui",
			args: []string{"run", "failed-profile", "--no-ui"},
			before: func() {
				mockLogger.EXPECT().Debug().Return(nil)
				mockRunner.EXPECT().Run(gomock.AssignableToTypeOf(context.Background()), "failed-profile").Return(errors.New("runner failed"))
				mockLogger.EXPECT().Error().Return(nil)
			},
			expectedExit:  1,
			expectedError: true,
		},
		{
			name: "Unknown command",
			args: []string{"unknown"},
			before: func() {
				mockLogger.EXPECT().Debug().Return(nil)
			},
			expectedExit:  1,
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			tt.before()
			exitCode, err := c.Run(tt.args)

			w.Close()

			os.Stdout = oldStdout

			var buf bytes.Buffer

			_, _ = io.Copy(&buf, r)

			assert.Equal(t, tt.expectedExit, exitCode)

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
	}

	tests := []struct {
		name          string
		before        func()
		profile       string
		expectedExit  int
		expectedError bool
	}{
		{
			name:    "Success",
			profile: "test-profile",
			before: func() {
				mockLogger.EXPECT().Debug().Return(nil)
				mockRunner.EXPECT().Run(gomock.AssignableToTypeOf(context.Background()), "test-profile").Return(nil)
			},
			expectedExit:  0,
			expectedError: false,
		},
		{
			name:    "Failure",
			profile: "failed-profile",
			before: func() {
				mockLogger.EXPECT().Debug().Return(nil)
				mockRunner.EXPECT().Run(gomock.AssignableToTypeOf(context.Background()), "failed-profile").Return(errors.New("runner failed"))
				mockLogger.EXPECT().Error().Return(nil)
			},
			expectedExit:  1,
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			tt.before()
			exitCode, err := c.handleRun(tt.profile, true)

			w.Close()

			os.Stdout = oldStdout

			var buf bytes.Buffer

			_, _ = io.Copy(&buf, r)

			assert.Equal(t, tt.expectedExit, exitCode)

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
	mockLogger.EXPECT().Debug().Return(nil).AnyTimes()

	c := &cli{log: mockLogger}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	_, _ = c.handleHelp()

	w.Close()

	os.Stdout = oldStdout

	var buf bytes.Buffer

	_, _ = io.Copy(&buf, r)
	output := buf.String()

	assert.Equal(t, fmt.Sprintf("%s\n", Usage), output)
}

func Test_handleVersion(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().Debug().Return(nil).AnyTimes()

	c := &cli{log: mockLogger}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	_, _ = c.handleVersion()

	w.Close()

	os.Stdout = oldStdout

	var buf bytes.Buffer

	_, _ = io.Copy(&buf, r)
	output := buf.String()

	assert.Equal(t, fmt.Sprintf("Version: %s\n", config.Version), output)
}

func Test_handleUnknown(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().Debug().Return(nil).AnyTimes()

	c := &cli{log: mockLogger}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	_, _ = c.handleUnknown()

	w.Close()

	os.Stdout = oldStdout

	var buf bytes.Buffer

	_, _ = io.Copy(&buf, r)
	output := buf.String()

	assert.Equal(t, "Unknown command. Use 'fuku help' for more information\n", output)
}
