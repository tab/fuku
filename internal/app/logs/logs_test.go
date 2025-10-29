package logs

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_NewManager(t *testing.T) {
	m := NewManager()
	assert.NotNil(t, m)

	instance, ok := m.(*manager)
	assert.True(t, ok)
	assert.NotNil(t, instance.logs)
	assert.NotNil(t, instance.filteredNames)
	assert.Equal(t, 0, len(instance.logs))
	assert.Equal(t, 0, len(instance.filteredNames))
}

func Test_AddLog(t *testing.T) {
	m := NewManager()

	m.AddLog("service1", "log line 1")
	m.AddLog("service1", "log line 2")
	m.AddLog("service2", "log line 3")

	logs := m.GetLogs()
	assert.Len(t, logs, 3)
	assert.Equal(t, "service1", logs[0].ServiceName)
	assert.Equal(t, "log line 1", logs[0].Text)
	assert.Equal(t, "service2", logs[2].ServiceName)
	assert.Equal(t, "log line 3", logs[2].Text)
}

func Test_AddLog_MaxEntries(t *testing.T) {
	m := NewManager()

	// Add more than MaxEntries
	for i := 0; i < MaxEntries+100; i++ {
		m.AddLog("service", "log line")
	}

	logs := m.GetLogs()
	assert.Equal(t, MaxEntries, len(logs))
}

func Test_GetLogs(t *testing.T) {
	m := NewManager()

	m.AddLog("service1", "line 1")
	m.AddLog("service2", "line 2")

	logs := m.GetLogs()
	assert.Len(t, logs, 2)
}

func Test_GetFilteredLogs(t *testing.T) {
	m := NewManager()

	m.AddLog("service1", "line 1")
	m.AddLog("service2", "line 2")
	m.AddLog("service1", "line 3")
	m.AddLog("service3", "line 4")

	m.AddFilter("service1")
	m.AddFilter("service3")

	filtered := m.GetFilteredLogs()
	assert.Len(t, filtered, 3)
	assert.Equal(t, "service1", filtered[0].ServiceName)
	assert.Equal(t, "line 1", filtered[0].Text)
	assert.Equal(t, "service1", filtered[1].ServiceName)
	assert.Equal(t, "line 3", filtered[1].Text)
	assert.Equal(t, "service3", filtered[2].ServiceName)
	assert.Equal(t, "line 4", filtered[2].Text)
}

func Test_GetFilteredLogs_NoFilters(t *testing.T) {
	m := NewManager()

	m.AddLog("service1", "line 1")
	m.AddLog("service2", "line 2")

	filtered := m.GetFilteredLogs()
	assert.Len(t, filtered, 0)
}

func Test_IsFiltered(t *testing.T) {
	m := NewManager()

	assert.False(t, m.IsFiltered("service1"))

	m.AddFilter("service1")
	assert.True(t, m.IsFiltered("service1"))
	assert.False(t, m.IsFiltered("service2"))
}

func Test_AddFilter(t *testing.T) {
	m := NewManager()

	m.AddFilter("service1")
	m.AddFilter("service2")

	assert.True(t, m.IsFiltered("service1"))
	assert.True(t, m.IsFiltered("service2"))
	assert.Equal(t, 2, m.GetFilterCount())
}

func Test_RemoveFilter(t *testing.T) {
	m := NewManager()

	m.AddFilter("service1")
	m.AddFilter("service2")
	assert.Equal(t, 2, m.GetFilterCount())

	m.RemoveFilter("service1")
	assert.False(t, m.IsFiltered("service1"))
	assert.True(t, m.IsFiltered("service2"))
	assert.Equal(t, 1, m.GetFilterCount())
}

func Test_ToggleFilter(t *testing.T) {
	m := NewManager()

	m.ToggleFilter("service1")
	assert.True(t, m.IsFiltered("service1"))

	m.ToggleFilter("service1")
	assert.False(t, m.IsFiltered("service1"))

	m.ToggleFilter("service1")
	assert.True(t, m.IsFiltered("service1"))
}

func Test_AddAllFilters(t *testing.T) {
	m := NewManager()

	services := []string{"service1", "service2", "service3"}
	m.AddAllFilters(services)

	assert.Equal(t, 3, m.GetFilterCount())
	assert.True(t, m.IsFiltered("service1"))
	assert.True(t, m.IsFiltered("service2"))
	assert.True(t, m.IsFiltered("service3"))
}

func Test_GetFilterCount(t *testing.T) {
	m := NewManager()

	assert.Equal(t, 0, m.GetFilterCount())

	m.AddFilter("service1")
	assert.Equal(t, 1, m.GetFilterCount())

	m.AddFilter("service2")
	assert.Equal(t, 2, m.GetFilterCount())

	m.RemoveFilter("service1")
	assert.Equal(t, 1, m.GetFilterCount())
}

func Test_GetFilteredNames(t *testing.T) {
	m := NewManager()

	m.AddFilter("service1")
	m.AddFilter("service2")

	names := m.GetFilteredNames()
	assert.Len(t, names, 2)
	assert.True(t, names["service1"])
	assert.True(t, names["service2"])

	// Verify it's a copy, not the original
	names["service3"] = true
	assert.Equal(t, 2, m.GetFilterCount())
}
