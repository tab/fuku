package session

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/shirou/gopsutil/v4/process"

	"fuku/internal/app/errors"
	"fuku/internal/config"
	"fuku/internal/config/logger"
)

const pidTimeTolerance = 2 * time.Second

// Entry represents a tracked service process
type Entry struct {
	Service   string    `json:"service"`
	PID       int       `json:"pid"`
	StartedAt time.Time `json:"started_at"`
}

// State represents the current session state
type State struct {
	Profile   string    `json:"profile"`
	StartedAt time.Time `json:"started_at"`
	Entries   []Entry   `json:"entries"`
}

// Session defines the interface for session tracking
type Session interface {
	Save(state *State) error
	Load() (*State, error)
	Delete() error
	Add(entry Entry) error
	Remove(service string) error
}

type session struct {
	mu   sync.Mutex
	path string
}

// NewSession creates a new session instance
func NewSession() Session {
	return &session{
		path: config.SessionPath,
	}
}

// Save writes the session state to disk
func (s *session) Save(state *State) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.save(state)
}

// Load reads the session state from disk
func (s *session) Load() (*State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.load()
}

// Delete removes the session file
func (s *session) Delete() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.Remove(s.path); err != nil && !os.IsNotExist(err) {
		return err
	}

	return nil
}

// Add adds or replaces a service entry in the session
func (s *session) Add(entry Entry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	state, err := s.load()
	if err != nil && !errors.Is(err, errors.ErrSessionNotFound) {
		return err
	}

	if state == nil {
		state = &State{
			StartedAt: time.Now(),
		}
	}

	replaced := false

	for i, e := range state.Entries {
		if e.Service == entry.Service {
			state.Entries[i] = entry
			replaced = true

			break
		}
	}

	if !replaced {
		state.Entries = append(state.Entries, entry)
	}

	return s.save(state)
}

// Remove removes a service entry from the session
func (s *session) Remove(service string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	state, err := s.load()
	if err != nil {
		if errors.Is(err, errors.ErrSessionNotFound) {
			return nil
		}

		return err
	}

	filtered := make([]Entry, 0, len(state.Entries))
	for _, e := range state.Entries {
		if e.Service != service {
			filtered = append(filtered, e)
		}
	}

	state.Entries = filtered

	return s.save(state)
}

func (s *session) save(state *State) error {
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("%w: %w", errors.ErrSessionDirCreate, err)
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("%w: %w", errors.ErrSessionFileWrite, err)
	}

	if err := os.WriteFile(s.path, data, 0600); err != nil {
		return fmt.Errorf("%w: %w", errors.ErrSessionFileWrite, err)
	}

	return nil
}

func (s *session) load() (*State, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errors.ErrSessionNotFound
		}

		return nil, fmt.Errorf("%w: %w", errors.ErrSessionFileRead, err)
	}

	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("%w: %w", errors.ErrSessionCorrupted, err)
	}

	return &state, nil
}

// VerifyPID checks if a PID is still running and matches the expected start time
func VerifyPID(entry Entry) bool {
	proc, err := process.NewProcess(int32(entry.PID)) //nolint:gosec // PID values are always within int32 range
	if err != nil {
		return false
	}

	createTime, err := proc.CreateTime()
	if err != nil {
		return false
	}

	procStart := time.UnixMilli(createTime)
	diff := math.Abs(float64(procStart.Sub(entry.StartedAt).Milliseconds()))

	return diff <= float64(pidTimeTolerance.Milliseconds())
}

// KillOrphans terminates orphaned processes from a stale session
func KillOrphans(state *State, log logger.Logger) int {
	killed := 0

	for _, entry := range state.Entries {
		if !VerifyPID(entry) {
			log.Debug().Msgf("Process '%s' (PID %d) is no longer running", entry.Service, entry.PID)

			continue
		}

		log.Info().Msgf("Killing orphaned process '%s' (PID %d)", entry.Service, entry.PID)

		if err := syscall.Kill(-entry.PID, syscall.SIGTERM); err != nil {
			log.Warn().Msgf("Failed to kill process group for '%s' (PID %d): %v", entry.Service, entry.PID, err)

			continue
		}

		killed++
	}

	return killed
}
