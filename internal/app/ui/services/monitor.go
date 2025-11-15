package services

import (
	"fmt"
	"math"
	"time"

	"github.com/shirou/gopsutil/v4/process"
)

func (m *Model) updateProcessStats() {
	for _, service := range m.services {
		if service.Monitor.PID > 0 && service.Status != StatusStopped {
			if service.Monitor.PID > math.MaxInt32 {
				continue
			}

			proc, err := process.NewProcess(int32(service.Monitor.PID)) // #nosec G115 -- PID range checked above
			if err != nil {
				continue
			}

			cpuPercent, err := proc.CPUPercent()
			if err == nil {
				service.Monitor.CPU = cpuPercent
			}

			memInfo, err := proc.MemoryInfo()
			if err == nil {
				service.Monitor.MEM = float64(memInfo.RSS) / 1024 / 1024
			}
		}
	}
}

func (m Model) getUptimeRaw(service *ServiceState) string {
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

func (m Model) getCPURaw(service *ServiceState) string {
	if service.Status == StatusStopped || service.Monitor.PID == 0 {
		return ""
	}

	return fmt.Sprintf("%.1f%%", service.Monitor.CPU)
}

func (m Model) getMemRaw(service *ServiceState) string {
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
