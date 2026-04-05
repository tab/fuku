package cli

import (
	"bytes"
	"context"
	"io"
	"os"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"fuku/internal/app/bus"
	"fuku/internal/app/errors"
	"fuku/internal/app/logs"
	"fuku/internal/app/runner"
	"fuku/internal/app/ui/wire"
	"fuku/internal/app/watcher"
	"fuku/internal/config"
	"fuku/internal/config/logger"
)

func Test_NewTUI(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cmd := &Options{Type: CommandRun, Profile: config.Default, NoUI: true}
	mockRunner := runner.NewMockRunner(ctrl)
	mockWatcher := watcher.NewMockWatcher(ctrl)
	mockLogsScreen := logs.NewMockScreen(ctrl)
	mockUI := func(ctx context.Context, profile string) (*tea.Program, error) {
		return nil, nil
	}
	mockLogger := logger.NewMockLogger(ctrl)
	componentLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().WithComponent("TUI").Return(componentLogger)

	b := bus.NoOp()
	tuiInstance := NewTUI(cmd, b, mockRunner, mockWatcher, mockLogsScreen, mockUI, mockLogger)
	assert.NotNil(t, tuiInstance)

	instance, ok := tuiInstance.(*tui)
	assert.True(t, ok)
	assert.NotNil(t, instance)
	assert.Equal(t, cmd, instance.cmd)
	assert.Equal(t, b, instance.bus)
	assert.Equal(t, mockRunner, instance.runner)
	assert.Equal(t, mockWatcher, instance.watcher)
	assert.Equal(t, mockLogsScreen, instance.streamer)
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
				mockRunner.EXPECT().Run(gomock.Any(), config.Default).Return(nil)
				mockWatcher.EXPECT().Close()
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
				mockRunner.EXPECT().Stop(gomock.Any(), config.Default).Return(nil)
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
				mockRunner.EXPECT().Run(gomock.Any(), "test-profile").Return(nil)
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
				mockRunner.EXPECT().Run(gomock.Any(), "failed-profile").Return(errors.New("runner failed"))
				mockWatcher.EXPECT().Close()
				mockLogger.EXPECT().Error().Return(nil)
			},
			expectedExit:  1,
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tu := &tui{
				cmd:     tt.cmd,
				bus:     bus.NoOp(),
				runner:  mockRunner,
				watcher: mockWatcher,
				ui:      mockUI,
				log:     mockLogger,
			}

			oldStdout := os.Stdout
			r, w, err := os.Pipe()
			require.NoError(t, err)

			os.Stdout = w

			tt.before()

			exitCode, err := tu.Execute(t.Context())

			w.Close()

			os.Stdout = oldStdout

			var buf bytes.Buffer

			_, _ = io.Copy(&buf, r)

			assert.Equal(t, tt.expectedExit, exitCode)

			if tt.expectedError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func Test_Execute_LogsMode(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogsScreen := logs.NewMockScreen(ctrl)
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
			tu := &tui{
				cmd:      tt.cmd,
				bus:      bus.NoOp(),
				streamer: mockLogsScreen,
				log:      mockLogger,
			}

			mockLogsScreen.EXPECT().Run(gomock.Any(), tt.profile, tt.services).Return(0)

			exitCode, err := tu.Execute(t.Context())

			assert.Equal(t, 0, exitCode)
			require.NoError(t, err)
		})
	}
}

func Test_Execute_LogsMode_RespectsContextCancellation(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogsScreen := logs.NewMockScreen(ctrl)
	mockLogger := logger.NewMockLogger(ctrl)

	mockLogsScreen.EXPECT().Run(gomock.Any(), "", []string{"api"}).DoAndReturn(
		func(ctx context.Context, _ string, _ []string) int {
			<-ctx.Done()

			return 0
		},
	)

	tu := &tui{
		cmd:      &Options{Type: CommandLogs, Profile: "", Services: []string{"api"}},
		bus:      bus.NoOp(),
		streamer: mockLogsScreen,
		log:      mockLogger,
	}

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})

	go func() {
		tu.Execute(ctx)
		close(done)
	}()

	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Execute did not return after context cancellation")
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
				mockRunner.EXPECT().Run(gomock.Any(), "test-profile").Return(nil)
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
				mockRunner.EXPECT().Run(gomock.Any(), "failed-profile").Return(errors.New("runner failed"))
				mockWatcher.EXPECT().Close()
				mockLogger.EXPECT().Error().Return(nil)
			},
			expectedExit:  1,
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tu := &tui{
				cmd:     &Options{Type: CommandRun, Profile: tt.profile, NoUI: true},
				runner:  mockRunner,
				watcher: mockWatcher,
				log:     mockLogger,
			}

			oldStdout := os.Stdout
			r, w, err := os.Pipe()
			require.NoError(t, err)

			os.Stdout = w

			tt.before()
			exitCode, err := tu.handleRun(context.Background(), tt.profile)

			w.Close()

			os.Stdout = oldStdout

			var buf bytes.Buffer

			_, _ = io.Copy(&buf, r)

			assert.Equal(t, tt.expectedExit, exitCode)

			if tt.expectedError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

type quitModel struct{}

func (m quitModel) Init() tea.Cmd                       { return tea.Quit }
func (m quitModel) Update(tea.Msg) (tea.Model, tea.Cmd) { return m, tea.Quit }
func (m quitModel) View() tea.View                      { return tea.NewView("") }

func Test_runWithUI_UICreationError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRunner := runner.NewMockRunner(ctrl)
	mockLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().Error().Return(nil)

	mockRunner.EXPECT().Run(gomock.Any(), "test").Return(nil).AnyTimes()

	tu := &tui{
		cmd:    &Options{Type: CommandRun, Profile: "test", NoUI: false},
		runner: mockRunner,
		log:    mockLogger,
		ui: func(ctx context.Context, profile string) (*tea.Program, error) {
			return nil, errors.New("failed to create UI")
		},
	}

	exitCode, err := tu.runWithUI(t.Context(), "test")

	assert.Equal(t, 1, exitCode)
	require.Error(t, err)
}

func Test_runWithUI_RunnerError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRunner := runner.NewMockRunner(ctrl)
	mockLogger := logger.NewMockLogger(ctrl)

	mockRunner.EXPECT().Run(gomock.Any(), "test").DoAndReturn(func(ctx context.Context, profile string) error {
		<-ctx.Done()
		return errors.New("runner failed")
	})
	mockLogger.EXPECT().Error().Return(nil)

	inputR, inputW, err := os.Pipe()
	require.NoError(t, err)
	inputW.Close()

	tu := &tui{
		cmd:    &Options{Type: CommandRun, Profile: "test", NoUI: false},
		runner: mockRunner,
		log:    mockLogger,
		ui: func(ctx context.Context, profile string) (*tea.Program, error) {
			return tea.NewProgram(quitModel{}, tea.WithInput(inputR), tea.WithoutRenderer()), nil
		},
	}

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)

	os.Stdout = w

	exitCode, err := tu.runWithUI(t.Context(), "test")

	w.Close()

	os.Stdout = oldStdout

	inputR.Close()

	var buf bytes.Buffer

	_, _ = io.Copy(&buf, r)
	r.Close()

	assert.Equal(t, 1, exitCode)
	require.Error(t, err)
}

func Test_runWithUI_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRunner := runner.NewMockRunner(ctrl)
	mockLogger := logger.NewMockLogger(ctrl)

	mockRunner.EXPECT().Run(gomock.Any(), "test").DoAndReturn(func(ctx context.Context, profile string) error {
		<-ctx.Done()
		return nil
	})

	inputR, inputW, err := os.Pipe()
	require.NoError(t, err)
	inputW.Close()

	tu := &tui{
		cmd:    &Options{Type: CommandRun, Profile: "test", NoUI: false},
		runner: mockRunner,
		log:    mockLogger,
		ui: func(ctx context.Context, profile string) (*tea.Program, error) {
			return tea.NewProgram(quitModel{}, tea.WithInput(inputR), tea.WithoutRenderer()), nil
		},
	}

	exitCode, err := tu.runWithUI(t.Context(), "test")

	inputR.Close()

	assert.Equal(t, 0, exitCode)
	require.NoError(t, err)
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
				mockRunner.EXPECT().Stop(gomock.Any(), "test-profile").Return(nil)
			},
			expectedExit:  0,
			expectedError: false,
		},
		{
			name:    "Failure",
			profile: "failed-profile",
			before: func() {
				mockLogger.EXPECT().Debug().Return(nil)
				mockRunner.EXPECT().Stop(gomock.Any(), "failed-profile").Return(errors.New("stop failed"))
				mockLogger.EXPECT().Error().Return(nil)
			},
			expectedExit:  1,
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tu := &tui{
				cmd:    &Options{Type: CommandStop, Profile: tt.profile},
				runner: mockRunner,
				log:    mockLogger,
			}

			tt.before()
			exitCode, err := tu.handleStop(context.Background(), tt.profile)

			assert.Equal(t, tt.expectedExit, exitCode)

			if tt.expectedError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
