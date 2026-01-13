package services

import (
	"context"
	"fmt"
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
	Stats map[string]ServiceStats
}

// applyStatsUpdate applies batched stats to service monitors
func (m *Model) applyStatsUpdate(msg statsUpdateMsg) {
	for serviceName, stats := range msg.Stats {
		if service, exists := m.state.services[serviceName]; exists {
			service.Monitor.CPU = stats.CPU
			service.Monitor.MEM = stats.MEM
		}
	}
}

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

func (m *Model) getCPU(service *ServiceState) string {
	if !m.isServiceMonitored(service) {
		return ""
	}

	return fmt.Sprintf("%.1f%%", service.Monitor.CPU)
}

func (m *Model) getMem(service *ServiceState) string {
	if !m.isServiceMonitored(service) {
		return ""
	}

	if service.Monitor.MEM < components.MBToGB {
		return fmt.Sprintf("%.0fMB", service.Monitor.MEM)
	}

	return fmt.Sprintf("%.1fGB", service.Monitor.MEM/components.MBToGB)
}

func (m *Model) getPID(service *ServiceState) string {
	if service.Status == StatusRunning && service.Monitor.PID != 0 {
		return fmt.Sprintf("%d", service.Monitor.PID)
	}

	return ""
}

func (m *Model) isServiceMonitored(service *ServiceState) bool {
	return service.Status != StatusStopped && service.Status != StatusFailed && service.Monitor.PID != 0
}

func pad(n int) string {
	return fmt.Sprintf("%02d", n)
}

// statsWorkerCmd schedules a single stats collection and returns the result
func statsWorkerCmd(ctx context.Context, m *Model) tea.Cmd {
	return tea.Tick(components.StatsPollingInterval, func(t time.Time) tea.Msg {
		batchCtx, cancel := context.WithTimeout(ctx, components.StatsBatchTimeout)
		defer cancel()

		stats := m.collectStats(batchCtx)

		return statsUpdateMsg{Stats: stats}
	})
}

type job struct {
	name string
	pid  int
}

type result struct {
	name  string
	stats ServiceStats
	err   error
}

// collectStats polls all services and returns batched stats with per-call timeouts
func (m *Model) collectStats(ctx context.Context) map[string]ServiceStats {
	var jobs []job

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
