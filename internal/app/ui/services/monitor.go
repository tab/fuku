package services

import (
	"context"
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"fuku/internal/app/ui/components"
)

// ServiceStats holds CPU and memory statistics for a service
type ServiceStats struct {
	CPU float64
	MEM float64
}

// statsUpdateMsg is sent by the background worker with batched stats
type statsUpdateMsg struct {
	Stats  map[string]ServiceStats
	AppCPU float64
	AppMEM float64
}

// applyStatsUpdate applies batched stats to service monitors
func (m *Model) applyStatsUpdate(msg statsUpdateMsg) {
	for serviceName, stats := range msg.Stats {
		if service, exists := m.state.services[serviceName]; exists {
			service.Monitor.CPU = stats.CPU
			service.Monitor.MEM = stats.MEM
		}
	}

	m.state.appCPU = msg.AppCPU
	m.state.appMEM = msg.AppMEM
}

// updateBlinkAnimations updates blink state for services in transition states
func (m *Model) updateBlinkAnimations() bool {
	hasActiveBlinking := false

	for _, service := range m.state.services {
		if service.Blink == nil || service.FSM == nil {
			continue
		}

		switch service.FSM.Current() {
		case Starting, Stopping, Restarting:
			if !service.Blink.IsActive() {
				service.Blink.Start()
			}

			service.Blink.Update()

			hasActiveBlinking = true
		default:
			if service.Blink.IsActive() {
				service.Blink.Stop()
			}
		}
	}

	return hasActiveBlinking
}

// getUptime returns formatted uptime string for a service
func (m *Model) getUptime(service *ServiceState) string {
	if service.Status == StatusStopped || service.Status == StatusFailed || service.Monitor.StartTime.IsZero() {
		return ""
	}

	elapsed := time.Since(service.Monitor.StartTime)
	hours := int(elapsed.Hours())
	minutes := int(elapsed.Minutes()) % 60
	seconds := int(elapsed.Seconds()) % 60

	if hours > 0 {
		return pad(hours) + ":" + pad(minutes) + ":" + pad(seconds)
	}

	return pad(minutes) + ":" + pad(seconds)
}

// formatCPU formats a CPU percentage value
func formatCPU(cpu float64) string {
	return fmt.Sprintf("%.1f%%", cpu)
}

// formatMEM formats a memory value in MB or GB
func formatMEM(mem float64) string {
	if mem < components.MBToGB {
		return fmt.Sprintf("%.0fMB", mem)
	}

	return fmt.Sprintf("%.1fGB", mem/components.MBToGB)
}

// getCPU returns formatted CPU usage for a service
func (m *Model) getCPU(service *ServiceState) string {
	if !m.isServiceMonitored(service) {
		return ""
	}

	return formatCPU(service.Monitor.CPU)
}

// getMem returns formatted memory usage for a service
func (m *Model) getMem(service *ServiceState) string {
	if !m.isServiceMonitored(service) {
		return ""
	}

	return formatMEM(service.Monitor.MEM)
}

// getPID returns the process ID string for a running service
func (m *Model) getPID(service *ServiceState) string {
	if service.Status == StatusRunning && service.Monitor.PID != 0 {
		return fmt.Sprintf("%d", service.Monitor.PID)
	}

	return ""
}

// isServiceMonitored returns true if service has valid monitoring data
func (m *Model) isServiceMonitored(service *ServiceState) bool {
	return service.Status != StatusStopped && service.Status != StatusFailed && service.Monitor.PID != 0
}

// pad formats a number with leading zero
func pad(n int) string {
	return fmt.Sprintf("%02d", n)
}

// statsWorkerCmd schedules a single stats collection and returns the result
func statsWorkerCmd(ctx context.Context, m *Model) tea.Cmd {
	return tea.Tick(components.StatsPollingInterval, func(t time.Time) tea.Msg {
		batchCtx, cancel := context.WithTimeout(ctx, components.StatsBatchTimeout)
		defer cancel()

		stats := m.collectStats(batchCtx)
		appCPU, appMEM := m.collectAppStats(batchCtx)

		return statsUpdateMsg{Stats: stats, AppCPU: appCPU, AppMEM: appMEM}
	})
}

// job represents a stats collection task
type job struct {
	name string
	pid  int
}

// result holds stats collection output
type result struct {
	name  string
	stats ServiceStats
	err   error
}

// collectStats polls all services and returns batched stats with per-call timeouts
func (m *Model) collectStats(ctx context.Context) map[string]ServiceStats {
	jobs := make([]job, 0, len(m.state.services))

	for name, service := range m.state.services {
		if service.Monitor.PID > 0 && service.Status != StatusStopped {
			jobs = append(jobs, job{name: name, pid: service.Monitor.PID})
		}
	}

	if len(jobs) == 0 {
		return nil
	}

	sem := make(chan struct{}, components.StatsMaxConcurrency)
	results := make(chan result, len(jobs))

	launched := m.launchStatsWorkers(ctx, jobs, sem, results)

	stats := make(map[string]ServiceStats)

	for i := 0; i < launched; i++ {
		select {
		case <-ctx.Done():
			return stats
		case result := <-results:
			if result.err == nil {
				stats[result.name] = result.stats
			}
		}
	}

	return stats
}

// launchStatsWorkers spawns goroutines to collect stats concurrently
func (m *Model) launchStatsWorkers(ctx context.Context, jobs []job, sem chan struct{}, results chan result) int {
	launched := 0

	for _, j := range jobs {
		if ctx.Err() != nil {
			return launched
		}

		select {
		case <-ctx.Done():
			return launched
		case sem <- struct{}{}:
			launched++

			go func(j job) {
				defer func() { <-sem }()

				callCtx, cancel := context.WithTimeout(ctx, components.StatsCallTimeout)
				stats, err := m.getStatsWithContext(callCtx, j.pid)

				cancel()

				results <- result{name: j.name, stats: stats, err: err}
			}(j)
		}
	}

	return launched
}

// getStatsWithContext calls monitor.GetStats with context for cancellation support
func (m *Model) getStatsWithContext(ctx context.Context, pid int) (ServiceStats, error) {
	stats, err := m.monitor.GetStats(ctx, pid)
	if err != nil {
		return ServiceStats{}, err
	}

	return ServiceStats{CPU: stats.CPU, MEM: stats.MEM}, nil
}

// collectAppStats collects CPU and memory stats for the fuku process itself
func (m *Model) collectAppStats(ctx context.Context) (cpu float64, mem float64) {
	stats, err := m.monitor.GetStats(ctx, os.Getpid())
	if err != nil {
		return 0, 0
	}

	return stats.CPU, stats.MEM
}
