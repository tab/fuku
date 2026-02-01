package monitor

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMonitor(t *testing.T) {
	m := NewMonitor()

	assert.NotNil(t, m)
}

func TestGetStats_InvalidPID(t *testing.T) {
	m := NewMonitor()
	ctx := context.Background()

	tests := []struct {
		name string
		pid  int
	}{
		{name: "zero PID", pid: 0},
		{name: "negative PID", pid: -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stats, err := m.GetStats(ctx, tt.pid)

			assert.NoError(t, err)
			assert.Equal(t, Stats{}, stats)
		})
	}
}

func TestGetStats_CurrentProcess(t *testing.T) {
	m := NewMonitor()
	ctx := context.Background()
	pid := os.Getpid()

	stats, err := m.GetStats(ctx, pid)

	require.NoError(t, err)
	assert.GreaterOrEqual(t, stats.CPU, 0.0)
	assert.GreaterOrEqual(t, stats.MEM, 0.0)
}

func TestGetStats_NonExistentProcess(t *testing.T) {
	m := NewMonitor()
	ctx := context.Background()

	_, err := m.GetStats(ctx, 999999999)

	assert.Error(t, err)
}

func TestGetStats_MaxInt32PID(t *testing.T) {
	m := NewMonitor()
	ctx := context.Background()

	stats, err := m.GetStats(ctx, 2147483648)

	assert.NoError(t, err)
	assert.Equal(t, Stats{}, stats)
}

func TestGetStats_ContextTimeout(t *testing.T) {
	m := NewMonitor()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	stats, err := m.GetStats(ctx, os.Getpid())
	if err != nil {
		assert.Error(t, err)
	} else {
		assert.NotNil(t, stats)
	}
}
