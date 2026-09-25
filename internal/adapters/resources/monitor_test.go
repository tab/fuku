package resources

import (
	"math"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_NewProcessMonitor(t *testing.T) {
	m := NewProcessMonitor()

	assert.NotNil(t, m)
	assert.Empty(t, m.prev)
}

func Test_ProcessMonitor_GetStats_OutOfRangePID(t *testing.T) {
	m := NewProcessMonitor()

	pid := math.MaxInt32 + 1

	stats, err := m.GetStats(t.Context(), pid)

	require.NoError(t, err)
	assert.Equal(t, Stats{}, stats)
}

func Test_ProcessMonitor_GetStats_CurrentProcess(t *testing.T) {
	m := NewProcessMonitor()

	pid := os.Getpid()

	first, err := m.GetStats(t.Context(), pid)
	require.NoError(t, err)

	second, err := m.GetStats(t.Context(), pid)
	require.NoError(t, err)

	assert.InDelta(t, 0.0, first.CPU, 0.001, "the first sample has no delta to measure")
	assert.GreaterOrEqual(t, second.CPU, 0.0)
	assert.Greater(t, second.MEM, 0.0)
	assert.Positive(t, second.RawMEM)
}

func Test_ProcessMonitor_GetStats_NonExistentProcess(t *testing.T) {
	m := NewProcessMonitor()

	_, err := m.GetStats(t.Context(), 999999999)

	require.Error(t, err)
}

func Test_ProcessMonitor_cpuPercent(t *testing.T) {
	m := NewProcessMonitor()
	m.prev[2] = cpuState{createTime: 100, total: 1.0, time: time.Now()}
	m.prev[3] = cpuState{createTime: 100, total: 1.0, time: time.Now()}
	m.prev[4] = cpuState{createTime: 100, total: 1.0, time: time.Now().Add(-time.Second)}

	tests := []struct {
		name       string
		pid        int32
		createTime int64
		total      float64
		expected   float64
	}{
		{
			name:       "a first reading has no delta",
			pid:        1,
			createTime: 100,
			total:      1.0,
			expected:   0,
		},
		{
			name:       "a restarted process has no delta",
			pid:        2,
			createTime: 200,
			total:      1.5,
			expected:   0,
		},
		{
			name:       "no CPU spent reads zero",
			pid:        3,
			createTime: 100,
			total:      1.0,
			expected:   0,
		},
		{
			name:       "CPU spent reads its share of the elapsed time",
			pid:        4,
			createTime: 100,
			total:      1.5,
			expected:   50,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := m.cpuPercent(tt.pid, tt.createTime, tt.total)

			assert.InDelta(t, tt.expected, result, 1)
		})
	}
}
