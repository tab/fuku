package services

import (
	"context"
	"fmt"
	"os"
	"strconv"

	tea "charm.land/bubbletea/v2"

	"fuku/internal/app/ui/components"
)

// appStatsMsg contains sampled CPU and memory for the fuku process
type appStatsMsg struct {
	cpu float64
	mem float64
}

// updateBlinkAnimations updates blink state for services in transition states
func (m *Model) updateBlinkAnimations() bool {
	hasActiveBlinking := false

	for _, service := range m.state.services {
		if service.Blink == nil {
			continue
		}

		switch service.Status {
		case StatusStarting, StatusStopping, StatusRestarting:
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
	if service.Status.IsStartable() || service.StartTime.IsZero() || m.state.now.IsZero() {
		return ""
	}

	elapsed := m.state.now.Sub(service.StartTime)
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
		return formatCPU(service.CPU)
	}

	return ""
}

// getMem returns formatted memory usage for a service
func (m *Model) getMem(service *ServiceState) string {
	if m.isServiceMonitored(service) {
		return formatMEM(service.MEM)
	}

	return ""
}

// getPID returns the process ID string for a running service
func (m *Model) getPID(service *ServiceState) string {
	if m.isServiceMonitored(service) {
		return strconv.Itoa(service.PID)
	}

	return ""
}

// isServiceMonitored returns true if service has valid monitoring data
func (m *Model) isServiceMonitored(service *ServiceState) bool {
	return service.Status == StatusRunning && service.PID != 0
}

// sampleAppStatsCmd returns a command that samples fuku process stats off the UI thread
func (m Model) sampleAppStatsCmd() tea.Cmd {
	ctx := m.ctx
	mon := m.monitor

	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(ctx, components.StatsCallTimeout)
		defer cancel()

		stats, err := mon.GetStats(ctx, os.Getpid())
		if err != nil {
			return appStatsMsg{}
		}

		return appStatsMsg{cpu: stats.CPU, mem: stats.MEM}
	}
}

// updateAPIHealth derives the API status from live state
func (m *Model) updateAPIHealth() {
	if m.api == nil || m.api.Address() == "" {
		m.state.apiStatus = apiStatusDown

		return
	}

	if m.store.IsResolved() {
		m.state.apiStatus = apiStatusReady
	} else {
		m.state.apiStatus = apiStatusDown
	}
}

// pad formats a number with leading zero
func pad(n int) string {
	return fmt.Sprintf("%02d", n)
}
