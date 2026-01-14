package runtime

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testFilter is a simple LogFilter implementation for testing
type testFilter struct {
	enabled map[string]bool
}

func newTestFilter() *testFilter {
	return &testFilter{enabled: make(map[string]bool)}
}

func (f *testFilter) IsEnabled(service string) bool {
	enabled, exists := f.enabled[service]
	if !exists {
		return true
	}

	return enabled
}

func (f *testFilter) Set(service string, enabled bool) {
	f.enabled[service] = enabled
}

func Test_LogWriter_Start_CreatesFile(t *testing.T) {
	_ = os.Remove(LogFilePath)

	eb := NewEventBus(10)
	defer eb.Close()

	filter := newTestFilter()
	lw := NewLogWriter(eb, filter)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	lw.Start(ctx)
	defer lw.Close()

	time.Sleep(10 * time.Millisecond)

	_, err := os.Stat(LogFilePath)
	assert.NoError(t, err, "log file should exist after Start")
}

func Test_LogWriter_Close_RemovesFile(t *testing.T) {
	_ = os.Remove(LogFilePath)

	eb := NewEventBus(10)
	defer eb.Close()

	filter := newTestFilter()
	lw := NewLogWriter(eb, filter)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	lw.Start(ctx)

	time.Sleep(10 * time.Millisecond)

	err := lw.Close()
	require.NoError(t, err)

	_, err = os.Stat(LogFilePath)
	assert.True(t, os.IsNotExist(err), "log file should be removed after Close")
}

func Test_LogWriter_WritesLogEvents(t *testing.T) {
	_ = os.Remove(LogFilePath)

	eb := NewEventBus(10)
	defer eb.Close()

	filter := newTestFilter()
	lw := NewLogWriter(eb, filter)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	lw.Start(ctx)
	defer lw.Close()

	eb.Publish(Event{
		Type: EventLogLine,
		Data: LogLineData{Service: "test-service", Tier: "default", Stream: "STDOUT", Message: "hello world"},
	})

	time.Sleep(50 * time.Millisecond)

	content, err := os.ReadFile(LogFilePath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "hello world")
}

func Test_LogWriter_FiltersDisabledServices(t *testing.T) {
	_ = os.Remove(LogFilePath)

	eb := NewEventBus(10)
	defer eb.Close()

	filter := newTestFilter()
	filter.Set("disabled-service", false)

	lw := NewLogWriter(eb, filter)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	lw.Start(ctx)
	defer lw.Close()

	eb.Publish(Event{
		Type: EventLogLine,
		Data: LogLineData{Service: "disabled-service", Tier: "default", Stream: "STDOUT", Message: "should not appear"},
	})
	eb.Publish(Event{
		Type: EventLogLine,
		Data: LogLineData{Service: "enabled-service", Tier: "default", Stream: "STDOUT", Message: "should appear"},
	})

	time.Sleep(50 * time.Millisecond)

	content, err := os.ReadFile(LogFilePath)
	require.NoError(t, err)
	assert.NotContains(t, string(content), "should not appear")
	assert.Contains(t, string(content), "should appear")
}

func Test_LogWriter_IgnoresNonLogEvents(t *testing.T) {
	_ = os.Remove(LogFilePath)

	eb := NewEventBus(10)
	defer eb.Close()

	filter := newTestFilter()
	lw := NewLogWriter(eb, filter)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	lw.Start(ctx)
	defer lw.Close()

	eb.Publish(Event{Type: EventServiceStarting, Data: ServiceStartingData{Service: "test"}})
	eb.Publish(Event{Type: EventPhaseChanged, Data: PhaseChangedData{Phase: PhaseRunning}})

	time.Sleep(50 * time.Millisecond)

	content, err := os.ReadFile(LogFilePath)
	require.NoError(t, err)
	assert.Empty(t, string(content), "file should be empty when only non-log events are published")
}

func Test_LogWriter_PreservesOrder(t *testing.T) {
	_ = os.Remove(LogFilePath)

	eb := NewEventBus(10)
	defer eb.Close()

	filter := newTestFilter()
	lw := NewLogWriter(eb, filter)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	lw.Start(ctx)
	defer lw.Close()

	for i := 1; i <= 5; i++ {
		eb.Publish(Event{
			Type: EventLogLine,
			Data: LogLineData{Service: "test", Message: fmt.Sprintf("line %d", i)},
		})
	}

	time.Sleep(50 * time.Millisecond)

	content, err := os.ReadFile(LogFilePath)
	require.NoError(t, err)

	expected := "line 1\nline 2\nline 3\nline 4\nline 5\n"
	assert.Equal(t, expected, string(content))
}

func Test_LogWriter_Close_WithoutStart(t *testing.T) {
	eb := NewEventBus(10)
	defer eb.Close()

	filter := newTestFilter()
	lw := NewLogWriter(eb, filter)

	err := lw.Close()
	assert.NoError(t, err, "Close should not error if Start was never called")
}

func Test_LogWriter_RuntimeFilterChange(t *testing.T) {
	_ = os.Remove(LogFilePath)

	eb := NewEventBus(10)
	defer eb.Close()

	filter := newTestFilter()
	lw := NewLogWriter(eb, filter)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	lw.Start(ctx)
	defer lw.Close()

	eb.Publish(Event{
		Type: EventLogLine,
		Data: LogLineData{Service: "service-a", Message: "before filter"},
	})

	time.Sleep(50 * time.Millisecond)

	filter.Set("service-a", false)

	eb.Publish(Event{
		Type: EventLogLine,
		Data: LogLineData{Service: "service-a", Message: "after filter"},
	})

	time.Sleep(50 * time.Millisecond)

	content, err := os.ReadFile(LogFilePath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "before filter")
	assert.NotContains(t, string(content), "after filter")
}
