# CHANGELOG

## [v0.9.0]

### Features
- **feat:** Add Unix socket-based log streaming

### Documentation
- **docs:** Update documentation for log streaming architecture and components

## [v0.8.3]

### Fixes

- **fix:** Add buffer size constant and configure scanner buffer

## [v0.8.2]

### Fixes
- **fix:** Add graceful degradation for failed services during startup
- **fix:** Correct context cancellation handling

### Build
- **build:** Bump actions/checkout from 5 to 6
- **build:** Bump github.com/shirou/gopsutil/v4 from 4.25.10 to 4.25.12
- **build:** Bump codecov/codecov-action from 5.5.1 to 5.5.2

## [v0.8.1]

### Fixes
- **fix:** Adjust process command to prevent indefinite blocking
- **fix:** Replace time.Sleep with assert.Eventually

### Chore
- **chore:** Fix help key bindings info
- **chore:** Add goreleaser workflow
- **chore:** Add CHANGELOG.md

### Tests
- **test:** Add filter tests
- **test:** Add runner package tests
- **test:** Add monitor package tests

### Documentation
- **docs:** Add codecov coverage badge

## v0.8.0

### Features
- **feat:** Add process group termination and graceful shutdown
- **feat:** Show service PID

### Fixes
- **fix:** Add column headers for services
- **fix:** Rename StatusReady to StatusRunning

### Chore
- **chore:** Update format string in renderServiceRow

## v0.7.1

### Fixes
- **fix:** Resolve issue when no services are found for a profile

## v0.7.0

### Features
- **feat:** Add registry for process lifecycle management

### Fixes
- **fix:** Correct service name generation in tests

## v0.6.0

### Features
- **feat:** Enable custom tier names in configuration

### Fixes
- **fix:** Correct service row background in UI
- **fix:** Fix visual indicators for selected and checked states

### Refactor
- **refactor:** Extract getServiceIndicator method in UI

### Chore
- **chore:** Add DefaultTopology method
- **chore:** Fix unnecessary whitespace linter error
- **chore:** Updated test case for empty tier normalization
- **chore:** Added badges to documentation

### Documentation
- **docs:** Updated CLAUDE.md and README.md

## v0.5.0

### Features
- **feat:** Add log clear with ctrl+r
- **feat:** Add blink animation component
- **feat:** Add text wrapping and highlighting utilities for logs

### Fixes
- **fix:** Improve performance of log rendering

### Performance
- **perf:** Optimize log rendering

### Refactor
- **refactor:** Update dependency injection for new components
- **refactor:** Simplify view rendering in services
- **refactor:** Extract view helper functions
- **refactor:** Implement batched concurrent statistics collection
- **refactor:** Centralize UI constants in components package
- **refactor:** Add context support to process monitoring

### Chore
- **chore:** Update go.mod dependencies
- **chore:** Remove unused imports
- **chore:** Add detailed simulated log messages

### Documentation
- **docs:** Update README.md

## v0.4.0

### Features
- **feat:** Add toggle all log streams; update keybindings

### Fixes
- **fix:** Improve test case readability
- **fix:** Change UI key bindings from 'l' to 'tab'
- **fix:** Remove unused style variables

## v0.3.2

### Fixes
- **fix:** Fix header layout
- **fix:** Fix inconsistent indentation
- **fix:** Sort services alphabetically within tier

### Chore
- **chore:** Update README.md and ARCHITECTURE.md
- **chore:** Use auth service in examples

## v0.3.1

### Features
- **feat:** Add monitor package
- **feat:** Add controller to services package for lifecycle management

### Fixes
- **fix:** Resolve UI shutdown state handling
- **fix:** Add UI wire module for dependency injection
- **fix:** Implement LogFilter interface and inject into services
- **fix:** Add StopAll method to controller for stopping all services
- **fix:** Fix UI issues
- **fix:** Use model context for FSM events
- **fix:** Add fixedColumnsWidth and minServiceNameWidth constants
- **fix:** Rename maxServiceNameLength to maxLogServiceNameLength
- **fix:** Add autoscroll to logs view

### Documentation
- **docs:** Add screenshot image to README

## v0.3.0

### Features
- **feat:** Add FSM for service state management
- **feat:** Enable running services in tiers
- **feat:** Enhance CLI with scoped run commands

### Fixes
- **fix:** Correct startTier and drainPipe methods
- **fix:** Resolve G602 slice index out of range (gosec)
- **fix:** Improve CheckLog readiness
- **fix:** Update readiness check implementation
