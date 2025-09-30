# Fuku Bubble Tea TUI Implementation

## Overview
Implement a modern TUI using Bubble Tea for the Fuku CLI orchestrator. The implementation will feature an interactive terminal UI for help command, while keeping version as simple console output.

## Phase 1: Setup and Dependencies ✅

### 1.1 Add Dependencies ✅
- Added bubbletea, bubbles, lipgloss dependencies
- All dependencies installed and working

### 1.2 Create TUI Package Structure ✅
**Implementation Note:** TUI files are located in `internal/app/cli/` (not separate `internal/tui/` package):
```
internal/app/cli/
├── cli.go              # CLI interface with command routing
├── tui.go              # Root model with view routing, TUI interface
├── help.go             # Help screen model
├── run.go              # Run screen model (future)
├── style.go            # Material Design 3 typography and styling
└── *_test.go           # Comprehensive test files
```

## Phase 2: MVP - Simple Views ✅

### 2.1 Implement Root Model (tui.go) ✅
- Created root model with activeView field and viewType routing
- Implemented Init(), Update(), View() methods
- Handles Ctrl+C for graceful exit
- Delegates messages to active view
- Generated mock with `go:generate mockgen` for testing

### 2.2 Version Command ✅
**Implementation Note:** Version command uses simple console output instead of TUI:
- Prints: `0.2.0 (Fuku)` to stdout
- No TUI required (faster, simpler)
- Implemented in `cli.go` handleVersion()

### 2.3 Implement Help View (help.go) ✅
- Created helpModel with tea.Model interface
- Displays usage section, examples, version info
- Uses Material Design 3 typography from style.go
- Handles 'q', 'esc', 'ctrl+c' to quit
- Full test coverage

### 2.4 Styling System (style.go) ✅
**Implementation Note:** Centralized Material Design 3 typography:
- Complete MD3 scale: Display, Headline, Title, Body, Label variants
- Semantic mappings for consistent UI
- Helper functions: RenderTitle(), RenderHelp()
- Inline styles for proper horizontal rendering

### 2.5 Integrate TUI with CLI ✅
- Modified cli.go to use TUI interface via dependency injection
- handleHelp() launches Bubble Tea program with helpModel
- handleVersion() prints to console (no TUI)
- FX module provides both TUI and CLI instances

### 2.6 Testing ✅
- Comprehensive test coverage: 75.2% for internal/app/cli
- Test files: cli_test.go, tui_test.go, help_test.go, style_test.go
- Mock-based testing with gomock
- Table-driven tests following CLAUDE.md patterns
- All tests passing with 0 linter issues

## Phase 3: Run View - Basic Structure

### 3.1 Define Message Types (messages.go)
```go
type phaseStartMsg struct { phase string }
type phaseCompleteMsg struct { phase string }
type serviceStartedMsg struct { name string, pid int, startTime time.Time }
type serviceLogMsg struct { name string, stream string, line string }
type serviceStoppedMsg struct { name string, exitCode int }
type serviceFailedMsg struct { name string, err error }
type allServicesReadyMsg struct {}
```

### 3.2 Implement Run Model Structure (run.go)
- Create runModel struct with fields:
  - config *config.Config
  - profile string
  - phase string (current startup phase)
  - services map[string]*serviceStatus
  - logs []logEntry
  - progress bubbles/progress component
  - table bubbles/table component
  - viewport bubbles/viewport component
  - state enum (starting, running, stopped)

### 3.3 Implement Run View - Startup Phase
- Show 4-stage progress indicator:
  1. Reading configuration
  2. Service discovery
  3. Dependency resolution
  4. Launching services
- Display current phase name
- Animate progress bar as phases complete

### 3.4 Implement Service Status Table
- Create table with columns:
  - Name (service name)
  - Status (Starting → ✓ Running → ✗ Failed)
  - PID (process ID)
  - Started (time)
  - Duration (running time)
- Update table rows in real-time as services start
- Use color coding: yellow (starting), green (running), red (failed)

### 3.5 Implement Log Viewport
- Use bubbles/viewport for scrollable log display
- Format logs: `[service-name:STDOUT] log line`
- Auto-scroll to bottom (with option to pause)
- Keyboard controls:
  - Arrow up/down: scroll logs
  - PgUp/PgDn: page through logs
  - Home/End: jump to top/bottom
  - Space: pause/resume auto-scroll

## Phase 4: Runner Integration

### 4.1 Refactor Runner for Event-Driven Architecture
- Create runner.EventCallback interface or channel-based approach
- Modify runner.Run() to accept callback/channel parameter
- Send events during:
  - Configuration loading
  - Service discovery
  - Dependency resolution
  - Service startup (per service)
  - Log line output (per service)
  - Service exit/failure

### 4.2 Implement Parallel Service Startup
- Group services by dependency level (topological sort)
- Start all services at same level concurrently
- Track each service independently
- Send serviceStartedMsg for each service as it starts
- Continue streaming logs from all services

### 4.3 Connect Runner to TUI
- Create tea.Cmd functions that wrap runner operations
- Use goroutines + channels to bridge runner events to Bubble Tea messages
- Handle context cancellation for graceful shutdown
- Propagate errors from runner to TUI

### 4.4 Implement Signal Handling
- Handle SIGINT/SIGTERM in TUI
- Display shutdown message
- Stop all services gracefully
- Clean up resources
- Exit TUI cleanly

## Phase 5: Polish and Testing

### 5.1 Styling and Layout
- Apply consistent color scheme using lipgloss
- Add borders, padding, and margins
- Ensure responsive layout (handle terminal resize)
- Add status indicators (spinners, icons)
- Improve readability with proper spacing

### 5.2 Error Handling
- Display errors inline (not crash TUI)
- Show error messages in red
- Provide actionable error information
- Allow user to quit after error

### 5.3 Performance Optimization
- Limit log buffer size (e.g., last 1000 lines)
- Throttle log updates (batch messages)
- Optimize table re-rendering
- Profile memory usage with many services

### 5.4 Comprehensive Testing
- Unit tests for each model's Update logic
- Test message handling
- Test view rendering (snapshot tests)
- Integration tests with mock runner
- Test with various fuku.yaml configurations
- Test edge cases: no services, circular deps, failures

### 5.5 Update Tests
- Modify existing CLI tests to work with TUI
- Add TUI-specific tests
- Ensure test coverage remains >80%
- Update mocks if needed

## Phase 6: Documentation and Cleanup

### 6.1 Update Documentation
- Update README.md with TUI screenshots/demo
- Document keyboard shortcuts
- Add troubleshooting section
- Update CLAUDE.md with TUI architecture

### 6.2 Code Cleanup
- Remove old CLI print statements
- Clean up unused code
- Ensure consistent code style
- Run linters and formatters

### 6.3 Final Testing
- Run full test suite: `make test`
- Run linter: `make lint`
- Test with real services
- Verify all commands work

## Implementation Notes

### Key Design Decisions
- Use root model as view router (not separate routing package)
- Keep models focused (one model per view)
- Use custom messages for events (type-safe)
- Channel-based communication between runner and TUI
- Graceful degradation if terminal doesn't support TUI features

### Bubble Tea Patterns
- Model: Struct holding state
- Init(): Return initial commands (can be nil)
- Update(msg): Handle messages, return updated model + commands
- View(): Render current state as string
- Messages: Immutable data about events
- Commands: Functions that return messages (async operations)

### Dependencies
- **bubbletea**: TUI framework (Elm architecture)
- **bubbles**: Pre-built components (progress, table, viewport, spinner)
- **lipgloss**: Styling and layout library

### Testing Strategy
- Extract testable functions from models
- Mock runner events for TUI testing
- Use table-driven tests for message handling
- Test view output with known state

### Git Workflow
- Create feature branch: `feature/bubbletea-ui`
- Commit after each phase
- Run tests before each commit: `go fmt ./... && make lint && make test`
- Merge to master when complete

## Success Criteria

### MVP (Phase 2) ✅
- [x] Version command prints to console (0.2.0 (Fuku))
- [x] Help command shows styled TUI with Material Design 3 typography
- [x] Keyboard navigation works (q/esc/ctrl+c to quit)
- [x] All tests pass (75.2% coverage for cli package)
- [x] Linter passes (0 issues)
- [x] Mock generation with gomock
- [x] Centralized styling system (style.go)

### Full Implementation (Phase 4-5)
- [ ] Run command shows progress bars
- [ ] Services start in parallel
- [ ] Service table updates in real-time
- [ ] Logs stream to viewport
- [ ] Keyboard controls work (scrolling, quitting)
- [ ] Graceful shutdown on Ctrl+C
- [ ] Error handling works
- [ ] All tests pass (>80% coverage)
- [ ] Documentation updated

## Current Status
- Phase 1: ✅ Complete
- Phase 2: ✅ Complete (MVP achieved)
- Phase 3-6: Pending (run command TUI implementation)
