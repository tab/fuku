package monitor

import (
	"context"
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
	GetStats(ctx context.Context, pid int) (Stats, error)
}

// monitor implements the Monitor interface
type monitor struct{}

// NewMonitor creates a new Monitor instance
func NewMonitor() Monitor {
	return &monitor{}
}

// GetStats retrieves CPU and memory statistics for a process
func (m *monitor) GetStats(ctx context.Context, pid int) (Stats, error) {
	if pid <= 0 || pid > math.MaxInt32 {
		return Stats{}, nil
	}

	proc, err := process.NewProcessWithContext(ctx, int32(pid)) // #nosec G115 -- PID range checked above
	if err != nil {
		return Stats{}, err
	}

	stats := Stats{}

	cpuPercent, err := proc.CPUPercentWithContext(ctx)
	if err == nil {
		stats.CPU = cpuPercent
	}

	memInfo, err := proc.MemoryInfoWithContext(ctx)
	if err == nil {
		stats.MEM = float64(memInfo.RSS) / 1024 / 1024
	}

	return stats, nil
}
