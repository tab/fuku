package logs

const (
	// MaxEntries is the maximum number of log entries to keep in memory
	MaxEntries = 10000
)

// Entry represents a single log entry from a service
type Entry struct {
	ServiceName string
	Text        string
}

// Manager defines the interface for log management
type Manager interface {
	AddLog(serviceName, text string)
	GetLogs() []Entry
	GetFilteredLogs() []Entry
	IsFiltered(serviceName string) bool
	AddFilter(serviceName string)
	RemoveFilter(serviceName string)
	ToggleFilter(serviceName string)
	AddAllFilters(serviceNames []string)
	GetFilterCount() int
	GetFilteredNames() map[string]bool
}

// manager manages service logs and filtering
type manager struct {
	logs          []Entry
	filteredNames map[string]bool
}

// NewManager creates a new log manager
func NewManager() Manager {
	return &manager{
		logs:          []Entry{},
		filteredNames: make(map[string]bool),
	}
}

// AddLog adds a new log entry, maintaining the maximum size
func (m *manager) AddLog(serviceName, text string) {
	m.logs = append(m.logs, Entry{
		ServiceName: serviceName,
		Text:        text,
	})
	if len(m.logs) > MaxEntries {
		m.logs = m.logs[len(m.logs)-MaxEntries:]
	}
}

// GetLogs returns all log entries
func (m *manager) GetLogs() []Entry {
	return m.logs
}

// GetFilteredLogs returns only logs for filtered services
func (m *manager) GetFilteredLogs() []Entry {
	filtered := make([]Entry, 0, len(m.logs))
	for _, entry := range m.logs {
		if m.filteredNames[entry.ServiceName] {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

// IsFiltered checks if a service is in the filter
func (m *manager) IsFiltered(serviceName string) bool {
	return m.filteredNames[serviceName]
}

// AddFilter adds a service to the filter
func (m *manager) AddFilter(serviceName string) {
	m.filteredNames[serviceName] = true
}

// RemoveFilter removes a service from the filter
func (m *manager) RemoveFilter(serviceName string) {
	delete(m.filteredNames, serviceName)
}

// ToggleFilter toggles a service filter
func (m *manager) ToggleFilter(serviceName string) {
	if m.filteredNames[serviceName] {
		delete(m.filteredNames, serviceName)
	} else {
		m.filteredNames[serviceName] = true
	}
}

// AddAllFilters adds all provided service names to the filter
func (m *manager) AddAllFilters(serviceNames []string) {
	for _, name := range serviceNames {
		m.filteredNames[name] = true
	}
}

// GetFilterCount returns the number of filtered services
func (m *manager) GetFilterCount() int {
	return len(m.filteredNames)
}

// GetFilteredNames returns a copy of the filtered names map
func (m *manager) GetFilteredNames() map[string]bool {
	names := make(map[string]bool, len(m.filteredNames))
	for k, v := range m.filteredNames {
		names[k] = v
	}
	return names
}
