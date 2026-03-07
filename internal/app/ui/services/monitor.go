package services

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	tea "charm.land/bubbletea/v2"

	"fuku/internal/app/monitor"
	"fuku/internal/app/ui/components"
)

// ServiceStats holds CPU and memory statistics for a service
type ServiceStats struct {
	CPU float64
	MEM float64
}

// statsUpdateMsg is sent by the background worker with batched stats
type statsUpdateMsg struct {
	Stats      map[string]ServiceStats
	AppCPU     float64
	AppMEM     float64
	NextOffset int
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
	if m.isServiceMonitored(service) {
		return formatCPU(service.Monitor.CPU)
	}

	return ""
}

// getMem returns formatted memory usage for a service
func (m *Model) getMem(service *ServiceState) string {
	if m.isServiceMonitored(service) {
		return formatMEM(service.Monitor.MEM)
	}

	return ""
}

// getPID returns the process ID string for a running service
func (m *Model) getPID(service *ServiceState) string {
	if m.isServiceMonitored(service) {
		return strconv.Itoa(service.Monitor.PID)
	}

	return ""
}

// isServiceMonitored returns true if service has valid monitoring data
func (m *Model) isServiceMonitored(service *ServiceState) bool {
	return service.Status == StatusRunning && service.Monitor.PID != 0
}

// pad formats a number with leading zero
func pad(n int) string {
	return fmt.Sprintf("%02d", n)
}

// monitoredService holds a name+PID pair for stats collection
type monitoredService struct {
	name string
	pid  int
}

// statsWorkerCmd schedules a single stats collection and returns the result
func statsWorkerCmd(ctx context.Context, mon monitor.Monitor, services []monitoredService, offset int) tea.Cmd {
	return tea.Tick(components.StatsPollingInterval, func(t time.Time) tea.Msg {
		batchCtx, cancel := context.WithTimeout(ctx, components.StatsBatchTimeout)
		defer cancel()

		stats, nextOffset := collectStats(batchCtx, mon, services, offset)
		appCPU, appMEM := collectAppStats(batchCtx, mon)

		return statsUpdateMsg{
			Stats:      stats,
			AppCPU:     appCPU,
			AppMEM:     appMEM,
			NextOffset: nextOffset,
		}
	})
}

// collectStats polls services serially with round-robin from offset
func collectStats(ctx context.Context, mon monitor.Monitor, services []monitoredService, offset int) (map[string]ServiceStats, int) {
	if len(services) == 0 {
		return nil, 0
	}

	stats := make(map[string]ServiceStats)

	if offset >= len(services) {
		offset = 0
	}

	for i := range services {
		if ctx.Err() != nil {
			return stats, (offset + i) % len(services)
		}

		idx := (offset + i) % len(services)
		svc := services[idx]

		callCtx, cancel := context.WithTimeout(ctx, components.StatsCallTimeout)
		result, err := mon.GetStats(callCtx, svc.pid)

		cancel()

		if err != nil && ctx.Err() != nil {
			return stats, idx
		}

		if err != nil {
			continue
		}

		stats[svc.name] = ServiceStats{CPU: result.CPU, MEM: result.MEM}
	}

	return stats, (offset + len(services)) % len(services)
}

// buildMonitoredList builds a deterministic list of monitored services using tier order
func (m *Model) buildMonitoredList() []monitoredService {
	services := make([]monitoredService, 0, len(m.state.services))

	for _, tier := range m.state.tiers {
		for _, name := range tier.Services {
			service, exists := m.state.services[name]
			if !exists {
				continue
			}

			if m.isServiceMonitored(service) {
				services = append(services, monitoredService{name: name, pid: service.Monitor.PID})
			}
		}
	}

	return services
}

// collectAppStats collects CPU and memory stats for the fuku process itself
func collectAppStats(ctx context.Context, mon monitor.Monitor) (cpu float64, mem float64) {
	stats, err := mon.GetStats(ctx, os.Getpid())
	if err != nil {
		return 0, 0
	}

	return stats.CPU, stats.MEM
}
