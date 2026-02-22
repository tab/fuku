package preflight

import (
	"context"
	"os"
	"sort"
	"sync"
	"syscall"
	"time"

	"github.com/shirou/gopsutil/v4/process"

	"fuku/internal/app/bus"
	"fuku/internal/app/worker"
	"fuku/internal/config"
	"fuku/internal/config/logger"
)

// Result represents a process killed during preflight
type Result struct {
	Service string
	Name    string
	PID     int32
}

// Preflight handles pre-start cleanup of orphaned processes
type Preflight interface {
	Cleanup(ctx context.Context, dirs map[string]string) ([]Result, error)
}

// entry holds information about a running process
type entry struct {
	name string
	dir  string
	pid  int32
}

// match holds a process matched to a service
type match struct {
	service string
	entry   entry
}

type scanFunc func() ([]entry, error)
type killFunc func(pid int32) error

type preflight struct {
	scan scanFunc
	kill killFunc
	bus  bus.Bus
	pool worker.Pool
	log  logger.Logger
}

// NewPreflight creates a new Preflight instance
func NewPreflight(cfg *config.Config, bus bus.Bus, log logger.Logger) Preflight {
	return &preflight{
		scan: scan,
		kill: kill,
		bus:  bus,
		pool: worker.NewWorkerPool(cfg),
		log:  log.WithComponent("PREFLIGHT"),
	}
}

// Cleanup scans running processes and kills any whose working directory matches a service directory
func (p *preflight) Cleanup(ctx context.Context, dirs map[string]string) ([]Result, error) {
	if len(dirs) == 0 {
		return nil, nil
	}

	p.bus.Publish(bus.Message{
		Type:     bus.EventPreflightStarted,
		Data:     bus.PreflightStarted{Services: sortedKeys(dirs)},
		Critical: true,
	})

	matches, err := p.matchProcesses(dirs)
	if err != nil {
		p.log.Warn().Err(err).Msg("Failed to scan processes")
		p.bus.Publish(bus.Message{
			Type:     bus.EventPreflightComplete,
			Data:     bus.PreflightComplete{Killed: 0},
			Critical: true,
		})

		return nil, nil
	}

	results := p.killMatches(ctx, matches)

	p.bus.Publish(bus.Message{
		Type:     bus.EventPreflightComplete,
		Data:     bus.PreflightComplete{Killed: len(results)},
		Critical: true,
	})

	return results, nil
}

// matchProcesses scans running processes and returns those matching service directories
func (p *preflight) matchProcesses(dirs map[string]string) ([]match, error) {
	processes, err := p.scan()
	if err != nil {
		return nil, err
	}

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

	return matches, nil
}

// killMatches kills matched processes concurrently using the worker pool
func (p *preflight) killMatches(ctx context.Context, matches []match) []Result {
	if len(matches) == 0 {
		return nil
	}

	var (
		mu      sync.Mutex
		wg      sync.WaitGroup
		results = make([]Result, 0, len(matches))
	)

	for _, m := range matches {
		if err := p.pool.Acquire(ctx); err != nil {
			p.log.Warn().Err(err).Msg("Context cancelled, stopping preflight kills")

			break
		}

		wg.Add(1)

		go func(m match) {
			defer wg.Done()
			defer p.pool.Release()

			p.log.Info().Msgf("Killing process '%s' (PID: %d) in '%s' for service '%s'", m.entry.name, m.entry.pid, m.entry.dir, m.service)

			p.bus.Publish(bus.Message{
				Type: bus.EventPreflightKill,
				Data: bus.PreflightKill{
					Service: m.service,
					PID:     int(m.entry.pid),
					Name:    m.entry.name,
				},
			})

			if err := p.kill(m.entry.pid); err != nil {
				p.log.Warn().Err(err).Msgf("Failed to kill process %d", m.entry.pid)
			}

			mu.Lock()

			results = append(results, Result{
				Service: m.service,
				Name:    m.entry.name,
				PID:     m.entry.pid,
			})

			mu.Unlock()
		}(m)
	}

	wg.Wait()

	return results
}

func scan() ([]entry, error) {
	processes, err := process.Processes()
	if err != nil {
		return nil, err
	}

	results := make([]entry, 0, len(processes))

	for _, p := range processes {
		dir, err := p.Cwd()
		if err != nil {
			continue
		}

		name, _ := p.Name()
		results = append(results, entry{
			name: name,
			dir:  dir,
			pid:  p.Pid,
		})
	}

	return results, nil
}

func kill(pid int32) error {
	// Try SIGTERM to process group first (catches child processes)
	if err := syscall.Kill(-int(pid), syscall.SIGTERM); err != nil {
		if err := syscall.Kill(int(pid), syscall.SIGTERM); err != nil {
			return nil
		}
	}

	deadline := time.After(config.PreFlightKillTimeout)

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-deadline:
			_ = syscall.Kill(-int(pid), syscall.SIGKILL)
			_ = syscall.Kill(int(pid), syscall.SIGKILL)

			return nil
		case <-ticker.C:
			if err := syscall.Kill(int(pid), 0); err != nil {
				return nil
			}
		}
	}
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))

	for k := range m {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	return keys
}
