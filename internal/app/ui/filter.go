package ui

import "sync"

// LogFilter provides thread-safe access to service log visibility settings
type LogFilter interface {
	Set(service string, enabled bool)
	IsEnabled(service string) bool
	All() map[string]bool
	ToggleAll(services []string)
}

type logFilter struct {
	mu      sync.RWMutex
	enabled map[string]bool
}

// NewLogFilter creates a new log filter
func NewLogFilter() LogFilter {
	return &logFilter{
		enabled: make(map[string]bool),
	}
}

// Set updates the visibility state for a service
func (f *logFilter) Set(service string, enabled bool) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.enabled[service] = enabled
}

// IsEnabled returns whether logs are enabled for a service
func (f *logFilter) IsEnabled(service string) bool {
	f.mu.RLock()
	defer f.mu.RUnlock()

	return f.enabled[service]
}

// All returns a copy of all filter settings
func (f *logFilter) All() map[string]bool {
	f.mu.RLock()
	defer f.mu.RUnlock()

	result := make(map[string]bool, len(f.enabled))
	for k, v := range f.enabled {
		result[k] = v
	}

	return result
}

// ToggleAll toggles all services
func (f *logFilter) ToggleAll(services []string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	isSelected := true

	for _, service := range services {
		if !f.enabled[service] {
			isSelected = false
			break
		}
	}

	for _, service := range services {
		f.enabled[service] = !isSelected
	}
}
