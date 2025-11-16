package monitor

import (
	"math"

	"github.com/shirou/gopsutil/v4/process"
)

// Stats contains process resource statistics
type Stats struct {
	CPU float64
	MEM float64 // in MB
}

// Monitor provides process resource monitoring
type Monitor interface {
	GetStats(pid int) (Stats, error)
}

type monitor struct{}

// NewMonitor creates a new Monitor instance
func NewMonitor() Monitor {
	return &monitor{}
}

func (m *monitor) GetStats(pid int) (Stats, error) {
	if pid <= 0 || pid > math.MaxInt32 {
		return Stats{}, nil
	}

	proc, err := process.NewProcess(int32(pid)) // #nosec G115 -- PID range checked above
	if err != nil {
		return Stats{}, err
	}

	stats := Stats{}

	cpuPercent, err := proc.CPUPercent()
	if err == nil {
		stats.CPU = cpuPercent
	}

	memInfo, err := proc.MemoryInfo()
	if err == nil {
		stats.MEM = float64(memInfo.RSS) / 1024 / 1024
	}

	return stats, nil
}
