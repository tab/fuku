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

func Test_Run_WithEmptyArgs(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := config.DefaultConfig()
	mockRunner := runner.NewMockRunner(ctrl)
	mockLogger := logger.NewMockLogger(ctrl)

	commandLineInterface := NewCLI(cfg, mockRunner, mockLogger)

	defer func() {
		if rec := recover(); rec == nil {
			t.Fatal("expected os.Exit(0) panic, but none occurred")
		}
	}()

	err := commandLineInterface.Run([]string{})
	assert.NoError(t, err)
}

func Test_Run_WithEmptyScope(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := config.DefaultConfig()
	mockRunner := runner.NewMockRunner(ctrl)
	mockLogger := logger.NewMockLogger(ctrl)

	mockLogger.EXPECT().Debug().Return(nil).AnyTimes()
	mockRunner.EXPECT().Run(gomock.Any(), "default").Return(nil)

	commandLineInterface := NewCLI(cfg, mockRunner, mockLogger)

	defer func() {
		if rec := recover(); rec == nil {
			t.Fatal("expected os.Exit(0) panic, but none occurred")
		}
	}()

	err := commandLineInterface.Run([]string{"--run="})
	assert.NoError(t, err)
}

func Test_Run_WithScope(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := config.DefaultConfig()
	mockRunner := runner.NewMockRunner(ctrl)
	mockLogger := logger.NewMockLogger(ctrl)

	mockLogger.EXPECT().Debug().Return(nil).AnyTimes()
	mockRunner.EXPECT().Run(gomock.Any(), "testscope").Return(nil)

	commandLineInterface := NewCLI(cfg, mockRunner, mockLogger)

	defer func() {
		if rec := recover(); rec == nil {
			t.Fatal("expected os.Exit(0) panic, but none occurred")
		}
	}()

	err := commandLineInterface.Run([]string{"run", "testscope"})
	assert.NoError(t, err)
}

func Test_Run_RunCommandWithoutScope(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := config.DefaultConfig()
	mockRunner := runner.NewMockRunner(ctrl)
	mockLogger := logger.NewMockLogger(ctrl)

	mockLogger.EXPECT().Debug().Return(nil).AnyTimes()
	mockRunner.EXPECT().Run(gomock.Any(), "default").Return(nil)

	commandLineInterface := NewCLI(cfg, mockRunner, mockLogger)

	defer func() {
		if rec := recover(); rec == nil {
			t.Fatal("expected os.Exit(0) panic, but none occurred")
		}
	}()

	err := commandLineInterface.Run([]string{"run"})
	assert.NoError(t, err)
}

func Test_Run_WithOneDash(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := config.DefaultConfig()
	mockRunner := runner.NewMockRunner(ctrl)
	mockLogger := logger.NewMockLogger(ctrl)

	mockLogger.EXPECT().Debug().Return(nil).AnyTimes()
	mockRunner.EXPECT().Run(gomock.Any(), "myscope").Return(nil)

	commandLineInterface := NewCLI(cfg, mockRunner, mockLogger)

	defer func() {
		if rec := recover(); rec == nil {
			t.Fatal("expected os.Exit(0) panic, but none occurred")
		}
	}()

	err := commandLineInterface.Run([]string{"-r", "myscope"})
	assert.NoError(t, err)
}

func Test_Run_WithDoubleDash(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := config.DefaultConfig()
	mockRunner := runner.NewMockRunner(ctrl)
	mockLogger := logger.NewMockLogger(ctrl)

	mockLogger.EXPECT().Debug().Return(nil).AnyTimes()
	mockRunner.EXPECT().Run(gomock.Any(), "anothescope").Return(nil)

	commandLineInterface := NewCLI(cfg, mockRunner, mockLogger)

	defer func() {
		if rec := recover(); rec == nil {
			t.Fatal("expected os.Exit(0) panic, but none occurred")
		}
	}()

	err := commandLineInterface.Run([]string{"--run", "anothescope"})
	assert.NoError(t, err)
}

func Test_Run_Help(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := config.DefaultConfig()
	mockRunner := runner.NewMockRunner(ctrl)
	mockLogger := logger.NewMockLogger(ctrl)

	commandLineInterface := NewCLI(cfg, mockRunner, mockLogger)

	variants := []string{"--help", "-h"}

	for _, variant := range variants {
		t.Run(fmt.Sprintf("help_%s", variant), func(t *testing.T) {
			mockLogger.EXPECT().Debug().Return(nil).AnyTimes()

			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			defer func() {
				w.Close()
				os.Stdout = oldStdout

				var buf bytes.Buffer
				_, _ = io.Copy(&buf, r)
				output := buf.String()
				assert.Equal(t, fmt.Sprintf("%s\n", Usage), output)

				if rec := recover(); rec == nil {
					t.Fatal("expected os.Exit(0) panic, but none occurred")
				}
			}()

			_ = commandLineInterface.Run([]string{variant})
		})
	}
}

func Test_Run_Version(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := config.DefaultConfig()
	mockRunner := runner.NewMockRunner(ctrl)
	mockLogger := logger.NewMockLogger(ctrl)

	commandLineInterface := NewCLI(cfg, mockRunner, mockLogger)

	variants := []string{"--version", "-v"}

	for _, variant := range variants {
		t.Run(fmt.Sprintf("version_%s", variant), func(t *testing.T) {
			mockLogger.EXPECT().Debug().Return(nil).AnyTimes()

			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			defer func() {
				w.Close()
				os.Stdout = oldStdout

				var buf bytes.Buffer
				_, _ = io.Copy(&buf, r)
				output := buf.String()
				assert.Equal(t, fmt.Sprintf("Version: %s\n", config.Version), output)

				if rec := recover(); rec == nil {
					t.Fatal("expected os.Exit(0) panic, but none occurred")
				}
			}()

			_ = commandLineInterface.Run([]string{variant})
		})
	}
}

func Test_Run_Unknown(t *testing.T) {
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
