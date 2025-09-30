package procstats

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func Test_NewProvider(t *testing.T) {
	p := NewProvider()
	assert.NotNil(t, p)

	instance, ok := p.(*provider)
	assert.True(t, ok)
	assert.NotNil(t, instance)

	// Test that the provider can get stats
	pid := os.Getpid()
	stats := p.GetStats(pid)
	assert.GreaterOrEqual(t, stats.CPUPercent, 0.0)
	assert.GreaterOrEqual(t, stats.MemoryBytes, uint64(0))
}

func Test_GetStats(t *testing.T) {
	pid := os.Getpid()
	stats := GetStats(pid)

	assert.GreaterOrEqual(t, stats.CPUPercent, 0.0)
	assert.GreaterOrEqual(t, stats.MemoryBytes, uint64(0))
}

func Test_GetStats_InvalidPID(t *testing.T) {
	stats := GetStats(999999)

	assert.Equal(t, 0.0, stats.CPUPercent)
	assert.Equal(t, uint64(0), stats.MemoryBytes)
}

func Test_FormatMemory(t *testing.T) {
	tests := []struct {
		name     string
		bytes    uint64
		expected string
	}{
		{"Bytes", 512, "  512B"},
		{"Kilobytes", 2048, "2.00 Kb"},
		{"Megabytes", 10 * 1024 * 1024, "10.0 Mb"},
		{"Gigabytes", 5 * 1024 * 1024 * 1024, "5.00 Gb"},
		{"Large megabytes", 150 * 1024 * 1024, " 150 Mb"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatMemory(tt.bytes)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_FormatUptime(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		expected string
	}{
		{"Seconds", 30 * time.Second, "  30s"},
		{"Minutes and seconds", 2*time.Minute + 15*time.Second, " 2m15s"},
		{"Hours and minutes", 3*time.Hour + 45*time.Minute, " 3h45m"},
		{"One second", 1 * time.Second, "   1s"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatUptime(tt.duration)
			assert.Equal(t, tt.expected, result)
		})
	}
}
