package monitor

import (
	"context"
	"math"
	"sync"
	"time"

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

type cpuState struct {
	createTime int64
	total      float64
	time       time.Time
}

// monitor implements the Monitor interface
type monitor struct {
	mu   sync.Mutex
	prev map[int32]cpuState
}

// NewMonitor creates a new Monitor instance
func NewMonitor() Monitor {
	return &monitor{
		prev: make(map[int32]cpuState),
	}
}

// GetStats retrieves CPU and memory statistics for a process
func (m *monitor) GetStats(ctx context.Context, pid int) (Stats, error) {
	if pid <= 0 || pid > math.MaxInt32 {
		return Stats{}, nil
	}

	pid32 := int32(pid) // #nosec G115 -- PID range checked above

	proc, err := process.NewProcessWithContext(ctx, pid32)
	if err != nil {
		m.evict(pid32)

		return Stats{}, err
	}

	stats := Stats{}

	times, timesErr := proc.TimesWithContext(ctx)
	createTime, ctErr := proc.CreateTimeWithContext(ctx)

	switch {
	case timesErr != nil || ctErr != nil:
		m.evict(pid32)
	default:
		stats.CPU = m.cpuPercent(pid32, createTime, times.User+times.System)
	}

	memInfo, err := proc.MemoryInfoWithContext(ctx)
	if err == nil {
		stats.MEM = float64(memInfo.RSS) / 1024 / 1024
	}

	return stats, nil
}

func (m *monitor) evict(pid int32) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.prev, pid)
}

func (m *monitor) cpuPercent(pid int32, createTime int64, total float64) float64 {
	now := time.Now()

	m.mu.Lock()
	defer m.mu.Unlock()

	prev, exists := m.prev[pid]
	m.prev[pid] = cpuState{createTime: createTime, total: total, time: now}

	if !exists || prev.createTime != createTime {
		return 0
	}

	delta := total - prev.total
	elapsed := now.Sub(prev.time).Seconds()

	if delta <= 0 || elapsed <= 0 {
		return 0
	}

	return (delta / elapsed) * 100
}
