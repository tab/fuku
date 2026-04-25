# CHANGELOG

## v0.19.0

### Features
- **feat:** Add REST API server with auth middleware
- **feat:** Add service filter with vim-style search in TUI
- **feat:** Add per-service readiness timeline in TUI
- **feat:** Restart all failed services in TUI

### Fixes
- **fix:** Fix Backfill to use ring-buffer in TUI
- **fix:** Scope auth middleware to API prefix

### Refactor
- **refactor:** Add start functions for lifecycle management
- **refactor:** Use UUID service identity in bus

### Build
- **build:** Bump charm.land/bubbles/v2 from 2.0.0 to 2.1.0
- **build:** Bump github.com/shirou/gopsutil/v4 from 4.26.2 to 4.26.3
- **build:** Bump github.com/getsentry/sentry-go from 0.43.0 to 0.44.1

### Chore
- **chore:** Add API spec and bruno-collection
- **chore:** Update documentation and README.md

### Tests
- **test:** Add REST API e2e tests

## v0.18.0

### Features
- **feat:** Rewrite log streaming with history replay support
- **feat:** Show profile title in UI

### Fixes
- **fix:** Validate services before starting `run` and `stop` commands

### Refactor
- **refactor:** Split logs into relay, render, and logs packages

### Build
- **build:** Update bubbletea dependencies

### Chore
- **chore:** Update golangci-lint version to v2.11.3

### Tests
- **test:** Add e2e tests for missing and empty config scenarios

## v0.17.0

### Features
- **feat:** Add configuration override files (`fuku.override.yaml`) and support for both `.yaml`/`.yml` formats

### Fixes
- **fix:** Clean up stale Unix sockets in log streaming

### Refactor
- **refactor:** Split CLI and TUI into standalone and UI command runners

### Chore
- **chore:** Update Sentry release naming and CI workflows

### Tests
- **test:** Add e2e tests for config overrides, socket cleanup, and yml config

### Documentation
- **docs:** Update project documentation and add new pages

## v0.16.0

### Features
- **feat:** Add custom command option for services (`command` field)
- **feat:** Add resource sampling for CPU/memory monitoring and telemetry

### Chore
- **chore:** Add linters to golangci-lint configuration
- **chore:** Update CI workflows

### Tests
- **test:** Add custom command e2e tests

### Documentation
- **docs:** Add new feature and about pages
- **docs:** Update documentation

## v0.15.4

### Build
- **build:** Add Homebrew tap support via goreleaser

## v0.15.3

### Fixes
- **fix:** Replace concurrent stats collection with serial collector in monitor

### Build
- **build:** Bump github.com/shirou/gopsutil/v4 from 4.26.1 to 4.26.2

### Chore
- **chore:** Update Go version to 1.25

## v0.15.2

### Fixes
- **fix:** Fix UI initialization sequence

### Refactor
- **refactor:** Update to bubbletea v2.0.0

### Tests
- **test:** Add lifecycle management e2e tests

## v0.15.1

### Fixes
- **fix:** Update default environment to production

## v0.15.0

### Features
- **feat:** Add Sentry integration with configuration, scope tagging, and metrics
- **feat:** Add environment variable loading with priority files (`.env.local`, `.env.<GO_ENV>.local`, `.env`)

### Chore
- **chore:** Add Sentry release integration to CI
- **chore:** Update goreleaser configuration

### Documentation
- **docs:** Add metrics and Sentry integration details to documentation

## v0.14.0

### Features
- **feat:** Add preflight cleanup to detect and terminate orphaned processes before starting services

### Refactor
- **refactor:** Extract shared worker pool package from runner

### Chore
- **chore:** Add `nestif` linter to enforce flat control flow
- **chore:** Update golangci-lint action to v9.2.0 and linter to v2.8.0

### Documentation
- **docs:** Update architecture diagrams and documentation

## v0.13.0

### Features
- **feat:** Add `init` command to generate config

### Fixes
- **fix:** Enforce no arguments for `version` command

### Build
- **build:** Bump github.com/charmbracelet/bubbles from 0.21.0 to 1.0.0
- **build:** Bump github.com/spf13/cobra from 1.8.0 to 1.10.2

### Chore
- **chore:** Add `ConfigFile` constant

## v0.12.0

### Features
- **feat:** Add service logs output with banner display via `fuku logs`
- **feat:** Add per-service log output configuration (`logs.output`)

### Fixes
- **fix:** Fix e2e tests

### Refactor
- **refactor:** Replace scanner-based stream reading with buffered reader in teeStream

### Chore
- **chore:** Update UI text to lowercase and remove bold from service status styles
- **chore:** Update CI and codecov badge links

### Documentation
- **docs:** Update readiness checks and add pre-flight checks to README

## v0.11.0

### Features
- **feat:** Add file watcher with hot-reload support for automatic service restart on file changes
- **feat:** Add TCP readiness check for service health monitoring
- **feat:** Add port pre-flight check to detect port conflicts before starting services

### Fixes
- **fix:** Make restartService call asynchronous to prevent blocking
- **fix:** Fix error messages display in TUI

### Refactor
- **refactor:** Refactor app packages organization

### Chore
- **chore:** Add test:race target for race detection
- **chore:** Update examples with simple Go services

### Documentation
- **docs:** Update architecture diagrams and documentation

## v0.10.0

### Features
- **feat:** Add concurrency workers configuration (`concurrency.workers`)
- **feat:** Add retry configuration (`retry.attempts`, `retry.backoff`)
- **feat:** Add logs buffer size configuration (`logs.buffer`)
- **feat:** Add command parsing with Cobra framework

### Fixes
- **fix:** Handle force quit key press in TUI
- **fix:** Update config constants

### Refactor
- **refactor:** Refactor app layout in TUI
- **refactor:** Add `WithComponent` method to logger
- **refactor:** Add logger into logs runner

### Chore
- **chore:** Add godoc comments across packages
- **chore:** Add `IsNil` method for ServiceState
- **chore:** Update demo assets
- **chore:** Update examples

### Tests
- **test:** Add tests for missing makefile and directory
- **test:** Add formatter tests
- **test:** Use expected hash values in formatter_test
- **test:** Add layout component tests

## v0.9.1

### Fixes
- **fix:** Refine getServiceIndicator UI component

### Chore
- **chore:** Update log command examples in CLI documentation

## v0.9.0

### Features
- **feat:** Add Unix socket-based log streaming

### Documentation
- **docs:** Update documentation for log streaming architecture and components

## v0.8.3

### Fixes
- **fix:** Add buffer size constant and configure scanner buffer

## v0.8.2

### Fixes
- **fix:** Add graceful degradation for failed services during startup
- **fix:** Correct context cancellation handling

### Build
- **build:** Bump actions/checkout from 5 to 6
- **build:** Bump github.com/shirou/gopsutil/v4 from 4.25.10 to 4.25.12
- **build:** Bump codecov/codecov-action from 5.5.1 to 5.5.2

## v0.8.1

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
