package cli

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"fuku/internal/app/runner"
	"fuku/internal/app/state"
	"fuku/internal/config"
	"fuku/internal/config/logger"
)

func Test_newRunModel(t *testing.T) {
	ctx := context.Background()
	cfg := &config.Config{}
	log := logger.NewLogger(&config.Config{})
	profile := "test"

	m := newRunModel(ctx, cfg, log, profile)

	assert.NotNil(t, m.ctx)
	assert.NotNil(t, m.cancel)
	assert.Equal(t, cfg, m.cfg)
	assert.Equal(t, log, m.log)
	assert.Equal(t, profile, m.profile)
	assert.Equal(t, runner.PhaseDiscovery, m.stateMgr.GetPhase())
	assert.NotNil(t, m.stateMgr.GetServices())
	assert.Len(t, m.stateMgr.GetServices(), 0)
	assert.NotNil(t, m.logMgr.GetLogs())
	assert.Len(t, m.logMgr.GetLogs(), 0)
	assert.NotNil(t, m.stateMgr.GetServiceOrder())
	assert.Len(t, m.stateMgr.GetServiceOrder(), 0)
	assert.Equal(t, 0, m.logMgr.GetFilterCount())
	assert.Equal(t, 0, m.ui.selectedIdx)
	assert.False(t, m.ui.showLogs)
}

func Test_runModel_getPhaseString(t *testing.T) {
	ctx := context.Background()
	m := newRunModel(ctx, &config.Config{}, logger.NewLogger(&config.Config{}), "test")

	tests := []struct {
		phase        runner.Phase
		expectSuffix string
	}{
		{runner.PhaseDiscovery, "Discovery"},
		{runner.PhaseExecution, "Starting services"},
		{runner.PhaseRunning, "✅ Running"},
		{runner.PhaseShutdown, "Stopping services"},
	}

	for _, tt := range tests {
		m.stateMgr.SetPhase(tt.phase)
		result := m.getPhaseString()
		assert.Contains(t, result, tt.expectSuffix)
	}
}

func Test_ServiceState_String(t *testing.T) {
	tests := []struct {
		svcState state.ServiceState
		expected string
	}{
		{state.Starting, "Starting"},
		{state.Running, "Running"},
		{state.Failed, "Failed"},
		{state.Stopped, "Stopped"},
		{state.Stopping, "Stopping"},
		{state.Restarting, "Restarting"},
	}

	for _, tt := range tests {
		result := tt.svcState.String()
		assert.Equal(t, tt.expected, result)
	}
}

func Test_runModel_handleRunnerEvent(t *testing.T) {
	ctx := context.Background()
	cfg := &config.Config{}
	log := logger.NewLogger(&config.Config{})
	m := newRunModel(ctx, cfg, log, "test")

	t.Run("EventPhaseStart", func(t *testing.T) {
		event := runner.Event{
			Type: runner.EventPhaseStart,
			Data: runner.PhaseStart{Phase: runner.PhaseExecution},
		}
		updatedModel, _ := m.handleRunnerEvent(event)
		rm := updatedModel.(runModel)
		assert.Equal(t, runner.PhaseExecution, rm.stateMgr.GetPhase())
	})

	t.Run("EventPhaseDone - Discovery selects all services", func(t *testing.T) {
		event := runner.Event{
			Type: runner.EventPhaseDone,
			Data: runner.PhaseDone{
				Phase:        runner.PhaseDiscovery,
				ServiceCount: 3,
				ServiceNames: []string{"service1", "service2", "service3"},
			},
		}
		updatedModel, _ := m.handleRunnerEvent(event)
		rm := updatedModel.(runModel)
		assert.Equal(t, 3, rm.logMgr.GetFilterCount())
		assert.True(t, rm.logMgr.IsFiltered("service1"))
		assert.True(t, rm.logMgr.IsFiltered("service2"))
		assert.True(t, rm.logMgr.IsFiltered("service3"))
	})

	t.Run("EventServiceStart", func(t *testing.T) {
		freshModel := newRunModel(ctx, cfg, log, "test")
		event := runner.Event{
			Type: runner.EventServiceStart,
			Data: runner.ServiceStart{
				Name:      "test-service",
				PID:       1234,
				StartTime: time.Now(),
			},
		}
		updatedModel, _ := freshModel.handleRunnerEvent(event)
		rm := updatedModel.(runModel)
		assert.Len(t, rm.stateMgr.GetServices(), 1)
		svc, exists := rm.stateMgr.GetService("test-service")
		assert.True(t, exists)
		assert.Equal(t, state.Initializing, svc.State)
		assert.Equal(t, 1234, svc.PID)
		assert.Len(t, rm.stateMgr.GetServiceOrder(), 1)
		assert.Equal(t, "test-service", rm.stateMgr.GetServiceOrder()[0])
	})

	t.Run("EventServiceLog", func(t *testing.T) {
		testTime := time.Now()
		event := runner.Event{
			Type: runner.EventServiceLog,
			Data: runner.ServiceLog{
				Name:   "test-service",
				Stream: "STDOUT",
				Line:   "test log line",
				Time:   testTime,
			},
		}
		updatedModel, _ := m.handleRunnerEvent(event)
		rm := updatedModel.(runModel)
		logs := rm.logMgr.GetLogs()
		assert.Len(t, logs, 1)
		assert.Equal(t, "test-service", logs[0].ServiceName)
		assert.Contains(t, logs[0].Text, "test-service")
		assert.Contains(t, logs[0].Text, "test log line")
	})

	t.Run("EventServiceStop - exit code 0", func(t *testing.T) {
		m.stateMgr.AddService(&state.ServiceStatus{
			Name:  "test-service",
			State: state.Running,
		})
		event := runner.Event{
			Type: runner.EventServiceStop,
			Data: runner.ServiceStop{
				Name:         "test-service",
				ExitCode:     0,
				StopTime:     time.Now(),
				GracefulStop: false,
			},
		}
		updatedModel, _ := m.handleRunnerEvent(event)
		rm := updatedModel.(runModel)
		svc, _ := rm.stateMgr.GetService("test-service")
		assert.Equal(t, state.Stopped, svc.State)
		assert.Equal(t, 0, svc.ExitCode)
	})

	t.Run("EventServiceStop - graceful with SIGTERM", func(t *testing.T) {
		m.stateMgr.AddService(&state.ServiceStatus{
			Name:  "test-service",
			State: state.Running,
		})
		event := runner.Event{
			Type: runner.EventServiceStop,
			Data: runner.ServiceStop{
				Name:         "test-service",
				ExitCode:     143,
				StopTime:     time.Now(),
				GracefulStop: true,
			},
		}
		updatedModel, _ := m.handleRunnerEvent(event)
		rm := updatedModel.(runModel)
		svc, _ := rm.stateMgr.GetService("test-service")
		assert.Equal(t, state.Stopped, svc.State)
		assert.Equal(t, 143, svc.ExitCode)
	})

	t.Run("EventServiceStop - graceful with any signal exit code", func(t *testing.T) {
		m.stateMgr.AddService(&state.ServiceStatus{
			Name:  "test-service",
			State: state.Running,
		})
		event := runner.Event{
			Type: runner.EventServiceStop,
			Data: runner.ServiceStop{
				Name:         "test-service",
				ExitCode:     255,
				StopTime:     time.Now(),
				GracefulStop: true,
			},
		}
		updatedModel, _ := m.handleRunnerEvent(event)
		rm := updatedModel.(runModel)
		svc, _ := rm.stateMgr.GetService("test-service")
		assert.Equal(t, state.Stopped, svc.State)
		assert.Equal(t, 255, svc.ExitCode)
	})

	t.Run("EventServiceStop - failure with non-zero exit", func(t *testing.T) {
		m.stateMgr.AddService(&state.ServiceStatus{
			Name:  "test-service",
			State: state.Running,
		})
		event := runner.Event{
			Type: runner.EventServiceStop,
			Data: runner.ServiceStop{
				Name:         "test-service",
				ExitCode:     1,
				StopTime:     time.Now(),
				GracefulStop: false,
			},
		}
		updatedModel, _ := m.handleRunnerEvent(event)
		rm := updatedModel.(runModel)
		svc, _ := rm.stateMgr.GetService("test-service")
		assert.Equal(t, state.Failed, svc.State)
		assert.Equal(t, 1, svc.ExitCode)
	})

	t.Run("EventServiceFail", func(t *testing.T) {
		m.stateMgr.AddService(&state.ServiceStatus{
			Name:  "test-service",
			State: state.Running,
		})
		testErr := assert.AnError
		event := runner.Event{
			Type: runner.EventServiceFail,
			Data: runner.ServiceFail{
				Name:  "test-service",
				Error: testErr,
				Time:  time.Now(),
			},
		}
		updatedModel, _ := m.handleRunnerEvent(event)
		rm := updatedModel.(runModel)
		svc, _ := rm.stateMgr.GetService("test-service")
		assert.Equal(t, state.Failed, svc.State)
		assert.Equal(t, testErr, svc.Err)
	})
}

func Test_runModel_updateLogViewport(t *testing.T) {
	ctx := context.Background()
	cfg := &config.Config{}
	log := logger.NewLogger(&config.Config{})
	m := newRunModel(ctx, cfg, log, "test")

	m.logMgr.AddLog("service1", "test log for service1")
	m.logMgr.AddLog("service2", "test log for service2")
	m.logMgr.AddLog("service1", "another log for service1")

	t.Run("No filter selected - no logs shown", func(t *testing.T) {
		m.updateLogViewport()
		content := strings.TrimSpace(m.ui.logViewport.View())
		assert.Equal(t, "", content)
	})

	t.Run("Filter single service", func(t *testing.T) {
		m.logMgr.AddFilter("service1")
		m.updateLogViewport()
		content := m.ui.logViewport.View()
		assert.Contains(t, content, "service1")
		assert.NotContains(t, content, "service2")
	})

	t.Run("Filter multiple services", func(t *testing.T) {
		m.logMgr.AddFilter("service2")
		m.updateLogViewport()
		content := m.ui.logViewport.View()
		assert.Contains(t, content, "service1")
		assert.Contains(t, content, "service2")
	})
}

func Test_wrapText(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		width    int
		expected []string
	}{
		{
			name:     "Short text fits in width",
			text:     "hello world",
			width:    20,
			expected: []string{"hello world"},
		},
		{
			name:     "Text longer than width wraps by character",
			text:     "this is a very long line that needs wrapping",
			width:    20,
			expected: []string{"this is a very long ", "line that needs wrap", "ping"},
		},
		{
			name:     "Long log line wraps by character",
			text:     "[2025-10-14T11:30:12.583412] [api] INF ../pkg/common/log/entry.go:144 > Server transport credentials created",
			width:    80,
			expected: []string{"[2025-10-14T11:30:12.583412] [api] INF ../pkg/common/log/entry.go:144 > Server t", "ransport credentials created"},
		},
		{
			name:     "Text with ANSI codes wraps by visible characters",
			text:     "\x1b[38;2;102;102;102m[timestamp]\x1b[0m \x1b[1;38;2;45;80;22m[service]\x1b[0m test log message that is very long and needs wrapping",
			width:    50,
			expected: []string{"\x1b[38;2;102;102;102m[timestamp]\x1b[0m \x1b[1;38;2;45;80;22m[service]\x1b[0m test log message that is ver", "y long and needs wrapping"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := wrapText(tt.text, tt.width)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_getVisibleLength(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		expected int
	}{
		{
			name:     "Plain text",
			text:     "hello world",
			expected: 11,
		},
		{
			name:     "Text with ANSI codes",
			text:     "\x1b[38;2;102;102;102m[timestamp]\x1b[0m plain text",
			expected: 22,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getVisibleLength(tt.text)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_runModel_getFilterString(t *testing.T) {
	ctx := context.Background()
	m := newRunModel(ctx, &config.Config{}, logger.NewLogger(&config.Config{}), "test")

	tests := []struct {
		name          string
		serviceOrder  []string
		filteredNames map[string]bool
		expected      string
	}{
		{
			name:          "No services selected",
			serviceOrder:  []string{"service1", "service2"},
			filteredNames: make(map[string]bool),
			expected:      "None",
		},
		{
			name:          "All services selected",
			serviceOrder:  []string{"service1", "service2"},
			filteredNames: map[string]bool{"service1": true, "service2": true},
			expected:      "All services",
		},
		{
			name:          "Single service",
			serviceOrder:  []string{"service1", "service2"},
			filteredNames: map[string]bool{"service1": true},
			expected:      "service1",
		},
		{
			name:          "Multiple services",
			serviceOrder:  []string{"service1", "service2", "service3"},
			filteredNames: map[string]bool{"service1": true, "service2": true},
			expected:      "2 services",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m = newRunModel(ctx, &config.Config{}, logger.NewLogger(&config.Config{}), "test")
			for _, name := range tt.serviceOrder {
				m.stateMgr.AddService(&state.ServiceStatus{Name: name})
			}
			for name, filtered := range tt.filteredNames {
				if filtered {
					m.logMgr.AddFilter(name)
				}
			}
			result := m.getFilterString()
			assert.Equal(t, tt.expected, result)
		})
	}
}
