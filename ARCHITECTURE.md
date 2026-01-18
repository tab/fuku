# Architecture

This document describes the core architectural patterns used in fuku's service orchestration system.

## Overview

Fuku uses a layered architecture with three distinct patterns:

1. **Data/Communication Layer** - Pure data structures and pub/sub messaging
2. **Event-Driven Orchestration** - Process lifecycle management via events
3. **FSM-Based UI State** - Finite state machines for UI representation

```
┌─────────────────────────────────────────────────────────────┐
│                         CLI Layer                           │
│                    (internal/app/cli)                       │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                    Application Container                    │
│                   (internal/app - Uber FX)                  │
└─────────────────────────────────────────────────────────────┘
                              │
              ┌───────────────┼───────────────┐
              ▼               ▼               ▼
┌──────────────────┐ ┌──────────────┐ ┌──────────────────┐
│  Runner Package  │ │ Logs Package │ │   UI Package     │
│  (Event-Driven)  │ │ (Streaming)  │ │  (FSM-Based)     │
│  ├─ discovery    │ │ ├─ server    │ │  ├─ services/    │
│  ├─ readiness    │ │ ├─ hub       │ │  ├─ components/  │
│  ├─ service      │ │ ├─ client    │ │  └─ wire/        │
│  ├─ registry     │ │ └─ formatter │ └──────────────────┘
│  └─ workerpool   │ └──────────────┘          │
└──────────────────┘        │                  │
         │                  │                  │
         └──────────────────┼──────────────────┘
                            ▼
              ┌──────────────────────┐
              │   Runtime Package    │
              │ (Data/Communication) │
              │   events + commands  │
              └──────────────────────┘
                            │
                            ▼
              ┌──────────────────────┐
              │   Config Package     │
              │  (internal/config)   │
              └──────────────────────┘
```

## 1. Data/Communication Layer

**Package**: `internal/app/runtime`

The runtime package provides pure data structures and communication primitives. It has no business logic and serves as the foundation for inter-component messaging.

### Event Bus

```go
type EventBus interface {
    Subscribe(ctx context.Context) <-chan Event
    Publish(event Event)
    Close()
}
```

The EventBus implements a pub/sub pattern for broadcasting runtime events:

- **Non-blocking publish**: Events are sent to subscriber channels without blocking
- **Critical events**: Important events (phase changes) block until delivered
- **Context-aware cleanup**: Subscribers auto-unsubscribe when context cancels
- **Buffered channels**: Prevents slow subscribers from blocking publishers

### Command Bus

```go
type CommandBus interface {
    Subscribe(ctx context.Context) <-chan Command
    Publish(cmd Command)
    Close()
}
```

The CommandBus handles user-initiated control commands:

- **Stop/Restart Service**: Individual service control
- **Stop All**: Graceful shutdown trigger
- **Decoupled control**: UI publishes commands without knowing runner internals

### Event Types

```
EventProfileResolved  → Profile and tier structure resolved
EventPhaseChanged     → Application phase transition
EventTierStarting     → Tier startup begins
EventTierReady        → All services in tier are ready
EventServiceStarting  → Service process started
EventServiceReady     → Service passed readiness check
EventServiceFailed    → Service failed to start/run
EventServiceStopped   → Service process terminated
EventRetryScheduled   → Service retry scheduled
EventLogLine          → Service stdout/stderr output
EventSignalCaught     → OS signal received (SIGINT/SIGTERM)
```

### Phases

```
PhaseStartup  → Services starting in tier order
PhaseRunning  → All services ready, accepting commands
PhaseStopping → Shutdown in progress
PhaseStopped  → All services terminated
```

### Design Principles

1. **No business logic** - Only data structures and channel management
2. **Type-safe events** - Strongly typed event data via interfaces
3. **Graceful degradation** - NoOp implementations for non-UI mode
4. **Thread-safe** - All operations protected by mutexes

## 2. Event-Driven Orchestration

**Package**: `internal/app/runner`

The runner package manages actual OS processes using event-driven patterns rather than state machines.

### Why Event-Driven?

1. **External state source**: OS manages process lifecycle, runner reacts to it
2. **Async by nature**: Process I/O, signals, and readiness are inherently async
3. **Flexible retry logic**: Exponential backoff doesn't fit FSM patterns
4. **Observable**: Events provide audit trail of what happened

### Process Lifecycle

```
                    ┌─────────────┐
                    │   Resolve   │
                    │   Profile   │
                    └──────┬──────┘
                           │
                           ▼
                    ┌─────────────┐
            ┌───────│ Start Tier  │◄─────┐
            │       └──────┬──────┘      │
            │              │             │
            │              ▼             │
            │       ┌─────────────┐      │
            │       │Start Service│      │
            │       └──────┬──────┘      │
            │              │             │
            │         ┌────┴────┐        │
            │         ▼         ▼        │
            │    ┌─────────┐ ┌─────────┐ │
            │    │ Success │ │  Retry  │─┘
            │    └────┬────┘ └─────────┘
            │         │
            │         ▼
            │    ┌─────────┐
            └────│Next Tier│
                 └────┬────┘
                      │
                      ▼
                 ┌─────────┐
                 │  Wait   │◄──────┐
                 │ Signals │       │
                 └────┬────┘       │
                      │            │
                 ┌────┴────┐       │
                 ▼         ▼       │
            ┌─────────┐ ┌─────────┐│
            │ Signal  │ │ Command ││
            │Received │ │Received │┘
            └────┬────┘ └─────────┘
                 │
                 ▼
            ┌──────────┐
            │ Shutdown │
            └──────────┘
```

### Service Orchestration

```go
func (r *runner) Run(ctx context.Context, profile string) error {
    // 1. Publish startup phase
    r.event.Publish(Event{Type: EventPhaseChanged, Data: PhaseStartup})

    // 2. Resolve profile into tier structure
    tiers, _ := r.discovery.Resolve(profile)
    r.event.Publish(Event{Type: EventProfileResolved, Data: tiers})

    // 3. Start tiers sequentially
    for _, tier := range tiers {
        r.event.Publish(Event{Type: EventTierStarting})
        // Start services concurrently within tier
        r.startTier(ctx, tier)
        r.event.Publish(Event{Type: EventTierReady})
    }

    // 4. Transition to running phase
    r.event.Publish(Event{Type: EventPhaseChanged, Data: PhaseRunning})

    // 5. Wait for signals or commands
    for {
        select {
        case sig := <-sigChan:
            // Handle OS signal
        case cmd := <-commandChan:
            // Handle user command
        }
    }

    // 6. Graceful shutdown
    r.event.Publish(Event{Type: EventPhaseChanged, Data: PhaseStopping})
    r.shutdown(processes)
    r.event.Publish(Event{Type: EventPhaseChanged, Data: PhaseStopped})
}
```

### Key Patterns

1. **Registry**: Single source of truth for tracking running processes
   - Maintains process lifecycle with WaitGroup synchronization
   - Tracks insertion order for deterministic reverse-order shutdown
   - Supports process detachment for restart scenarios
   - Ensures WaitGroup only decrements when processes actually exit
2. **Worker Pool**: Limit concurrent service starts
3. **Retry with Backoff**: Automatic retry on transient failures
4. **Graceful Shutdown**: SIGTERM → wait → SIGKILL

### Registry Pattern

The Registry provides centralized process lifecycle management with proper synchronization:

```go
type Lookup struct {
    Proc     Process
    Exists   bool
    Detached bool
}

type Registry interface {
    Add(name string, proc Process, tier string)
    Get(name string) Lookup
    SnapshotReverse() []Process
    Detach(name string)
    Wait()
}
```

**Key behaviors**:

1. **Add**: Registers process, increments WaitGroup, spawns goroutine to wait for exit
2. **Get**: Returns Lookup struct containing process, existence status, and detachment status
3. **Detach**: Removes from active map and marks as detached (for restart scenarios)
4. **Wait**: Blocks until all processes have actually exited, with configurable timeout to prevent infinite hangs
5. **SnapshotReverse**: Returns ALL processes (including detached) in reverse startup order for graceful shutdown

**Critical semantics**:
- WaitGroup only decrements when process **actually exits** (Done() channel closes)
- Detach() does NOT decrement WaitGroup immediately
- SnapshotReverse() includes BOTH active and detached processes to ensure shutdown can signal all processes
- This ensures `Wait()` doesn't return while detached processes are still running
- Prevents leftover PIDs and shutdown deadlocks

**Restart flow**:
```go
// 1. Detach old process (remove from map, mark as detached)
registry.Detach(serviceName)

// 2. Stop old process
service.Stop(oldProc)

// 3. Start new process
newProc := service.Start(ctx, serviceName, config)

// 4. Add new process to registry
registry.Add(serviceName, newProc, tier)

// 5. Old process exits → Done() fires → WaitGroup decremented
// 6. New process tracked independently
```

### Context Management

The CLI creates a single cancellable context that coordinates both the runner and UI lifecycles:

```go
func (c *cli) runWithUI(ctx context.Context, profile string) (int, error) {
    ctx, cancel := context.WithCancel(ctx)
    defer cancel()

    // Runner runs in background goroutine
    go func() {
        runnerErrChan <- c.runner.Run(ctx, profile)
    }()

    // UI runs in foreground, blocks until user quits
    p, _ := c.ui(ctx, profile)
    p.Run()

    // When UI exits, cancel context to stop runner
    cancel()

    // Wait for runner to finish
    err := <-runnerErrChan
}
```

**Key benefits**:
1. **Single source of truth**: One context controls entire application lifecycle
2. **Clean shutdown**: UI exit triggers context cancellation, runner responds gracefully
3. **Testability**: Tests can pass their own contexts for lifecycle control

**Critical events**: Lifecycle state changes are marked as `Critical: true` to guarantee delivery even when event buffers are full, preventing UI state desynchronization.

### Command Handling

Commands are processed in both startup and running phases:

```go
switch cmd.Type {
case CommandStopService:
    r.stopService(data.Service)
    // Publishes: EventServiceStopped (or EventServiceFailed if not found)

case CommandRestartService:
    r.restartService(ctx, data.Service)
    // Publishes: EventServiceStopped → EventServiceStarting → EventServiceReady
    // Or EventServiceFailed if service config not found

case CommandStopAll:
    return true  // Exit run loop, trigger shutdown
}
```

**Startup phase handling**: Commands (restart/stop individual services) are processed during startup to handle user interactions before all services are ready. StopAll command aborts the entire startup sequence.

## 3. FSM-Based UI State

**Package**: `internal/app/ui/services`

The UI uses finite state machines to manage visual representation of service states.

### Why FSM for UI?

1. **Predictable transitions**: Users expect consistent state progression
2. **Valid states only**: Prevent displaying invalid combinations
3. **Operation tracking**: Know what operations are in progress
4. **Loader management**: Associate spinners with specific transitions

### Service State Machine

```
                    ┌─────────────┐
                    │   Stopped   │
                    └──────┬──────┘
                           │ Start
                           ▼
                    ┌─────────────┐
         ┌──────────│  Starting   │──────────┐
         │ Failed   └──────┬──────┘          │ Stopped
         ▼                 │ Started         ▼
    ┌─────────┐            ▼            ┌─────────┐
    │ Failed  │      ┌─────────────┐    │ Stopped │
    └────┬────┘      │   Running   │    └─────────┘
         │           └──────┬──────┘
         │                  │
         └──────────┬───────┘
                    │ Restart
                    ▼
              ┌─────────────┐
              │ Restarting  │
              └──────┬──────┘
                     │ Stopped
                     ▼
               ┌─────────┐
               │ Stopped │──── Start ───► Starting
               └─────────┘
```

### State Definitions

```go
const (
    Stopped    = "stopped"    // Service not running
    Starting   = "starting"   // Process started, waiting for readiness
    Running    = "running"    // Service ready and operational
    Stopping   = "stopping"   // Shutdown in progress
    Restarting = "restarting" // Restart initiated
    Failed     = "failed"     // Service failed
)
```

### FSM Transitions

```go
fsm.Events{
    {Name: Start,   Src: []string{Stopped, Restarting}, Dst: Starting},
    {Name: Stop,    Src: []string{Running},             Dst: Stopping},
    {Name: Restart, Src: []string{Running, Failed, Stopped}, Dst: Restarting},
    {Name: Started, Src: []string{Starting},            Dst: Running},
    {Name: Stopped, Src: []string{Stopping, Restarting}, Dst: Stopped},
    {Name: Failed,  Src: []string{Starting, Running, Restarting}, Dst: Failed},
}
```

### FSM Callbacks

Callbacks execute side effects on state transitions:

```go
fsm.Callbacks{
    OnStarting: func(ctx, e) {
        service.MarkStarting()
        if !loader.Has(service) {           // Don't overwrite "Restarting..."
            loader.Start("Starting...")
        }
    },
    OnStopping: func(ctx, e) {
        service.MarkStopping()
        loader.Start("Stopping...")
        commandBus.Publish(CommandStopService)
    },
    OnRestarting: func(ctx, e) {
        loader.Start("Restarting...")
        commandBus.Publish(CommandRestartService)
    },
    OnRunning: func(ctx, e) {
        service.MarkRunning()
    },
    OnStopped: func(ctx, e) {
        service.MarkStopped()  // Clears PID
    },
    OnFailed: func(ctx, e) {
        service.MarkFailed()
    },
}
```

### Event → FSM Synchronization

The UI subscribes to EventBus and updates FSM accordingly:

```go
func (m Model) handleServiceReady(event Event) Model {
    data := event.Data.(ServiceReadyData)
    if service := m.services[data.Service]; service != nil {
        m.loader.Stop(data.Service)           // Remove spinner
        service.FSM.Event(ctx, Started)       // Transition FSM
        // FSM callback updates service.Status
    }
    return m
}
```

### Loader Queue (FIFO)

Operations are tracked in a first-in-first-out queue:

```go
type Loader struct {
    Model  spinner.Model
    Active bool
    queue  []LoaderItem  // FIFO queue
}

func (l *Loader) Start(service, msg string)  // Add/update operation
func (l *Loader) Stop(service string)        // Remove operation
func (l *Loader) Message() string            // Get front of queue
```

This provides:
- Predictable message ordering
- Multiple concurrent operations
- Visual feedback for each operation

## 4. Log Streaming

**Package**: `internal/app/logs`

The logs package provides real-time log streaming from running fuku instances via Unix sockets.

### Architecture

```
┌─────────────────────────┐
│   fuku --logs api db    │ ◄─── Separate terminal
└───────────┬─────────────┘
            │ Unix Socket
            │ /tmp/fuku-<profile>.sock
            ▼
┌──────────────────────────────────────────────┐
│        fuku --run=profile (main process)      │
│  Service → teeStream → Hub → SocketServer    │
└──────────────────────────────────────────────┘
```

### Components

1. **Server** - Unix socket server that accepts client connections
2. **Hub** - Connection hub for broadcasting log messages to subscribers
3. **Client** - Connects to running instance and streams logs
4. **Formatter** - Formats log output with colors (console) or JSON

### Protocol

JSON lines over Unix socket:

```json
// Client → Server (subscribe)
{"type":"subscribe","services":["api","db"]}

// Server → Client (log message)
{"type":"log","service":"api","message":"Server started on :8080"}
```

### FX Dependency Injection

All components use proper FX wiring:

```go
// logs/module.go
var Module = fx.Options(
    fx.Provide(NewClient),
    fx.Provide(NewRunner),
)

// NewLogFormatter takes *config.Config and configures styles internally
func NewLogFormatter(cfg *config.Config) *LogFormatter
```

## Separation of Concerns

The key insight is **source of truth separation**:

| Aspect | Package | Pattern | Why |
|--------|---------|---------|-----|
| Process lifecycle | Runner | Event-driven | OS manages reality |
| User actions | Runtime | Command bus | Decoupled control |
| System events | Runtime | Event bus | Observable history |
| Visual state | UI | FSM | Consistent UX |

### Data Flow Example: Restart Service

1. **User presses 'r'**
   ```
   UI: handleKeyPress → FSM.Event(Restart)
   ```

2. **FSM callback publishes command**
   ```
   UI: OnRestarting → CommandBus.Publish(RestartService)
   ```

3. **Runner receives command**
   ```
   Runner: handleCommand → stopService + startServiceWithRetry
   ```

4. **Runner publishes events**
   ```
   Runner: EventServiceStopped → EventServiceStarting → EventServiceReady
   ```

5. **UI handles events**
   ```
   UI: handleServiceStopped → loader.Stop()
   UI: handleServiceStarting → loader.Start()
   UI: handleServiceReady → FSM.Event(Started) → loader.Stop()
   ```
