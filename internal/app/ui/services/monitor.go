package services

import (
	"fmt"
	"time"
)

func (m *Model) updateProcessStats() {
	for _, service := range m.services {
		if service.Monitor.PID > 0 && service.Status != StatusStopped {
			stats, err := m.monitor.GetStats(service.Monitor.PID)
			if err != nil {
				continue
			}

			service.Monitor.CPU = stats.CPU
			service.Monitor.MEM = stats.MEM
		}
	}
}

func (m Model) getUptime(service *ServiceState) string {
	if service.Status == StatusStopped || service.Monitor.StartTime.IsZero() {
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

func (m Model) getCPU(service *ServiceState) string {
	if service.Status == StatusStopped || service.Monitor.PID == 0 {
		return ""
	}

	return fmt.Sprintf("%.1f%%", service.Monitor.CPU)
}

func (m Model) getMem(service *ServiceState) string {
	if service.Status == StatusStopped || service.Monitor.PID == 0 {
		return ""
	}

	if service.Monitor.MEM < 1024 {
		return fmt.Sprintf("%.0fMB", service.Monitor.MEM)
	}

	return fmt.Sprintf("%.1fGB", service.Monitor.MEM/1024)
}

func pad(n int) string {
	return fmt.Sprintf("%02d", n)
}
