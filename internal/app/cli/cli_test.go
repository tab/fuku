package cli

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"fuku/internal/config"
	"fuku/internal/config/logger"
)

func Test_NewCli(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logger.NewMockLogger(ctrl)

	commandLineInterface := NewCLI(mockLogger)
	assert.NotNil(t, commandLineInterface)

	instance, ok := commandLineInterface.(*cli)
	assert.True(t, ok)
	assert.NotNil(t, instance)
}

func Test_Run(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logger.NewMockLogger(ctrl)

	commandLineInterface := NewCLI(mockLogger)

	tests := []struct {
		name   string
		args   []string
		before func()
		output string
	}{
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
