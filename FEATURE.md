# Feature: Simple Log File

## Problem

Current TUI-based logs view has persistent issues with text wrapping and syntax highlighting. The complexity of maintaining a proper terminal UI for log viewing outweighs its benefits.

## Solution

Replace the TUI logs view with a simple log file approach. Users view logs using standard Unix tools (`tail -f`, `grep`, `less`, etc.).

## Design

### Log File

- **Location**: `/tmp/fuku.log`
- **Format**: Raw output from services, preserving ANSI escape codes
- **Lifecycle**:
  - Delete existing file on startup
  - Create new file
  - Delete on clean shutdown
  - Stale file after crash is acceptable

### Architecture

Single writer goroutine ensures correct log ordering:

```
Service A stdout ─┐
Service A stderr ─┼─► EventBus ─► LogWriter ─► /tmp/fuku.log
Service B stdout ─┤
Service B stderr ─┘
```

All service output flows through EventBus to LogWriter. Single goroutine writes sequentially, guaranteeing correct ordering without file locking or interleaved writes.

### Filtering

- Runtime filtering: user can change which services' logs appear while running
- Only logs from selected services are written to the file
- Filter changes take effect immediately (new logs only, no retroactive filtering)
- Reuse existing `ui.LogFilter` pattern for thread-safe filter state

### User Experience

1. Start fuku with a profile
2. Open separate terminal: `tail -f /tmp/fuku.log`
3. Use grep for filtering: `tail -f /tmp/fuku.log | grep "error"`
4. Colors work automatically (ANSI codes preserved)
5. Toggle service log inclusion from TUI at runtime

## Out of Scope

- Log rotation
- Persistent logs across sessions
- Service name prefixes (may add later)
- Multiple log files

---

## Implementation Plan

### Phase 1: LogWriter Core

**Location**: `internal/app/runtime/logwriter.go`

**Interface**:
```go
type LogWriter interface {
    Start(ctx context.Context)
    Close() error
    SetFilter(service string, enabled bool)
    IsEnabled(service string) bool
}
```

**Implementation details**:
- Subscribe to EventBus on Start()
- Filter for `EventLogLine` events only
- Single goroutine reads from subscription channel and writes to file
- Thread-safe filter map (same pattern as `ui.LogFilter`)
- Write raw `LogLineData.Message` with newline, no formatting

**Files to create**:
- `internal/app/runtime/logwriter.go` - interface and implementation
- `internal/app/runtime/logwriter_test.go` - unit tests
- `internal/app/runtime/logwriter_mock.go` - generated mock

### Phase 2: File Lifecycle

**In LogWriter**:
- `Start()`: Delete existing `/tmp/fuku.log`, create new file
- `Close()`: Close file handle, delete file
- Handle write errors gracefully (log warning, don't crash)

**Constants**:
```go
const LogFilePath = "/tmp/fuku.log"
```

### Phase 3: FX Wiring

**Update `internal/app/runtime/module.go`**:
```go
var Module = fx.Module("runtime",
    fx.Provide(
        func() EventBus { return NewEventBus(100) },
        func() CommandBus { return NewCommandBus(10) },
        NewLogWriter,  // Add LogWriter provider
    ),
)
```

**LogWriter constructor**:
```go
func NewLogWriter(eventBus EventBus) LogWriter {
    return &logWriter{
        eventBus: eventBus,
        filter:   make(map[string]bool),
    }
}
```

**Lifecycle hook** (in `internal/app/app.go` or dedicated place):
- OnStart: call `logWriter.Start(ctx)`
- OnStop: call `logWriter.Close()`

### Phase 4: TUI Integration

**Filter control** (reuse existing pattern from services view):
- Add key binding to toggle log file inclusion per service
- Can reuse existing `ui.LogFilter` or sync state with LogWriter

**Display log file path**:
- Show `/tmp/fuku.log` in status bar or help
- User knows where to `tail -f`

**Commands** (optional, if needed):
- Add `CommandToggleLogFilter` to `runtime/commands.go`
- TUI publishes command, LogWriter subscribes and updates filter

### Phase 5: Cleanup (After Verification)

- Remove `internal/app/ui/logs/` package
- Remove logs view from TUI navigation
- Update tests

---

## File Changes Summary

### New Files
| File | Purpose |
|------|---------|
| `internal/app/runtime/logwriter.go` | LogWriter interface and implementation |
| `internal/app/runtime/logwriter_test.go` | Unit tests |
| `internal/app/runtime/logwriter_mock.go` | Generated mock |

### Modified Files
| File | Changes |
|------|---------|
| `internal/app/runtime/module.go` | Add LogWriter to providers |
| `internal/app/app.go` | Add lifecycle hooks for LogWriter |
| `internal/app/ui/services/` | Add key binding for log filter toggle |

### Removed Files (Phase 5)
| File | Reason |
|------|--------|
| `internal/app/ui/logs/*.go` | Replaced by log file |

---

## Testing Strategy

### Unit Tests
- LogWriter filters correctly by service
- LogWriter writes to file in order
- LogWriter handles filter changes at runtime
- File lifecycle (create on start, delete on close)

### Integration Tests
- Multiple services write logs concurrently, verify ordering
- Filter toggle affects subsequent writes only

### Manual Testing
1. Start fuku with multiple services
2. Run `tail -f /tmp/fuku.log` in separate terminal
3. Verify logs appear in correct order
4. Toggle service filter, verify effect
5. Stop fuku, verify file is deleted

---

## Decisions

1. **Filter default**: All services enabled by default (same as current behavior)
2. **Service prefix**: Raw output only, no service name prefix (may add later)
