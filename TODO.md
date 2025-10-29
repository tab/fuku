# Fuku Codebase Improvement Plan

This document outlines improvements to adopt best practices and enhance the codebase quality, maintainability, and user experience.

---

## Overview

Fuku is a CLI process orchestration tool built with Bubbletea (TUI framework) and Uber FX (dependency injection). This plan focuses on enhancing the existing architecture with better observability, testability, and reusable patterns.

---

## Phase 1: Enhanced Observability (Week 1)

### 1.1 Improve Key Bindings and Add Application Log Viewer

**Priority:** HIGH
**Effort:** 4-5 hours
**Impact:** MAJOR UX improvement - better navigation and observability

**Files to create:**
- `internal/config/logger/buffer.go`
- `internal/config/logger/hook.go`
- `internal/config/logger/buffer_test.go`
- `internal/config/logger/hook_test.go`

**Files to modify:**
- `internal/config/logger/logger.go`
- `internal/config/logger/module.go`
- `internal/app/cli/run.go`

**Current state:**
- Service logs use simple slice buffer (10k entries)
- Press `l` to toggle between Services and Service Logs views
- Application logs only go to console or file
- No way to see application debug logs in TUI

**Target state:**
- Ring buffer captures last 1000 application log entries
- Press **`Tab`** to switch between Services view and Service Logs view (changed from `l`)
- Press **`l`** for Application Log overlay/modal (consistent with CMT)
- Service logs keep existing slice buffer (already efficient)
- Live tail of app logs with color-coded levels

**Key Binding Changes:**
| Key | Old Behavior | New Behavior |
|-----|-------------|--------------|
| `Tab` | _(unused)_ | Toggle Services ↔ Service Logs |
| `l` | Toggle Services ↔ Service Logs | Toggle Application Logs (overlay) |
| `space` | Toggle filter | _(unchanged)_ |
| `a` | Filter all | _(unchanged)_ |
| `s` | Stop/Start service | _(unchanged)_ |
| `r` | Restart service | _(unchanged)_ |

**Three Views:**
1. **Services View** (default) - Service list with stats, CPU, memory
2. **Service Logs View** (Tab) - stdout/stderr from selected services
3. **Application Logs** (`l`) - Internal Fuku logs as overlay on any view

**Implementation steps:**

1. **Create `internal/config/logger/buffer.go`:**

```go
package logger

import (
    "sync"
    "time"
)

const maxBufferSize = 1000

// LogEntry represents a single log entry
type LogEntry struct {
    Level     string
    Timestamp time.Time
    Message   string
    Fields    map[string]interface{}
}

// LogBuffer is a thread-safe ring buffer for log entries
type LogBuffer struct {
    entries []LogEntry
    index   int
    size    int
    mu      sync.RWMutex
}

// NewLogBuffer creates a new log buffer with max size
func NewLogBuffer() *LogBuffer {
    return &LogBuffer{
        entries: make([]LogEntry, maxBufferSize),
        index:   0,
        size:    0,
    }
}

// Add adds a log entry to the buffer (thread-safe)
func (b *LogBuffer) Add(entry LogEntry) {
    b.mu.Lock()
    defer b.mu.Unlock()

    b.entries[b.index] = entry
    b.index = (b.index + 1) % maxBufferSize

    if b.size < maxBufferSize {
        b.size++
    }
}

// GetEntries returns all entries in chronological order (thread-safe)
func (b *LogBuffer) GetEntries() []LogEntry {
    b.mu.RLock()
    defer b.mu.RUnlock()

    if b.size == 0 {
        return []LogEntry{}
    }

    entries := make([]LogEntry, b.size)

    if b.size < maxBufferSize {
        // Buffer not full yet, return from start
        copy(entries, b.entries[:b.size])
    } else {
        // Buffer is full, return from oldest to newest
        copy(entries, b.entries[b.index:])
        copy(entries[maxBufferSize-b.index:], b.entries[:b.index])
    }

    return entries
}

// Clear removes all entries (thread-safe)
func (b *LogBuffer) Clear() {
    b.mu.Lock()
    defer b.mu.Unlock()

    b.entries = make([]LogEntry, maxBufferSize)
    b.index = 0
    b.size = 0
}

// Size returns current number of entries
func (b *LogBuffer) Size() int {
    b.mu.RLock()
    defer b.mu.RUnlock()
    return b.size
}
```

2. **Create `internal/config/logger/hook.go`:**

```go
package logger

import (
    "time"

    "github.com/rs/zerolog"
)

// BufferHook is a zerolog hook that captures logs to a buffer
type BufferHook struct {
    buffer *LogBuffer
}

// NewBufferHook creates a new hook that writes to the buffer
func NewBufferHook(buffer *LogBuffer) *BufferHook {
    return &BufferHook{buffer: buffer}
}

// Run implements zerolog.Hook
func (h *BufferHook) Run(e *zerolog.Event, level zerolog.Level, msg string) {
    if h.buffer == nil {
        return
    }

    entry := LogEntry{
        Level:     level.String(),
        Timestamp: time.Now(),
        Message:   msg,
        Fields:    make(map[string]interface{}),
    }

    h.buffer.Add(entry)
}
```

3. **Update `internal/config/logger/logger.go`:**

Add buffer support:

```go
// LogBuffer getter for TUI access
func (l *AppLogger) GetBuffer() *LogBuffer {
    return l.buffer
}

// Update NewLogger to support buffer
func NewLogger(cfg *Config) (Logger, *LogBuffer, error) {
    var buffer *LogBuffer

    level := parseLevel(cfg.Logging.Level)

    var output io.Writer
    switch cfg.Logging.Format {
    case "console":
        output = zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339}
    case "json":
        output = os.Stdout
    case "ui":
        // For UI mode, use buffer and discard console output
        buffer = NewLogBuffer()
        output = io.Discard
    default:
        output = os.Stdout
    }

    zlog := zerolog.New(output).
        Level(level).
        With().
        Timestamp().
        Logger()

    // Attach buffer hook if buffer exists
    if buffer != nil {
        zlog = zlog.Hook(NewBufferHook(buffer))
    }

    return &AppLogger{
        log:    zlog,
        buffer: buffer,
    }, buffer, nil
}
```

4. **Update `internal/config/logger/module.go`:**

```go
var Module = fx.Options(
    fx.Provide(func(cfg *config.Config) (Logger, *LogBuffer, error) {
        return NewLogger(cfg)
    }),
)
```

5. **Update `internal/app/cli/run.go`** to add application log viewer:

Add to `runModel`:

```go
type runModel struct {
    // ... existing fields ...
    appLogViewport  viewport.Model
    showAppLogs     bool
    appLogBuffer    *logger.LogBuffer
}
```

Update constructor:

```go
func newRunModel(
    ctx context.Context,
    cfg *config.Config,
    runner runner.Runner,
    stateMgr state.Manager,
    logMgr logs.Manager,
    procStatsProv procstats.Provider,
    log logger.Logger,
    appLogBuffer *logger.LogBuffer,  // NEW parameter
) runModel {
    // ... existing code ...
    m.appLogBuffer = appLogBuffer
    return m
}
```

Update key handlers:

```go
func (m runModel) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
    // ... existing key handlers ...

    case "tab":
        // Toggle between Services and Service Logs (was "l")
        m.ui.showLogs = !m.ui.showLogs
        return m, nil

    case "l":
        // Toggle application log overlay (new behavior)
        m.ui.showAppLogs = !m.ui.showAppLogs
        if m.ui.showAppLogs {
            m.updateAppLogViewport()
        }
        return m, nil
    }
}
```

Add log rendering:

```go
func (m *runModel) updateAppLogViewport() {
    if m.appLogBuffer == nil {
        return
    }

    entries := m.appLogBuffer.GetEntries()
    var lines []string

    for _, entry := range entries {
        // Format log entry with colors
        levelStyle := getLevelStyle(entry.Level)
        timestamp := logTimestampStyle.Render(entry.Timestamp.Format("15:04:05"))
        level := levelStyle.Render(fmt.Sprintf("[%s]", entry.Level))
        message := entry.Message

        line := fmt.Sprintf("%s %s %s", timestamp, level, message)
        lines = append(lines, line)
    }

    content := strings.Join(lines, "\n")
    m.appLogViewport.SetContent(content)
    m.appLogViewport.GotoBottom()
}

func getLevelStyle(level string) lipgloss.Style {
    switch level {
    case "debug", "trace":
        return lipgloss.NewStyle().Foreground(lipgloss.Color("#666666"))
    case "info":
        return lipgloss.NewStyle().Foreground(lipgloss.Color("#00BCD4"))
    case "warn":
        return lipgloss.NewStyle().Foreground(lipgloss.Color("#FFA726"))
    case "error", "fatal":
        return lipgloss.NewStyle().Foreground(lipgloss.Color("#FF5252"))
    default:
        return lipgloss.NewStyle()
    }
}
```

Update View() for overlay style:

```go
func (m runModel) View() string {
    if m.err != nil {
        return fmt.Sprintf("Error: %v\n\nPress q to quit.", m.err)
    }

    var b strings.Builder

    // Build main view (Services or Service Logs)
    total, running, starting, stopped, failed := m.getServiceCounts()
    phaseStr := m.getPhaseString()
    filterStr := m.getFilterString()

    var counters strings.Builder
    fmt.Fprintf(&counters, "%d/%d running", running, total)
    if starting > 0 {
        fmt.Fprintf(&counters, ", %d starting", starting)
    }
    if failed > 0 {
        fmt.Fprintf(&counters, ", %d failed", failed)
    }
    if stopped > 0 {
        fmt.Fprintf(&counters, ", %d stopped", stopped)
    }

    header := headlineLarge.Render(fmt.Sprintf("Fuku - %s | %s | %s | Filter: %s",
        m.profile, phaseStr, counters.String(), filterStr))
    b.WriteString(header)
    b.WriteString("\n\n")

    // Show either Services or Service Logs view
    if m.ui.showLogs {
        b.WriteString(m.ui.logViewport.View())
    } else {
        b.WriteString(m.ui.serviceViewport.View())
    }
    b.WriteString("\n\n")

    // Update help text
    viewName := "Services"
    helpText := "View: %s • Tab: toggle service logs • l: app logs • space: filter • a: all • s: stop/start • r: restart • ↑/↓: navigate • q: quit"
    if m.ui.showLogs {
        viewName = "Service Logs"
        helpText = "View: %s • Tab: toggle services • l: app logs • ↑/↓: navigate • q: quit"
    }

    help := labelLarge.Render(fmt.Sprintf(helpText, viewName))
    b.WriteString(help)

    mainView := b.String()

    // Overlay application logs if active
    if m.ui.showAppLogs {
        return m.renderAppLogOverlay(mainView)
    }

    return mainView
}

func (m runModel) renderAppLogOverlay(baseView string) string {
    // Render app logs as modal overlay
    var b strings.Builder

    // Show base view dimmed (optional - could be omitted for full overlay)
    // Or just show the app log viewport directly

    header := headlineLarge.Render("Application Logs")
    b.WriteString(header)
    b.WriteString("\n\n")

    b.WriteString(m.appLogViewport.View())
    b.WriteString("\n\n")

    help := labelLarge.Render("↑/↓: Scroll | l: Close | q: Quit")
    b.WriteString(help)

    return b.String()
}
```

6. **Create tests `internal/config/logger/buffer_test.go`:**

```go
package logger

import (
    "testing"
    "time"

    "github.com/stretchr/testify/assert"
)

func TestLogBuffer_Add(t *testing.T) {
    buffer := NewLogBuffer()

    entry := LogEntry{
        Level:     "info",
        Timestamp: time.Now(),
        Message:   "test message",
    }

    buffer.Add(entry)
    assert.Equal(t, 1, buffer.Size())
}

func TestLogBuffer_GetEntries(t *testing.T) {
    buffer := NewLogBuffer()

    // Add entries
    for i := 0; i < 5; i++ {
        buffer.Add(LogEntry{
            Level:     "info",
            Timestamp: time.Now(),
            Message:   "message",
        })
    }

    entries := buffer.GetEntries()
    assert.Equal(t, 5, len(entries))
}

func TestLogBuffer_RingBuffer(t *testing.T) {
    buffer := NewLogBuffer()

    // Fill beyond max size
    for i := 0; i < maxBufferSize+100; i++ {
        buffer.Add(LogEntry{
            Level:   "info",
            Message: "message",
        })
    }

    // Should only keep last maxBufferSize entries
    assert.Equal(t, maxBufferSize, buffer.Size())
}

func TestLogBuffer_Clear(t *testing.T) {
    buffer := NewLogBuffer()

    buffer.Add(LogEntry{Level: "info", Message: "test"})
    assert.Equal(t, 1, buffer.Size())

    buffer.Clear()
    assert.Equal(t, 0, buffer.Size())
}
```

7. **Update help text** (already done in View() method above):

Key changes:
- `Tab` now toggles between Services and Service Logs
- `l` now opens Application Logs overlay
- Help text dynamically updates based on current view
- More concise and intuitive navigation

8. **Test:**

```bash
cd fuku
go test ./internal/config/logger/...
go run cmd/main.go run dev
# Press Tab to switch to Service Logs
# Press Tab again to switch back to Services
# Press 'l' to open Application Logs overlay
# Press 'l' again to close overlay
```

**Benefits:**
- More intuitive navigation with Tab key
- Consistent with CMT (`l` for app logs)
- Application logs as overlay (less disruptive)
- Service logs buffer already efficient (no changes needed)
- Debug issues without restarting
- See internal application logs in real-time
- Better observability and easier troubleshooting

---

### 1.2 Enhanced Error Messages with User-Friendly Formatting

**Priority:** HIGH
**Effort:** 2 hours
**Files to create:**
- `internal/app/errors/format.go`
- `internal/app/errors/format_test.go`

**Files to modify:**
- `internal/app/errors/errors.go`
- `internal/app/cli/run.go` (error display)

**Current state:**
- Raw error messages shown to users
- No visual distinction for different error types
- Stack traces visible in production

**Target state:**
- User-friendly error messages with emojis
- Color-coded by severity
- Technical details hidden (available in logs)
- Actionable error messages

**Implementation steps:**

1. **Create `internal/app/errors/format.go`:**

```go
package errors

import (
    "fmt"

    "github.com/charmbracelet/lipgloss"
)

var (
    errorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF5252")).Bold(true)
    warningStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFA726")).Bold(true)
    infoStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#00BCD4")).Bold(true)
)

// Format returns a user-friendly error message with appropriate styling
func Format(err error) string {
    if err == nil {
        return ""
    }

    switch {
    case Is(err, ErrFailedToReadConfig):
        return warningStyle.Render("⚠️  Configuration file not found or invalid")

    case Is(err, ErrFailedToParseConfig):
        return errorStyle.Render("❌ Failed to parse configuration file")

    case Is(err, ErrInvalidRegexPattern):
        return errorStyle.Render("❌ Invalid regular expression pattern in configuration")

    case Is(err, ErrReadinessCheckFailed):
        return warningStyle.Render("⚠️  Service readiness check failed")

    default:
        return errorStyle.Render(fmt.Sprintf("❌ %s", err.Error()))
    }
}

// FormatWithHint returns formatted error with actionable hint
func FormatWithHint(err error, hint string) string {
    msg := Format(err)
    if hint != "" {
        hintStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9E9E9E")).Italic(true)
        msg += "\n" + hintStyle.Render(fmt.Sprintf("💡 Hint: %s", hint))
    }
    return msg
}

// Common error hints
const (
    HintCheckConfig     = "Check your fuku.yaml configuration file"
    HintCheckService    = "Verify the service command and working directory"
    HintCheckDeps       = "Check if service dependencies are running"
    HintCheckPort       = "Port may already be in use"
    HintCheckLogs       = "Press 'Tab' to view service logs for details"
    HintCheckAppLogs    = "Press 'l' to view application logs"
)
```

2. **Update `internal/app/errors/errors.go`:**

```go
package errors

import (
    "errors"
)

var (
    ErrFailedToReadConfig  = errors.New("failed to read config file")
    ErrFailedToParseConfig = errors.New("failed to parse config file")

    ErrInvalidRegexPattern = errors.New("invalid regex pattern")

    ErrReadinessCheckFailed = errors.New("readiness check failed")

    // Add more specific errors
    ErrServiceNotFound     = errors.New("service not found")
    ErrCircularDependency  = errors.New("circular dependency detected")
    ErrProfileNotFound     = errors.New("profile not found")
    ErrServiceStartFailed  = errors.New("service failed to start")
    ErrPortInUse           = errors.New("port already in use")
)

// Is wraps errors.Is for convenience
func Is(err, target error) bool {
    return errors.Is(err, target)
}

// As wraps errors.As for convenience
func As(err error, target interface{}) bool {
    return errors.As(err, target)
}
```

3. **Create `internal/app/errors/format_test.go`:**

```go
package errors

import (
    "errors"
    "testing"

    "github.com/stretchr/testify/assert"
)

func TestFormat(t *testing.T) {
    tests := []struct {
        name     string
        err      error
        contains string
    }{
        {
            name:     "Config read error",
            err:      ErrFailedToReadConfig,
            contains: "Configuration file",
        },
        {
            name:     "Config parse error",
            err:      ErrFailedToParseConfig,
            contains: "parse configuration",
        },
        {
            name:     "Unknown error",
            err:      errors.New("custom error"),
            contains: "custom error",
        },
        {
            name:     "Nil error",
            err:      nil,
            contains: "",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := Format(tt.err)
            if tt.contains != "" {
                assert.Contains(t, result, tt.contains)
            } else {
                assert.Empty(t, result)
            }
        })
    }
}

func TestFormatWithHint(t *testing.T) {
    err := ErrServiceNotFound
    result := FormatWithHint(err, HintCheckConfig)

    assert.Contains(t, result, "service not found")
    assert.Contains(t, result, "Check your fuku.yaml")
}
```

4. **Update `internal/app/cli/run.go`** to use formatted errors:

```go
// In View() method:
func (m runModel) View() string {
    if m.err != nil {
        return errors.Format(m.err) + "\n\nPress q to quit."
    }
    // ... rest of view
}

// In error handling:
func (m runModel) handleRunnerEvent(event runner.Event) (runModel, tea.Cmd) {
    // ... existing code ...

    case runner.EventServiceFail:
        data := event.Data.(runner.ServiceFail)
        // Log technical error
        m.log.Error().
            Err(data.Error).
            Str("service", data.Name).
            Msg("Service failed")

        // Show user-friendly error
        m.stateMgr.UpdateService(data.Name, func(svc *state.ServiceStatus) {
            svc.Status = state.StatusFailed
            svc.Error = errors.Format(data.Error)  // User-friendly format
        })

        return m, nil
    }
}
```

5. **Test:**

```bash
cd fuku
go test ./internal/app/errors/...
# Test with invalid config, missing service, etc.
```

**Benefits:**
- Better user experience
- Clear, actionable error messages
- Professional appearance
- Easier troubleshooting

---

## Phase 2: Improved Testability (Week 1-2)

### 2.1 Add Command Executor Abstraction

**Priority:** HIGH
**Effort:** 3 hours
**Files to create:**
- `internal/app/runner/executor.go`
- `internal/app/runner/executor_mock.go`
- `internal/app/runner/executor_test.go`

**Files to modify:**
- `internal/app/runner/runner.go`
- `internal/app/runner/runner_test.go`

**Current state:**
- `exec.Command()` called directly in runner
- Difficult to test process spawning
- Can't mock command execution

**Target state:**
- Executor interface abstracts command execution
- Easy to mock in tests
- Better test coverage for runner

**Implementation steps:**

1. **Create `internal/app/runner/executor.go`:**

```go
package runner

import (
    "context"
    "os/exec"
)

// Executor abstracts command execution for testability
type Executor interface {
    Run(ctx context.Context, name string, arg ...string) *exec.Cmd
}

// executor is the real implementation
type executor struct{}

// NewExecutor creates a new command executor
func NewExecutor() Executor {
    return &executor{}
}

// Run creates and returns a command
func (e *executor) Run(ctx context.Context, name string, arg ...string) *exec.Cmd {
    return exec.CommandContext(ctx, name, arg...)
}
```

2. **Update `internal/app/runner/runner.go`:**

Add executor field:

```go
type runner struct {
    cfg              *config.Config
    log              logger.Logger
    readinessFactory readiness.Factory
    callback         EventCallback
    control          chan ServiceControlRequest
    executor         Executor  // NEW field
}

func NewRunner(
    cfg *config.Config,
    readinessFactory readiness.Factory,
    log logger.Logger,
    callback EventCallback,
) Runner {
    return &runner{
        cfg:              cfg,
        log:              log,
        readinessFactory: readinessFactory,
        callback:         callback,
        control:          make(chan ServiceControlRequest, 10),
        executor:         NewExecutor(),  // Use real executor by default
    }
}

// WithExecutor sets a custom executor (for testing)
func (r *runner) WithExecutor(exec Executor) *runner {
    r.executor = exec
    return r
}
```

Update command execution:

```go
func (r *runner) startService(ctx context.Context, name string, svc *config.Service) error {
    // ... existing code ...

    // OLD: cmd := exec.CommandContext(ctx, cmdParts[0], cmdParts[1:]...)
    // NEW:
    cmd := r.executor.Run(ctx, cmdParts[0], cmdParts[1:]...)

    // ... rest of function ...
}
```

3. **Generate mock** (add to `executor.go`):

```go
//go:generate mockgen -source=executor.go -destination=executor_mock.go -package=runner
```

Run mock generation:
```bash
cd fuku
go install go.uber.org/mock/mockgen@latest
go generate ./internal/app/runner/
```

4. **Create `internal/app/runner/executor_test.go`:**

```go
package runner

import (
    "context"
    "os/exec"
    "testing"

    "github.com/stretchr/testify/assert"
)

func TestExecutor_Run(t *testing.T) {
    exec := NewExecutor()
    ctx := context.Background()

    cmd := exec.Run(ctx, "echo", "test")

    assert.NotNil(t, cmd)
    assert.Equal(t, "echo", cmd.Path)
    assert.Contains(t, cmd.Args, "test")
}
```

5. **Update `internal/app/runner/runner_test.go`** to use mock:

```go
func TestRunner_StartService_WithMock(t *testing.T) {
    ctrl := gomock.NewController(t)
    defer ctrl.Finish()

    mockExecutor := NewMockExecutor(ctrl)
    mockLogger := logger.NewMockLogger(ctrl)

    cfg := &config.Config{
        Services: map[string]*config.Service{
            "test": {
                Command: "echo hello",
                Dir:     ".",
            },
        },
    }

    runner := NewRunner(cfg, nil, mockLogger, func(Event) {}).(*runner)
    runner.executor = mockExecutor

    ctx := context.Background()

    // Setup mock expectations
    mockExecutor.EXPECT().
        Run(gomock.Any(), "echo", "hello").
        DoAndReturn(func(ctx context.Context, name string, args ...string) *exec.Cmd {
            cmd := exec.Command("echo", "mock output")
            return cmd
        })

    mockLogger.EXPECT().Info().Return(&zerolog.Event{}).AnyTimes()
    mockLogger.EXPECT().Debug().Return(&zerolog.Event{}).AnyTimes()

    err := runner.startService(ctx, "test", cfg.Services["test"])
    assert.NoError(t, err)
}
```

6. **Update `internal/app/runner/module.go`:**

```go
var Module = fx.Options(
    fx.Provide(NewExecutor),
    fx.Provide(newRunnerForFX),
)

func newRunnerForFX(
    cfg *config.Config,
    readinessFactory readiness.Factory,
    executor Executor,
    log logger.Logger,
) Runner {
    r := NewRunner(cfg, readinessFactory, log, func(Event) {})
    return r.(*runner).WithExecutor(executor).(*runner)
}
```

7. **Test:**

```bash
cd fuku
go test ./internal/app/runner/...
```

**Benefits:**
- Testable process spawning
- Can mock command execution
- Better test coverage
- Follows dependency injection pattern

---

### 2.2 Standardize Table-Driven Test Pattern

**Priority:** MEDIUM
**Effort:** 3 hours
**Files to update:**
- Various `*_test.go` files

**Implementation steps:**

1. **Document pattern in `CONTRIBUTING.md`:**

```markdown
## Test Patterns

All tests should follow the table-driven pattern with `before` functions:

```go
func Test_FunctionName(t *testing.T) {
    // Setup (once per test function)
    ctrl := gomock.NewController(t)
    defer ctrl.Finish()

    mockDep := NewMockDependency(ctrl)
    subject := NewSubject(mockDep)

    // Expected result type
    type result struct {
        output string
        err    error
    }

    // Test cases
    tests := []struct {
        name     string
        before   func()  // Mock setup
        input    string
        expected result
    }{
        {
            name: "Success case",
            before: func() {
                mockDep.EXPECT().Method(gomock.Any()).Return("output", nil)
            },
            input:    "test",
            expected: result{output: "output", err: nil},
        },
    }

    // Run tests
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            tt.before()
            output, err := subject.Function(tt.input)
            assert.Equal(t, tt.expected.output, output)
            if tt.expected.err != nil {
                assert.EqualError(t, err, tt.expected.err.Error())
            }
        })
    }
}
```
```

2. **Refactor key test files** to follow pattern:
   - `internal/app/runner/runner_test.go`
   - `internal/app/state/state_test.go`
   - `internal/app/logs/logs_test.go`

3. **Run tests:**
```bash
cd fuku
go test -v ./...
```

**Benefits:**
- Consistent test style
- Easier to add test cases
- Better readability
- Easier onboarding

---

## Phase 3: Reusable Components (Week 2)

### 3.1 Extract File Tree Renderer Component

**Priority:** MEDIUM
**Effort:** 4 hours
**Files to create:**
- `internal/app/cli/components/filetree.go`
- `internal/app/cli/components/filetree_test.go`

**Current state:**
- No file tree visualization
- Service directories shown as flat paths

**Target state:**
- Hierarchical file tree view for service directories
- Unicode box-drawing characters
- Color-coded file types
- Reusable component

**Implementation steps:**

1. **Create `internal/app/cli/components/filetree.go`:**

```go
package components

import (
    "io/fs"
    "path/filepath"
    "sort"
    "strings"

    "github.com/charmbracelet/lipgloss"
)

var (
    dirStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#00BCD4")).Bold(true)
    fileStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#E0E0E0"))
)

// FileEntry represents a file or directory
type FileEntry struct {
    Name     string
    IsDir    bool
    Children []*FileEntry
}

// FileTree represents a directory tree
type FileTree struct {
    Root *FileEntry
}

// BuildFileTree creates a file tree from a directory path
func BuildFileTree(rootPath string, maxDepth int) (*FileTree, error) {
    root := &FileEntry{
        Name:     filepath.Base(rootPath),
        IsDir:    true,
        Children: []*FileEntry{},
    }

    err := filepath.WalkDir(rootPath, func(path string, d fs.DirEntry, err error) error {
        if err != nil {
            return err
        }

        // Skip root
        if path == rootPath {
            return nil
        }

        // Calculate depth
        relPath, _ := filepath.Rel(rootPath, path)
        depth := strings.Count(relPath, string(filepath.Separator))
        if maxDepth > 0 && depth >= maxDepth {
            if d.IsDir() {
                return filepath.SkipDir
            }
            return nil
        }

        // Add to tree
        entry := &FileEntry{
            Name:  d.Name(),
            IsDir: d.IsDir(),
        }

        if d.IsDir() {
            entry.Children = []*FileEntry{}
        }

        // Add to parent (simplified - full implementation needs path tracking)
        root.Children = append(root.Children, entry)

        return nil
    })

    if err != nil {
        return nil, err
    }

    // Sort children
    sortEntries(root.Children)

    return &FileTree{Root: root}, nil
}

func sortEntries(entries []*FileEntry) {
    sort.Slice(entries, func(i, j int) bool {
        // Directories first
        if entries[i].IsDir != entries[j].IsDir {
            return entries[i].IsDir
        }
        // Then alphabetically
        return entries[i].Name < entries[j].Name
    })

    // Recursively sort children
    for _, entry := range entries {
        if entry.IsDir && len(entry.Children) > 0 {
            sortEntries(entry.Children)
        }
    }
}

// RenderTree renders the file tree with box-drawing characters
func RenderTree(tree *FileTree) string {
    if tree == nil || tree.Root == nil {
        return ""
    }

    var sb strings.Builder
    renderNode(&sb, tree.Root, "", true)
    return sb.String()
}

func renderNode(sb *strings.Builder, node *FileEntry, prefix string, isLast bool) {
    // Render current node
    if node.Name != "" {
        var connector string
        if isLast {
            connector = "└── "
        } else {
            connector = "├── "
        }

        style := fileStyle
        if node.IsDir {
            style = dirStyle
        }

        sb.WriteString(prefix)
        sb.WriteString(connector)
        sb.WriteString(style.Render(node.Name))
        sb.WriteString("\n")
    }

    // Render children
    if node.IsDir && len(node.Children) > 0 {
        childPrefix := prefix
        if node.Name != "" {
            if isLast {
                childPrefix += "    "
            } else {
                childPrefix += "│   "
            }
        }

        for i, child := range node.Children {
            isLastChild := i == len(node.Children)-1
            renderNode(sb, child, childPrefix, isLastChild)
        }
    }
}

// CountFiles counts total files in tree
func CountFiles(tree *FileTree) int {
    if tree == nil || tree.Root == nil {
        return 0
    }
    return countInNode(tree.Root)
}

func countInNode(node *FileEntry) int {
    count := 0
    if !node.IsDir {
        count = 1
    }
    for _, child := range node.Children {
        count += countInNode(child)
    }
    return count
}
```

2. **Create tests**

3. **Add to runner view** (optional - show service directory structure)

**Benefits:**
- Visualize service directory structure
- Reusable component
- Professional appearance
- Easier debugging

---

### 3.2 Add Vim-Style Modal Editing for Configuration

**Priority:** LOW
**Effort:** 6 hours
**Use case:** Edit service configuration in TUI

**Implementation steps:**

1. Create modal editing component similar to CMT's implementation
2. Add 'e' key to edit service configuration
3. Implement Normal and Insert modes
4. Save changes back to config

**Benefits:**
- Edit config without leaving TUI
- Quick service configuration changes
- Better workflow

---

## Phase 4: Documentation (Week 3)

### 4.1 Add Package Documentation

**Priority:** MEDIUM
**Effort:** 3 hours

**Implementation steps:**

1. **Add package docs to major packages:**

```go
// Package runner provides service orchestration and process management.
//
// The runner is responsible for:
//   - Resolving service dependencies
//   - Starting services in topological order
//   - Managing process lifecycles
//   - Handling readiness checks
//   - Streaming service logs
//   - Coordinating graceful shutdown
//
// Usage:
//
//   runner := NewRunner(config, readinessFactory, logger, eventCallback)
//   err := runner.Run(ctx, "dev")
package runner
```

2. **Document all exported types and functions**

3. **Generate godoc:**
```bash
cd fuku
godoc -http=:6060
```

**Benefits:**
- Better developer experience
- IDE autocomplete with docs
- Living documentation

---

### 4.2 Create Architecture Documentation

**Priority:** MEDIUM
**Effort:** 2 hours
**Files to create:**
- `docs/ARCHITECTURE.md`

**Content:**
- System overview
- Component diagram
- Data flow
- State management
- Event system
- Extension points

**Benefits:**
- Easier onboarding
- Reference for future development
- Design documentation

---

## Phase 5: Performance & Polish (Week 4)

### 5.1 Add Resource Usage Monitoring Improvements

**Priority:** LOW
**Effort:** 3 hours

**Implementation steps:**

1. Add historical resource tracking (last 60 seconds)
2. Show resource trends (CPU/memory graphs)
3. Add alerts for high resource usage
4. Optimize polling interval

**Benefits:**
- Better observability
- Identify resource-heavy services
- Performance insights

---

### 5.2 Add Service Health Dashboard

**Priority:** LOW
**Effort:** 4 hours

**Implementation steps:**

1. Create dashboard view ('d' key)
2. Show service health matrix
3. Display dependency graph
4. Show aggregate metrics

**Benefits:**
- Quick system health overview
- Better visualization
- Professional appearance

---

## Testing Checklist

After each phase, verify:

- [ ] All tests pass: `go test ./...`
- [ ] No linter errors: `make lint`
- [ ] Application runs: `go run cmd/main.go run dev`
- [ ] Help command works: `go run cmd/main.go help`
- [ ] Service orchestration works
- [ ] Readiness checks work
- [ ] Log streaming works
- [ ] Service controls work (restart, stop, start)
- [ ] Graceful shutdown works
- [ ] No visual regressions in TUI
- [ ] Resource monitoring works
- [ ] Application log viewer works ('l' key - overlay)
- [ ] Service log viewer works ('Tab' key)

---

## Success Metrics

### Code Quality
- [ ] Test coverage > 70%
- [ ] No linter warnings
- [ ] All public APIs documented
- [ ] Consistent code patterns

### Performance
- [ ] Startup time < 200ms
- [ ] Service start time < 1s per service
- [ ] TUI response time < 50ms
- [ ] Memory usage < 100MB (idle)

### User Experience
- [ ] Professional visual appearance
- [ ] Responsive to all terminal sizes
- [ ] Clear error messages
- [ ] Live application logs
- [ ] Intuitive keyboard shortcuts
- [ ] Helpful error hints

---

## Future Considerations

### Potential Shared Package

Once these improvements are complete, consider extracting reusable components to a shared package:

**Candidates for extraction:**
- Material Design 3 typography (already in `internal/app/cli/style.go`)
- ANSI-aware text wrapping (add from other project)
- Command executor abstraction (`internal/app/runner/executor.go`)
- Error formatting system (`internal/app/errors/format.go`)
- Log buffer with hook (`internal/config/logger/buffer.go`, `hook.go`)
- File tree renderer (`internal/app/cli/components/filetree.go`)

**Proposed shared package structure:**
```
github.com/yourusername/tui-commons/
├── styles/       # MD3 typography
├── text/         # ANSI-aware utilities
├── components/   # Reusable TUI components
├── executor/     # Command executor
└── logger/       # Enhanced logger
```

This would enable code sharing between multiple CLI tools while maintaining project independence.

---

## Implementation Priority

**Week 1 (High Priority):**
1. Improved key bindings + application log viewer (Tab for service logs, l for app logs)
2. Error formatting system
3. Command executor abstraction

**Week 2 (Medium Priority):**
4. Table-driven test standardization
5. File tree component
6. Documentation updates

**Week 3-4 (Low Priority):**
7. Vim-style editing
8. Resource monitoring improvements
9. Service health dashboard
10. Architecture documentation

---

## Notes

- Maintain backward compatibility
- Keep commits atomic and well-documented
- Update CHANGELOG.md for each change
- Test incrementally
- Get feedback early

---

## Questions or Issues?

If you encounter issues while implementing:

1. Check existing test patterns
2. Review similar code in the codebase
3. Consult Go best practices
4. Test each change independently
5. Verify TUI still works after each phase

Good luck! 🚀
