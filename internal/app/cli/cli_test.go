package cli

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"fuku/internal/app/runner"
	"fuku/internal/config"
	"fuku/internal/config/logger"
)

func Test_NewCli(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := config.DefaultConfig()
	mockRunner := runner.NewMockRunner(ctrl)
	mockLogger := logger.NewMockLogger(ctrl)

	commandLineInterface := NewCLI(cfg, mockRunner, mockLogger)
	assert.NotNil(t, commandLineInterface)

	instance, ok := commandLineInterface.(*cli)
	assert.True(t, ok)
	assert.NotNil(t, instance)
	assert.Equal(t, cfg, instance.cfg)
	assert.Equal(t, mockRunner, instance.runner)
	assert.Equal(t, mockLogger, instance.log)
}

func Test_Run(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := config.DefaultConfig()
	mockRunner := runner.NewMockRunner(ctrl)
	mockLogger := logger.NewMockLogger(ctrl)

	commandLineInterface := NewCLI(cfg, mockRunner, mockLogger)

	tests := []struct {
		name   string
		args   []string
		before func()
		output string
	}{
		{
			name: "With run command format --run=API",
			args: []string{"--run=API"},
			before: func() {
				mockLogger.EXPECT().Debug().AnyTimes()
				mockRunner.EXPECT().Run(gomock.Any(), "API").Return(nil)
			},
			output: "",
		},
		{
			name: "With help command",
			args: []string{"help"},
			before: func() {
				mockLogger.EXPECT().Debug().AnyTimes()
			},
			output: fmt.Sprintf("%s\n", Usage),
		},
		{
			name: "With version command",
			args: []string{"version"},
			before: func() {
				mockLogger.EXPECT().Debug().AnyTimes()
			},
			output: fmt.Sprintf("Version: %s\n", config.Version),
		},
		{
			name: "With unknown command",
			args: []string{"unknown"},
			before: func() {
				mockLogger.EXPECT().Debug().AnyTimes()
			},
			output: "Unknown command. Use 'fuku help' for more information\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.before()

			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			defer func() {
				w.Close()
				os.Stdout = oldStdout

				var buf bytes.Buffer
				_, _ = io.Copy(&buf, r)
				output := buf.String()
				assert.Equal(t, tt.output, output)

				if rec := recover(); rec == nil {
					t.Fatal("expected os.Exit(0) panic, but none occurred")
				}
			}()

			_ = commandLineInterface.Run(tt.args)
		})
	}
}

func Test_HandleRun_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logger.NewMockLogger(ctrl)
	mockRunner := runner.NewMockRunner(ctrl)
	cfg := &config.Config{}

	mockLogger.EXPECT().Debug().Times(1)
	mockRunner.EXPECT().Run(gomock.Any(), "API").Return(nil)

	commandLineInterface := &cli{
		log:    mockLogger,
		runner: mockRunner,
		cfg:    cfg,
	}

	commandLineInterface.handleRun("API")
}

func Test_HandleHelp(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().Debug().AnyTimes()

	commandLineInterface := &cli{
		log: mockLogger,
	}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	defer func() {
		os.Stdout = oldStdout
	}()

	commandLineInterface.handleHelp()

	w.Close()
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	assert.Equal(t, fmt.Sprintf("%s\n", Usage), output)
}

func Test_HandleVersion(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().Debug().AnyTimes()

	commandLineInterface := &cli{
		log: mockLogger,
	}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	defer func() {
		os.Stdout = oldStdout
	}()

	commandLineInterface.handleVersion()

	w.Close()
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	assert.Equal(t, fmt.Sprintf("Version: %s\n", config.Version), output)
}

func Test_HandleUnknown(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().Debug().AnyTimes()

	commandLineInterface := &cli{
		log: mockLogger,
	}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	defer func() {
		os.Stdout = oldStdout
	}()

	commandLineInterface.handleUnknown()

	w.Close()
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	assert.Equal(t, "Unknown command. Use 'fuku help' for more information\n", output)
}
