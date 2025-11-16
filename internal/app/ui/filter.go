package ui

// LogFilter provides thread-safe access to service log visibility settings
type LogFilter interface {
	// Set updates the visibility state for a service
	Set(service string, enabled bool)
	// IsEnabled returns whether logs are enabled for a service
	IsEnabled(service string) bool
	// All returns a copy of all filter settings
	All() map[string]bool
	// EnableAll enables logs for all specified services
	EnableAll(services []string)
}

type logFilter struct {
	enabled map[string]bool
}

// NewLogFilter creates a new log filter
func NewLogFilter() LogFilter {
	return &logFilter{
		enabled: make(map[string]bool),
	}
}

func (f *logFilter) Set(service string, enabled bool) {
	f.enabled[service] = enabled
}

func (f *logFilter) IsEnabled(service string) bool {
	return f.enabled[service]
}

func (f *logFilter) All() map[string]bool {
	result := make(map[string]bool, len(f.enabled))
	for k, v := range f.enabled {
		result[k] = v
	}

	return result
}

func (f *logFilter) EnableAll(services []string) {
	for _, service := range services {
		f.enabled[service] = true
	}
}
