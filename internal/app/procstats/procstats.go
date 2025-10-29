package procstats

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

const (
	linuxPageSize = 4096
	kbToBytes     = 1024
)

// Stats represents process resource usage statistics
type Stats struct {
	CPUPercent  float64
	MemoryBytes uint64
}

// Provider defines the interface for retrieving process statistics
type Provider interface {
	GetStats(pid int) Stats
}

// provider implements the Provider interface
type provider struct{}

// NewProvider creates a new process statistics provider
func NewProvider() Provider {
	return &provider{}
}

// GetStats retrieves CPU and memory statistics for a process by PID
func (p *provider) GetStats(pid int) Stats {
	cpu, mem := getProcessStats(pid)
	return Stats{
		CPUPercent:  cpu,
		MemoryBytes: mem,
	}
}

// GetStats is a package-level convenience function
func GetStats(pid int) Stats {
	cpu, mem := getProcessStats(pid)
	return Stats{
		CPUPercent:  cpu,
		MemoryBytes: mem,
	}
}

func getProcessStats(pid int) (float64, uint64) {
	statPath := fmt.Sprintf("/proc/%d/stat", pid)
	data, err := os.ReadFile(statPath)
	if err == nil {
		fields := strings.Fields(string(data))
		if len(fields) >= 24 {
			utime, _ := strconv.ParseUint(fields[13], 10, 64)
			stime, _ := strconv.ParseUint(fields[14], 10, 64)
			rss, _ := strconv.ParseUint(fields[23], 10, 64)

			totalTime := utime + stime
			cpuPercent := float64(totalTime) / 100.0
			memBytes := rss * linuxPageSize

			return cpuPercent, memBytes
		}
	}

	return getProcessStatsMacOS(pid)
}

func getProcessStatsMacOS(pid int) (float64, uint64) {
	// First, try to find child processes (the actual service, not make)
	// #nosec G204 - pid is an integer, not user input
	childCmd := exec.Command("ps", "-A", "-o", "pid=,ppid=,%cpu=,rss=")
	childOutput, err := childCmd.Output()
	if err == nil {
		lines := strings.Split(string(childOutput), "\n")
		var totalCPU float64
		var totalMem uint64

		for _, line := range lines {
			fields := strings.Fields(line)
			if len(fields) >= 4 {
				ppid, _ := strconv.Atoi(fields[1])
				if ppid == pid {
					cpu, _ := strconv.ParseFloat(fields[2], 64)
					rssKB, _ := strconv.ParseUint(fields[3], 10, 64)
					totalCPU += cpu
					totalMem += rssKB * kbToBytes
				}
			}
		}

		if totalMem > 0 {
			return totalCPU, totalMem
		}
	}

	// Fallback to parent process stats
	// #nosec G204 - pid is an integer, not user input
	cmd := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "%cpu=,rss=")
	output, err := cmd.Output()
	if err != nil {
		return 0, 0
	}

	fields := strings.Fields(string(output))
	if len(fields) < 2 {
		return 0, 0
	}

	cpu, _ := strconv.ParseFloat(fields[0], 64)
	rssKB, _ := strconv.ParseUint(fields[1], 10, 64)
	memBytes := rssKB * kbToBytes

	return cpu, memBytes
}

// FormatMemory formats bytes into human-readable format (Bytes, Kb, Mb, Gb)
func FormatMemory(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%5dB", bytes)
	}

	suffixes := []string{"Kb", "Mb", "Gb"}
	value := float64(bytes)

	for i, suffix := range suffixes {
		value /= float64(unit)
		if value < float64(unit) || i == len(suffixes)-1 {
			if value >= 100 {
				return fmt.Sprintf("%4.0f %s", value, suffix)
			} else if value >= 10 {
				return fmt.Sprintf("%4.1f %s", value, suffix)
			}
			return fmt.Sprintf("%4.2f %s", value, suffix)
		}
	}

	return fmt.Sprintf("%4.0f Tb", value)
}

// FormatUptime formats a duration into human-readable uptime (Xh Ym or Xm Ys or Xs)
func FormatUptime(d time.Duration) string {
	d = d.Round(time.Second)

	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60

	if h > 0 {
		return fmt.Sprintf("%2dh%02dm", h, m)
	}
	if m > 0 {
		return fmt.Sprintf("%2dm%02ds", m, s)
	}
	return fmt.Sprintf("  %2ds", s)
}
