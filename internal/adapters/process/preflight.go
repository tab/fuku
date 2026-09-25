package process

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"syscall"
	"time"

	"github.com/shirou/gopsutil/v4/process"

	"fuku/internal/contracts"
)

// killTimeout is the grace period after SIGTERM before an orphan is killed
const killTimeout = 2 * time.Second

// running is a process the scan found, with the directory it runs in
type running struct {
	name string
	dir  string
	pid  int32
}

// match is a running process matched to a service
type match struct {
	service string
	entry   running
}

// Pool bounds how many orphans are killed at once
type Pool interface {
	Acquire(ctx context.Context) error
	Release()
}

// Preflight kills the processes still running in service directories before a launch
type Preflight struct {
	publisher contracts.Publisher
	worker    Pool
	log       Logger
}

// NewPreflight creates the preflight cleaner
func NewPreflight(publisher contracts.Publisher, worker Pool, log Logger) *Preflight {
	return &Preflight{
		publisher: publisher,
		worker:    worker,
		log:       log,
	}
}

// Cleanup kills every process running in one of the service directories and reports a scan that could not run
func (p *Preflight) Cleanup(ctx context.Context, dirs map[string]string) error {
	if len(dirs) == 0 {
		return nil
	}

	resolved, err := absDirs(dirs)
	if err != nil {
		return err
	}

	startTime := time.Now()

	p.publishStarted(slices.Sorted(maps.Keys(dirs)))

	processes, err := scan()
	if err != nil {
		p.publishComplete(0, time.Since(startTime))

		return fmt.Errorf("failed to scan processes: %w", err)
	}

	killed := p.killMatches(ctx, matchProcesses(processes, resolved))

	p.publishComplete(killed, time.Since(startTime))

	return nil
}

// matchProcesses returns the running processes matching a service directory
func matchProcesses(processes []running, dirs map[string]string) []match {
	ownPID := int32(os.Getpid()) // #nosec G115 -- PID fits in int32
	matches := make([]match, 0, len(processes))

	for _, proc := range processes {
		if proc.pid == ownPID {
			continue
		}

		for service, dir := range dirs {
			if proc.dir != dir {
				continue
			}

			matches = append(matches, match{service: service, entry: proc})

			break
		}
	}

	return matches
}

// killMatches kills matched processes concurrently using the worker pool and returns how many it went after
func (p *Preflight) killMatches(ctx context.Context, matches []match) int {
	if len(matches) == 0 {
		return 0
	}

	var (
		mu     sync.Mutex
		wg     sync.WaitGroup
		killed int
	)

	for _, m := range matches {
		if err := p.worker.Acquire(ctx); err != nil {
			p.log.Warn("Context cancelled, stopping preflight kills", "error", err)

			break
		}

		wg.Add(1)

		go func(m match) {
			defer wg.Done()
			defer p.worker.Release()

			p.log.Info(fmt.Sprintf("Killing process '%s' (PID: %d) in '%s' for service '%s'", m.entry.name, m.entry.pid, m.entry.dir, m.service))

			p.publishKilled(m.service, m.entry.name, int(m.entry.pid))

			if err := kill(m.entry.pid); err != nil {
				p.log.Warn(fmt.Sprintf("Failed to kill process %d", m.entry.pid), "error", err)
			}

			mu.Lock()
			killed++
			mu.Unlock()
		}(m)
	}

	wg.Wait()

	return killed
}

// absDirs resolves the service directories against the working directory where fuku runs
func absDirs(dirs map[string]string) (map[string]string, error) {
	resolved := make(map[string]string, len(dirs))

	for service, dir := range dirs {
		abs, err := filepath.Abs(dir)
		if err != nil {
			return nil, fmt.Errorf("failed to get working directory: %w", err)
		}

		resolved[service] = abs
	}

	return resolved, nil
}

func scan() ([]running, error) {
	processes, err := process.Processes()
	if err != nil {
		return nil, err
	}

	results := make([]running, 0, len(processes))

	for _, p := range processes {
		dir, err := p.Cwd()
		if err != nil {
			continue
		}

		name, _ := p.Name()
		results = append(results, running{
			name: name,
			dir:  dir,
			pid:  p.Pid,
		})
	}

	return results, nil
}

func kill(pid int32) error {
	proc, _ := os.FindProcess(int(pid))
	//nolint:errcheck // Release only frees the handle FindProcess opened; nothing acts on its error
	defer proc.Release()

	err := signalGroup(int(pid), proc, syscall.SIGTERM)
	if errors.Is(err, os.ErrProcessDone) {
		return nil
	}

	if err != nil {
		return err
	}

	deadline := time.After(killTimeout)

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-deadline:
			// sync: a pid reused since the last liveness poll, at most 100ms ago, would receive the kill
			_ = signalGroup(int(pid), proc, syscall.SIGKILL)
			_ = proc.Kill()

			return nil
		case <-ticker.C:
			if err := syscall.Kill(int(pid), 0); err != nil {
				return nil
			}
		}
	}
}
