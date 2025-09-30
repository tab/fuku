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
	mockTUI := NewMockTUI(ctrl)

	cliInstance := NewCLI(cfg, mockRunner, mockTUI, mockLogger)
	assert.NotNil(t, cliInstance)

	instance, ok := cliInstance.(*cli)
	assert.True(t, ok)
	assert.NotNil(t, instance)
	assert.Equal(t, cfg, instance.cfg)
	assert.Equal(t, mockRunner, instance.runner)
	assert.Equal(t, mockTUI, instance.tui)
	assert.Equal(t, mockLogger, instance.log)
}

func Test_Run(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRunner := runner.NewMockRunner(ctrl)
	mockLogger := logger.NewMockLogger(ctrl)
	mockTUI := NewMockTUI(ctrl)

	cfg := config.DefaultConfig()

	c := &cli{
		cfg:    cfg,
		runner: mockRunner,
		tui:    mockTUI,
		log:    mockLogger,
	}

	tests := []struct {
		name          string
		before        func()
		args          []string
		expectedExit  int
		expectedError bool
	}{
		{
			name: "No arguments - default profile",
			args: []string{},
			before: func() {
				mockLogger.EXPECT().Debug().Return(nil)
				mockRunner.EXPECT().Run(gomock.AssignableToTypeOf(context.Background()), config.DefaultProfile).Return(nil)
			},
			expectedExit:  0,
			expectedError: false,
		},
		{
			name: "Run command with profile",
			args: []string{"run", "test-profile"},
			before: func() {
				mockLogger.EXPECT().Debug().Return(nil)
				mockRunner.EXPECT().Run(gomock.AssignableToTypeOf(context.Background()), "test-profile").Return(nil)
			},
			expectedExit:  0,
			expectedError: false,
		},
		{
			name: "Run command with --run=profile",
			args: []string{"--run=test-profile"},
			before: func() {
				mockLogger.EXPECT().Debug().Return(nil)
				mockRunner.EXPECT().Run(gomock.AssignableToTypeOf(context.Background()), "test-profile").Return(nil)
			},
			expectedExit:  0,
			expectedError: false,
		},
		{
			name: "Run command with --run= (empty profile defaults to default profile)",
			args: []string{"--run="},
			before: func() {
				mockLogger.EXPECT().Debug().Return(nil)
				mockRunner.EXPECT().Run(gomock.AssignableToTypeOf(context.Background()), config.DefaultProfile).Return(nil)
			},
			expectedExit:  0,
			expectedError: false,
		},
		{
			name: "Run command failure",
			args: []string{"run", "failed-profile"},
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
	mockTUI := NewMockTUI(ctrl)

	c := &cli{
		runner: mockRunner,
		tui:    mockTUI,
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
			exitCode, err := c.handleRun(tt.profile)

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

	mockTUI := NewMockTUI(ctrl)
	mockLogger := logger.NewMockLogger(ctrl)

	c := &cli{
		tui: mockTUI,
		log: mockLogger,
	}

	tests := []struct {
		name          string
		before        func()
		expectedExit  int
		expectedError bool
	}{
		{
			name: "Success",
			before: func() {
				mockLogger.EXPECT().Debug().Return(nil)
				mockTUI.EXPECT().Help().Return(nil)
			},
			expectedExit:  0,
			expectedError: false,
		},
		{
			name: "Failure",
			before: func() {
				mockLogger.EXPECT().Debug().Return(nil)
				mockTUI.EXPECT().Help().Return(errors.New("help failed"))
				mockLogger.EXPECT().Error().Return(nil)
			},
			expectedExit:  1,
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.before()
			exitCode, err := c.handleHelp()

			assert.Equal(t, tt.expectedExit, exitCode)
			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func Test_handleVersion(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().Debug().Return(nil)

	c := &cli{log: mockLogger}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	exitCode, err := c.handleVersion()

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	assert.Equal(t, 0, exitCode)
	assert.NoError(t, err)
	assert.Equal(t, "0.2.0 (Fuku)\n", output)
}

func Test_handleUnknown(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().Debug().Return(nil).AnyTimes()
	mockTUI := NewMockTUI(ctrl)

	c := &cli{
		log: mockLogger,
		tui: mockTUI,
	}

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
