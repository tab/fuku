package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"fuku/internal/app/logs"
	"fuku/internal/app/runner"
	"fuku/internal/app/ui/wire"
	"fuku/internal/app/watcher"
	"fuku/internal/config"
	"fuku/internal/config/logger"
)

func Test_NewCLI(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cmd := &Options{Type: CommandRun, Profile: config.Default, NoUI: true}
	mockRunner := runner.NewMockRunner(ctrl)
	mockWatcher := watcher.NewMockWatcher(ctrl)
	mockLogsRunner := logs.NewMockRunner(ctrl)
	mockUI := func(ctx context.Context, profile string) (*tea.Program, error) {
		return nil, nil
	}
	mockLogger := logger.NewMockLogger(ctrl)
	componentLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().WithComponent("CLI").Return(componentLogger)

	cliInstance := NewCLI(cmd, mockRunner, mockWatcher, mockLogsRunner, mockUI, mockLogger)
	assert.NotNil(t, cliInstance)

	instance, ok := cliInstance.(*cli)
	assert.True(t, ok)
	assert.NotNil(t, instance)
	assert.Equal(t, cmd, instance.cmd)
	assert.Equal(t, mockRunner, instance.runner)
	assert.Equal(t, mockWatcher, instance.watcher)
	assert.Equal(t, mockLogsRunner, instance.streamer)
	assert.NotNil(t, instance.ui)
	assert.Equal(t, componentLogger, instance.log)
}

func Test_Execute(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRunner := runner.NewMockRunner(ctrl)
	mockWatcher := watcher.NewMockWatcher(ctrl)
	mockUI := wire.UI(func(ctx context.Context, profile string) (*tea.Program, error) {
		return nil, nil
	})
	mockLogger := logger.NewMockLogger(ctrl)

	tests := []struct {
		name          string
		before        func()
		cmd           *Options
		expectedExit  int
		expectedError bool
	}{
		{
			name: "Run command with default profile and --no-ui",
			cmd: &Options{
				Type:    CommandRun,
				Profile: config.Default,
				NoUI:    true,
			},
			before: func() {
				mockLogger.EXPECT().Debug().Return(nil)
				mockWatcher.EXPECT().Start(gomock.Any())
				mockRunner.EXPECT().Run(gomock.AssignableToTypeOf(context.Background()), config.Default).Return(nil)
				mockWatcher.EXPECT().Close()
			},
			expectedExit:  0,
			expectedError: false,
		},
		{
			name: "Init command",
			cmd: &Options{
				Type:    CommandInit,
				Profile: config.Default,
			},
			before: func() {
				dir := t.TempDir()
				origDir, _ := os.Getwd()

				os.Chdir(dir)
				t.Cleanup(func() { os.Chdir(origDir) })
				mockLogger.EXPECT().Debug().Return(nil)
			},
			expectedExit:  0,
			expectedError: false,
		},
		{
			name: "Help command",
			cmd: &Options{
				Type:    CommandHelp,
				Profile: config.Default,
			},
			before: func() {
				mockLogger.EXPECT().Debug().Return(nil)
			},
			expectedExit:  0,
			expectedError: false,
		},
		{
			name: "Version command",
			cmd: &Options{
				Type:    CommandVersion,
				Profile: config.Default,
			},
			before: func() {
				mockLogger.EXPECT().Debug().Return(nil)
			},
			expectedExit:  0,
			expectedError: false,
		},
		{
			name: "Stop command",
			cmd: &Options{
				Type:    CommandStop,
				Profile: config.Default,
			},
			before: func() {
				mockLogger.EXPECT().Debug().Return(nil)
				mockRunner.EXPECT().Stop(config.Default).Return(nil)
			},
			expectedExit:  0,
			expectedError: false,
		},
		{
			name: "Run command with profile and --no-ui",
			cmd: &Options{
				Type:    CommandRun,
				Profile: "test-profile",
				NoUI:    true,
			},
			before: func() {
				mockLogger.EXPECT().Debug().Return(nil)
				mockWatcher.EXPECT().Start(gomock.Any())
				mockRunner.EXPECT().Run(gomock.AssignableToTypeOf(context.Background()), "test-profile").Return(nil)
				mockWatcher.EXPECT().Close()
			},
			expectedExit:  0,
			expectedError: false,
		},
		{
			name: "Run command failure with --no-ui",
			cmd: &Options{
				Type:    CommandRun,
				Profile: "failed-profile",
				NoUI:    true,
			},
			before: func() {
				mockLogger.EXPECT().Debug().Return(nil)
				mockWatcher.EXPECT().Start(gomock.Any())
				mockRunner.EXPECT().Run(gomock.AssignableToTypeOf(context.Background()), "failed-profile").Return(errors.New("runner failed"))
				mockWatcher.EXPECT().Close()
				mockLogger.EXPECT().Error().Return(nil)
			},
			expectedExit:  1,
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &cli{
				cmd:     tt.cmd,
				runner:  mockRunner,
				watcher: mockWatcher,
				ui:      mockUI,
				log:     mockLogger,
			}

			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			tt.before()

			exitCode, err := c.Execute()

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

func Test_Execute_LogsMode(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogsRunner := logs.NewMockRunner(ctrl)
	mockLogger := logger.NewMockLogger(ctrl)

	tests := []struct {
		name     string
		cmd      *Options
		profile  string
		services []string
	}{
		{
			name: "Logs with services",
			cmd: &Options{
				Type:     CommandLogs,
				Profile:  "",
				Services: []string{"api"},
			},
			profile:  "",
			services: []string{"api"},
		},
		{
			name: "Logs with profile",
			cmd: &Options{
				Type:     CommandLogs,
				Profile:  "core",
				Services: []string{"api", "db"},
			},
			profile:  "core",
			services: []string{"api", "db"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &cli{
				cmd:      tt.cmd,
				streamer: mockLogsRunner,
				log:      mockLogger,
			}

			mockLogsRunner.EXPECT().Run(tt.profile, tt.services).Return(0)

			exitCode, err := c.Execute()

			assert.Equal(t, 0, exitCode)
			assert.NoError(t, err)
		})
	}
}

func Test_handleRun(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRunner := runner.NewMockRunner(ctrl)
	mockWatcher := watcher.NewMockWatcher(ctrl)
	mockLogger := logger.NewMockLogger(ctrl)

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
				mockWatcher.EXPECT().Start(gomock.Any())
				mockRunner.EXPECT().Run(gomock.AssignableToTypeOf(context.Background()), "test-profile").Return(nil)
				mockWatcher.EXPECT().Close()
			},
			expectedExit:  0,
			expectedError: false,
		},
		{
			name:    "Failure",
			profile: "failed-profile",
			before: func() {
				mockLogger.EXPECT().Debug().Return(nil)
				mockWatcher.EXPECT().Start(gomock.Any())
				mockRunner.EXPECT().Run(gomock.AssignableToTypeOf(context.Background()), "failed-profile").Return(errors.New("runner failed"))
				mockWatcher.EXPECT().Close()
				mockLogger.EXPECT().Error().Return(nil)
			},
			expectedExit:  1,
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &cli{
				cmd:     &Options{Type: CommandRun, Profile: tt.profile, NoUI: true},
				runner:  mockRunner,
				watcher: mockWatcher,
				log:     mockLogger,
			}

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

func Test_handleInit(t *testing.T) {
	t.Run("fuku.yaml already exists", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockLogger := logger.NewMockLogger(ctrl)
		mockLogger.EXPECT().Debug().Return(nil)

		dir := t.TempDir()
		origDir, _ := os.Getwd()

		os.Chdir(dir)
		defer os.Chdir(origDir)

		os.WriteFile(config.ConfigFile, []byte("existing"), 0600)

		c := &cli{log: mockLogger}

		oldStdout := os.Stdout
		r, w, _ := os.Pipe()

		os.Stdout = w

		exitCode, err := c.handleInit()

		w.Close()

		os.Stdout = oldStdout

		var buf bytes.Buffer

		_, _ = io.Copy(&buf, r)

		assert.Equal(t, 0, exitCode)
		assert.NoError(t, err)
		assert.Contains(t, buf.String(), "already exists")

		content, _ := os.ReadFile(config.ConfigFile)
		assert.Equal(t, "existing", string(content))
	})

	t.Run("fuku.yaml created successfully", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockLogger := logger.NewMockLogger(ctrl)
		mockLogger.EXPECT().Debug().Return(nil)

		dir := t.TempDir()
		origDir, _ := os.Getwd()

		os.Chdir(dir)
		defer os.Chdir(origDir)

		c := &cli{log: mockLogger}

		oldStdout := os.Stdout
		r, w, _ := os.Pipe()

		os.Stdout = w

		exitCode, err := c.handleInit()

		w.Close()

		os.Stdout = oldStdout

		var buf bytes.Buffer

		_, _ = io.Copy(&buf, r)

		assert.Equal(t, 0, exitCode)
		assert.NoError(t, err)
		assert.Contains(t, buf.String(), "Created")

		content, readErr := os.ReadFile(config.ConfigFile)
		assert.NoError(t, readErr)
		assert.Contains(t, string(content), "version: 1")
		assert.Contains(t, string(content), "services:")
	})

	t.Run("write error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockLogger := logger.NewMockLogger(ctrl)
		mockLogger.EXPECT().Debug().Return(nil)

		dir := t.TempDir()
		origDir, _ := os.Getwd()

		os.Chdir(dir)
		defer os.Chdir(origDir)

		os.Chmod(dir, 0444)
		defer os.Chmod(dir, 0755)

		c := &cli{log: mockLogger}

		exitCode, err := c.handleInit()

		assert.Equal(t, 1, exitCode)
		assert.Error(t, err)
	})
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

type quitModel struct{}

func (m quitModel) Init() tea.Cmd                       { return tea.Quit }
func (m quitModel) Update(tea.Msg) (tea.Model, tea.Cmd) { return m, tea.Quit }
func (m quitModel) View() string                        { return "" }

func Test_runWithUI(t *testing.T) {
	t.Run("UI creation error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockRunner := runner.NewMockRunner(ctrl)
		mockLogger := logger.NewMockLogger(ctrl)
		mockLogger.EXPECT().Error().Return(nil)

		mockRunner.EXPECT().Run(gomock.Any(), "test").Return(nil).AnyTimes()

		c := &cli{
			cmd:    &Options{Type: CommandRun, Profile: "test", NoUI: false},
			runner: mockRunner,
			log:    mockLogger,
			ui: func(ctx context.Context, profile string) (*tea.Program, error) {
				return nil, errors.New("failed to create UI")
			},
		}

		exitCode, err := c.runWithUI(context.Background(), "test")

		assert.Equal(t, 1, exitCode)
		assert.Error(t, err)
	})

	t.Run("Runner error after UI exits", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockRunner := runner.NewMockRunner(ctrl)
		mockLogger := logger.NewMockLogger(ctrl)

		mockRunner.EXPECT().Run(gomock.Any(), "test").DoAndReturn(func(ctx context.Context, profile string) error {
			<-ctx.Done()
			return errors.New("runner failed")
		})
		mockLogger.EXPECT().Error().Return(nil)

		inputR, inputW, _ := os.Pipe()
		inputW.Close()

		c := &cli{
			cmd:    &Options{Type: CommandRun, Profile: "test", NoUI: false},
			runner: mockRunner,
			log:    mockLogger,
			ui: func(ctx context.Context, profile string) (*tea.Program, error) {
				return tea.NewProgram(quitModel{}, tea.WithInput(inputR), tea.WithoutRenderer()), nil
			},
		}

		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		exitCode, err := c.runWithUI(context.Background(), "test")

		w.Close()

		os.Stdout = oldStdout

		inputR.Close()

		var buf bytes.Buffer

		_, _ = io.Copy(&buf, r)

		assert.Equal(t, 1, exitCode)
		assert.Error(t, err)
	})

	t.Run("Success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockRunner := runner.NewMockRunner(ctrl)
		mockLogger := logger.NewMockLogger(ctrl)

		mockRunner.EXPECT().Run(gomock.Any(), "test").DoAndReturn(func(ctx context.Context, profile string) error {
			<-ctx.Done()
			return nil
		})

		inputR, inputW, _ := os.Pipe()
		inputW.Close()

		c := &cli{
			cmd:    &Options{Type: CommandRun, Profile: "test", NoUI: false},
			runner: mockRunner,
			log:    mockLogger,
			ui: func(ctx context.Context, profile string) (*tea.Program, error) {
				return tea.NewProgram(quitModel{}, tea.WithInput(inputR), tea.WithoutRenderer()), nil
			},
		}

		exitCode, err := c.runWithUI(context.Background(), "test")

		inputR.Close()

		assert.Equal(t, 0, exitCode)
		assert.NoError(t, err)
	})
}

func Test_handleStop(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRunner := runner.NewMockRunner(ctrl)
	mockLogger := logger.NewMockLogger(ctrl)

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
				mockRunner.EXPECT().Stop("test-profile").Return(nil)
			},
			expectedExit:  0,
			expectedError: false,
		},
		{
			name:    "Failure",
			profile: "failed-profile",
			before: func() {
				mockLogger.EXPECT().Debug().Return(nil)
				mockRunner.EXPECT().Stop("failed-profile").Return(errors.New("stop failed"))
				mockLogger.EXPECT().Error().Return(nil)
			},
			expectedExit:  1,
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &cli{
				cmd:    &Options{Type: CommandStop, Profile: tt.profile},
				runner: mockRunner,
				log:    mockLogger,
			}

			tt.before()
			exitCode, err := c.handleStop(tt.profile)

			assert.Equal(t, tt.expectedExit, exitCode)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
