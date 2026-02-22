# Fuku Development Guide

## Project Overview

**Fuku** is a lightweight CLI orchestrator for running and managing multiple local services in development environments. It's designed for speed, simplicity, and readability. Key features include:

- Service orchestration with tier-based startup ordering
- Concurrent service execution with proper startup ordering
- Process lifecycle management with signal handling
- Simple YAML configuration format
- Structured logging with zerolog
- Dependency injection with Uber FX
- Clean architecture with interfaces and mocks

## Architecture Overview

### Core Components

1. **Entry Point** (`cmd/`)
   - `main.go` - Application bootstrap with FX dependency injection and configuration loading

2. **Core Packages** (`internal/`)
   - **app/** - Main application container and lifecycle management
   - **app/bus/** - Unified pub/sub messaging for events and commands
   - **app/cli/** - Command-line interface parsing and command handling
   - **app/discovery/** - Profile resolution to tiers and services
   - **app/lifecycle/** - Process termination with SIGTERM/SIGKILL handling
   - **app/logs/** - Log streaming via Unix sockets (server, hub, client, formatter)
   - **app/process/** - Process interface and handle implementation
   - **app/readiness/** - HTTP, TCP, and log-based health checks
   - **app/registry/** - Running process tracking with detach support
   - **app/runner/** - Service orchestration and startup coordination
   - **app/ui/services/** - Interactive TUI with Bubble Tea framework
   - **app/watcher/** - File change detection with debouncing for hot-reload
   - **config/** - Configuration loading, parsing, and data structures
   - **errors/** - Application-specific error definitions

### Key Interfaces and Abstractions

1. **runner.Runner** - Core abstraction for service orchestration:
   ```go
   type Runner interface {
       Run(ctx context.Context, profile string) error
   }
   ```

2. **cli.CLI** - Interface for command-line operations:
   ```go
   type CLI interface {
       Execute() (exitCode int, err error)
   }
   ```

3. **logger.Logger** - Structured logging interface using zerolog

4. **bus.Bus** - Unified pub/sub messaging for events and commands:
   ```go
   type Bus interface {
       Subscribe(ctx context.Context) <-chan Message
       Publish(msg Message)
       Close()
   }
   ```

5. **logs.Runner** - Log streaming mode runner:
   ```go
   type Runner interface {
       Run(profile string, services []string) int
   }
   ```

6. **logs.Client** - Unix socket client for log streaming:
   ```go
   type Client interface {
       Connect(socketPath string) error
       Subscribe(services []string) error
       Stream(ctx context.Context, output io.Writer) error
       Close() error
   }
   ```

### Execution Flow

1. **CLI Entry Point** (`cmd/main.go`)
   - Parses command-line arguments using cobra via `cli.Parse()`
   - Loads configuration from `fuku.yaml` using Viper
   - Initializes FX container with all dependencies
   - Starts application lifecycle

2. **Application Container** (`internal/app/app.go`)
   - Manages application lifecycle with FX hooks
   - Calls `cli.Execute()` to run the parsed command
   - Handles graceful shutdown

3. **Command Processing** (`internal/app/cli/`)
   - `commands.go` - Cobra-based argument parsing with `Parse()` function
   - `cli.go` - Command execution via `Execute()` method
   - Supports flags in any position (e.g., `--no-ui run core` or `run core --no-ui`)
   - Commands: `run`, `init`, `logs`, `version`, `help` with short aliases

4. **Service Orchestration** (`internal/app/runner/runner.go`)
   - Orders services by tier for startup sequencing
   - Manages process lifecycle with signal handling (SIGINT, SIGTERM)
   - Streams service logs with prefixed output format
   - Stops services in reverse order on shutdown

5. **Interactive TUI** (`internal/app/ui/services/`)
   - Bubble Tea framework for terminal UI
   - FSM-based service state management (stopped, starting, running, stopping, restarting, failed)
   - Real-time CPU and memory monitoring via gopsutil
   - Event-driven updates via EventBus subscription
   - Command publishing for service control
   - Log viewing with service filtering
   - FIFO loader queue for operation tracking

### Configuration Capabilities

1. **Service Definition**
   - Directory-based service configuration
   - Tier-based startup ordering
   - Automatic environment file detection (`.env.development`)
   - Makefile-based service execution (`make run`)

2. **Profile Management**
   - Logical grouping of services for batch execution
   - Profile values can be `"*"` (all services) or a list of service names
   - Default profile support for common configurations

3. **Logging Configuration**
   - Console and JSON format support
   - Configurable log levels (debug, info, warn, error)
   - Service-specific log streaming with prefixes

4. **Concurrency Configuration**
   - Worker pool size (`concurrency.workers`, default: 5)
   - Controls max concurrent service starts

5. **Retry Configuration**
   - Max retry attempts (`retry.attempts`, default: 3)
   - Initial backoff duration (`retry.backoff`, default: 500ms)

6. **Log Streaming Configuration**
   - Buffer size (`logs.buffer`, default: 100)
   - Controls socket log streaming buffer

7. **Watch Configuration (Hot-Reload)**
   - Per-service file watching with glob patterns (`watch.include`)
   - Ignore patterns for files to exclude (`watch.ignore`)
   - Shared paths for cross-service dependencies (`watch.shared`)
   - Debounce duration to prevent restart storms (`watch.debounce`)

8. **Per-Service Log Output**
   - Configurable output streams per service (`logs.output`)
   - Valid values: `stdout`, `stderr` (default: both streams)

### Testing Patterns

1. **Mock Generation**
   - Uses `go.uber.org/mock` for interface mocking
   - Generated mocks with mockgen command using full paths:
     ```bash
     mockgen -source=internal/app/runtime/commands.go -destination=internal/app/runtime/commands_mock.go -package=runtime
     ```
   - Separate mock files for each interface
   - Do NOT add `//go:generate` directives to source files

2. **Test Structure**
   - Table-driven tests with subtests using testify
   - Comprehensive error case coverage
   - Output capturing for CLI command testing
   - Mock expectation setup and verification
   - Entry point testing with extracted testable functions
   - Integration test skipping for complex application lifecycle scenarios

3. **Table Tests with Mocks Pattern**
   - Mocks are created once at the test function level
   - Each test case has a `before func()` that sets up mock expectations
   - Test data and mock expectations are co-located in the same test case
   - `tt.before()` is called just before executing the test logic
   - Example structure:
     ```go
     func Test_Example(t *testing.T) {
         ctrl := gomock.NewController(t)
         defer ctrl.Finish()

         mockDep := NewMockDependency(ctrl)
         subject := &Implementation{dep: mockDep}

         tests := []struct {
             name   string
             before func()
             input  string
             expect bool
         }{
             {
                 name: "success case",
                 input: "test-input",
                 before: func() {
                     mockDep.EXPECT().Method("test-input").Return(nil)
                 },
                 expect: true,
             },
         }

         for _, tt := range tests {
             t.Run(tt.name, func(t *testing.T) {
                 tt.before()
                 result := subject.TestMethod(tt.input)
                 assert.Equal(t, tt.expect, result)
             })
         }
     }
     ```

### Current Test Files
- `cmd/main_test.go` - Tests for entry point functions and FX application creation
- `internal/app/app_test.go` - Application container and lifecycle testing
- `internal/app/bus/bus_test.go` - Bus pub/sub messaging testing
- `internal/app/cli/cli_test.go` - CLI command execution testing
- `internal/app/cli/commands_test.go` - Cobra command parsing tests
- `internal/app/discovery/discovery_test.go` - Profile resolution testing
- `internal/app/lifecycle/lifecycle_test.go` - Process termination testing
- `internal/app/logs/broadcast_test.go` - Log broadcast message testing
- `internal/app/logs/client_test.go` - Unix socket client testing
- `internal/app/logs/formatter_test.go` - Log formatter testing
- `internal/app/logs/hub_test.go` - Log hub connection testing
- `internal/app/logs/runner_test.go` - Log streaming runner testing
- `internal/app/logs/server_test.go` - Log server testing
- `internal/app/monitor/monitor_test.go` - Process monitoring testing
- `internal/app/process/process_test.go` - Process handle testing
- `internal/app/readiness/readiness_test.go` - Readiness check testing
- `internal/app/registry/registry_test.go` - Process registry testing
- `internal/app/runner/guard_test.go` - Restart guard testing
- `internal/app/runner/runner_test.go` - Service orchestration and tier ordering
- `internal/app/runner/service_test.go` - Service start/stop/restart testing
- `internal/app/runner/worker_test.go` - Worker pool testing
- `internal/app/ui/components/blink_test.go` - Blink animation testing
- `internal/app/ui/components/layout_test.go` - Layout component testing
- `internal/app/ui/services/controller_test.go` - Service controller testing
- `internal/app/ui/services/helpers_test.go` - Service helper functions testing
- `internal/app/ui/services/keys_test.go` - Services view key bindings
- `internal/app/ui/services/loader_test.go` - Loader queue operations
- `internal/app/ui/services/model_test.go` - Service state methods and helpers
- `internal/app/ui/services/monitor_test.go` - CPU/memory formatting functions
- `internal/app/ui/services/state_test.go` - FSM state transitions and callbacks
- `internal/app/ui/services/update_test.go` - Event handlers
- `internal/app/ui/services/view_test.go` - View rendering functions
- `internal/app/ui/wire/module_test.go` - UI wire module testing
- `internal/app/watcher/debouncer_test.go` - Debouncer testing
- `internal/app/watcher/matcher_test.go` - File matcher testing
- `internal/app/watcher/watcher_test.go` - File watcher testing
- `internal/config/config_test.go` - Configuration loading and parsing
- `internal/config/logger/logger_test.go` - Logger implementation testing
- `internal/app/errors/` - Error definitions (no test file - contains only constants)
- `e2e/` - End-to-end tests (default tier, tier ordering, watch/hot-reload, logs command)

## Primary Guidelines

- provide brutally honest and realistic assessments of requests, feasibility, and potential issues. no sugar-coating. no vague possibilities where concrete answers are needed.
- always operate under the assumption that the user might be incorrect, misunderstanding concepts, or providing incomplete/flawed information. critically evaluate statements and ask clarifying questions when needed.
- don't be flattering or overly positive. be honest and direct.
- we work as equal partners and treat each other with respect as two senior developers with equal expertise and experience.
- prefer simple and focused solutions that are easy to understand, maintain and test.
- use table-driven tests ONLY when testing multiple scenarios with different inputs/outputs; for single test cases, use plain test functions instead of table tests with one entry
- table tests are appropriate when you have 2+ test cases with meaningful variations in input/output/behavior
- don't overthink solutions - implement the simplest thing that works, then iterate if needed

## Architecture Guidelines

### Dependency Injection with FX
- **always use Uber FX for dependency injection** - this is non-negotiable
- all components must be wired through FX modules (fx.Provide, fx.Invoke)
- never instantiate dependencies manually in application code; let FX handle the wiring
- use FX lifecycle hooks (fx.OnStart, fx.OnStop) for component initialization and cleanup

### Interfaces and Mocks
- **always define interfaces for dependencies** - this is required for FX injection and testability
- interfaces should be defined on the consumer side (idiomatic Go)
- **always generate mocks for interfaces** using `go.uber.org/mock`:
  ```bash
  mockgen -source=internal/path/to/file.go -destination=internal/path/to/file_mock.go -package=packagename
  ```
- every interface should have a corresponding mock file for testing
- mocks are stored alongside source files (e.g., `foo.go` → `foo_mock.go`)

### Keep It Simple
- **do not create abstractions unless they are needed** - YAGNI (You Aren't Gonna Need It)
- **never use the Factory pattern** - we always have exactly one implementation per interface, so factories add unnecessary indirection
- one interface = one implementation = one mock (for testing)
- if you're tempted to add a factory, abstract base class, or generalization - stop and ask if it's actually needed right now
- prefer concrete, straightforward code over clever abstractions
- don't build for hypothetical future requirements; solve the current problem

## Build, Lint and Test Commands

```bash
# Build binary
go build -o cmd/fuku ./cmd

# Run all tests (always run from the top level)
make test

# Lint code (always run from the top level)
make lint

# Coverage report (always run from the top level)
make coverage

# Format code
go fmt ./...

# Run completion sequence (formatting, linting and testing)
go fmt ./... && make lint && make vet && make test
```

**IMPORTANT:** NEVER commit without running tests, formatter and linters for the entire codebase!

## Important Workflow Notes

- always run tests, linter BEFORE committing anything
- run formatting, code generation, linting and testing on completion
- never commit without running completion sequence
- run tests and linter after making significant changes to verify functionality
- IMPORTANT: never put into commit message any mention of Claude or Claude Code
- IMPORTANT: never include "Test plan" sections in PR descriptions
- do not add comments that describe changes, progress, or historical modifications
- comments should only describe the current state and purpose of the code, not its history or evolution
- generate mocks using mockgen with full paths, never modify generated files manually
- mocks are generated with `go.uber.org/mock` and stored alongside source files
- do NOT add `//go:generate` directives to source files; run mockgen command directly
- after important functionality added, update README.md accordingly
- when merging master changes to an active branch, make sure both branches are pulled and up to date first
- don't leave commented out code in place
- if working with github repos use `gh`
- never nest if blocks — `if { if { } }` is forbidden; use guard clauses (early return/continue) to flatten
- never use goto
- prefer early returns to reduce nesting; else/else if are acceptable when they improve readability
- never inline table test cases; always use multi-line format with each field on its own line
- before any significant refactoring, ensure all tests pass and consider creating a new branch
- when refactoring, editing, or fixing failed tests:
  - do not redesign fundamental parts of the code architecture
  - if unable to fix an issue with the current approach, report the problem and ask for guidance
  - focus on minimal changes to address the specific issue at hand
  - preserve the existing patterns and conventions of the codebase

## Handling Files with Formatting Issues

When encountering files with mixed tabs/spaces or other formatting problems:
- **Do NOT** just read the file and wait for manual fixing
- **Do** use the Edit tool to fix formatting issues directly
- **Do** run `go fmt ./...` after making any edits to ensure consistent formatting
- If a file has pervasive formatting issues (mixed tabs/spaces throughout), consider using Write tool to rewrite with correct formatting
- Always include formatting fixes in the same commit as code changes

Example workflow:
1. Read file and notice formatting issues
2. Use Edit tool to fix the content AND formatting in one operation
3. Run `go fmt ./...` to ensure consistency
4. Verify with tests

## Code Style Guidelines

### Import Organization
- Organize imports in the following order:
  1. Standard library packages first (e.g., "fmt", "context")
  2. A blank line separator
  3. Third-party packages
  4. A blank line separator
  5. Project imports (e.g., "fuku/internal/*")
- Example:
  ```go
  import (
      "context"
      "fmt"
      "os"

      "github.com/rs/zerolog"
      "go.uber.org/fx"

      "fuku/internal/config"
  )
  ```
- never alias `fuku/internal/app/errors` as `apperrors` — the package already re-exports `errors.Is`, `errors.As`, and `errors.New`, so import it directly as `"fuku/internal/app/errors"` and use `errors.Is(...)`, `errors.ErrFoo`, etc.

### Error Handling
- return errors to the caller rather than using panics
- use descriptive error messages that help with debugging
- use error wrapping: `fmt.Errorf("failed to process request: %w", err)`
- check errors immediately after function calls
- return early when possible to avoid deep nesting
- for functions that return multiple values including errors, handle both the primary result and the error appropriately
- when logging errors, include contextual information: `c.log.Error().Err(err).Msgf("Failed to run profile '%s'", profile)`

### Variable Naming
- use descriptive camelCase names for variables and functions
  - good: `serviceProcess`, `dependencyGraph`, `profileConfig`
  - bad: `sp`, `x`, `temp1`
- be consistent with abbreviations
- local scope variables can be short (e.g., "cfg" instead of "configuration")

### Function Parameters
- group related parameters together logically
- use descriptive parameter names that indicate their purpose
- consider using parameter structs for functions with many (4+) parameters
- if function returns 3 or more results, consider wrapping in result/response struct
- if function accepts 3 or more input parameters, consider wrapping in request/input struct (but never add context to struct)

### Documentation
- all exported functions, types, and methods must have clear godoc comments
- begin comments with the name of the element being documented
- godoc comments must be a single sentence without ending punctuation
- start godoc comments with capital letter but no period at end
- include additional details in parentheses within the single sentence if needed
- keep internal comments concise and only when they add value
- avoid comments that merely repeat what the code does

### Code Structure
- keep code modular with focused responsibilities
- limit file sizes to 300-500 lines when possible
- group related functionality in the same package
- use interfaces to define behavior and enable mocking for tests
- keep code minimal and avoid unnecessary complexity
- don't keep old functions for imaginary compatibility
- interfaces should be defined on the consumer side (idiomatic Go)
- aim to pass interfaces but return concrete types when possible
- consider nested functions when they simplify complex functions

### Code Layout
- keep cyclomatic complexity under 30
- function size preferences:
  - keep functions focused on a single responsibility
  - break down large functions (100+ lines) into smaller, logical pieces
  - avoid functions that are too small if they reduce readability
- keep lines readable; while gofmt doesn't enforce line length, consider breaking very long lines for clarity
- manage conditional complexity:
  - never nest if blocks — `if { if { } }` is forbidden; flatten with guard clauses (early return/continue)
  - use if statements only for guard clauses (early returns/continue) or simple single-level branches
  - for multiple conditions or state-based logic, prefer switch statements
  - for many discrete values, prefer switch statements over long if-else-if chains
  - extract complex conditions into well-named boolean functions or variables
  - use context structs or functional options instead of multiple boolean flags
- for CLI command processing, use switch statements with multiple conditions per case (e.g., `case cmd == "help" || cmd == "--help" || cmd == "-h":`)
- when handling default values, check for empty strings and provide sensible defaults (e.g., `if profile == "" { profile = config.DefaultProfile }`)
- for functions that need to be testable, separate return values from system calls: return exit codes and errors instead of calling os.Exit() directly

### Testing
- write thorough tests with descriptive names (e.g., `Test_Runner_ResolvesComplexDependencies`)
- prefer subtests or table-based tests, using testify
- use table-driven tests ONLY when testing multiple scenarios (2+ test cases) with different inputs/outputs; for single test cases, use plain test functions instead of table tests with one entry
- table-driven tests for testing multiple cases with the same logic
- test both success and error scenarios
- mock external dependencies to ensure unit tests are isolated and fast
- aim for at least 80% code coverage
- keep tests compact but readable
- if test has too many subtests, consider splitting it to multiple tests
- never disable tests without a good reason and approval
- important: never update code with special conditions to just pass tests
- don't create new test files if one already exists matching the source file name
- add new tests to existing test files following the same naming and structuring conventions
- don't add comments before subtests, t.Run("description") already communicates what test case is doing
- never use godoc-style comments for test functions
- for main package testing, extract testable functions from main() and runApp() to enable unit testing
- skip integration tests that would cause hanging or require subprocess execution (e.g., os.Exit(), long-running FX apps)
- when testing CLI applications, use simple skip statements for complex integration scenarios to maintain test suite stability
- for mocking external dependencies:
  - create a local interface in the package that needs the mock
  - generate mocks using `go.uber.org/mock` with full path:
    ```bash
    mockgen -source=internal/path/to/file.go -destination=internal/path/to/file_mock.go -package=packagename
    ```
  - the mock should be located alongside the source file
  - always use mockgen-generated mocks, not testify mock
  - do NOT add `//go:generate` directives to source files
- for testing functions that can fail due to external dependencies (like config loading), use `t.Skip()` with descriptive messages rather than failing the test
- use descriptive test names that explain the scenario being tested (e.g., "No arguments - default profile", "Run command with profile and --no-ui")
- when testing CLI return values, test both exit codes and error conditions separately in table test fields like `expectedExit` and `expectedError`
- for testable code extraction: separate business logic from system calls (os.Exit, os.Args) by creating internal functions that can be unit tested
- always test multiple command variations (e.g., "help", "--help", "-h") to ensure all aliases work correctly
- never inline table test cases; always use multi-line format:
  ```go
  // BAD - inline format
  {name: "test case", input: "value", expected: true},

  // GOOD - multi-line format
  {
      name:     "test case",
      input:    "value",
      expected: true,
  },
  ```

## Git Workflow

### After merging a PR
```bash
# switch back to the master branch
git checkout master

# pull latest changes including the merged PR
git pull

# delete the temporary branch (might need -D for force delete if squash merged)
git branch -D feature-branch-name
```

## Commonly Used Libraries
- dependency injection: `go.uber.org/fx`
- CLI framework: `github.com/spf13/cobra`
- configuration: `github.com/spf13/viper`
- logging: `github.com/rs/zerolog`
- testing: `github.com/stretchr/testify`
- mock generation: `go.uber.org/mock`
- TUI framework: `github.com/charmbracelet/bubbletea`
- TUI components: `github.com/charmbracelet/bubbles`
- TUI styling: `github.com/charmbracelet/lipgloss`
- FSM: `github.com/looplab/fsm`
- process monitoring: `github.com/shirou/gopsutil/v4`

## Formatting Guidelines
- always use `go fmt` for code formatting
- run `go generate` for mock generation
- respect `.editorconfig` settings when editing files:
  - Go files use tabs for indentation (`indent_style = tab`)
  - Project preference: `tab_width = 2` (display only, doesn't affect file contents)
  - All files: UTF-8 encoding, LF line endings, final newline, trim trailing whitespace
- when using Edit tool, preserve existing formatting and indentation style
- `gofmt` handles actual file formatting regardless of editor display settings

## Logging Guidelines
- use structured logging with zerolog
- never use fmt.Printf for logging, only log methods

## Configuration Format

Fuku uses a YAML configuration file (`fuku.yaml`) with the following structure:

```yaml
version: 1

services:
  service-name:
    dir: path/to/service
    tier: foundation
    logs:
      output: [stdout, stderr]

profiles:
  default: "*"
  backend: [service1, service2]

logging:
  format: console
  level: info
```

## Working with Services

- services are defined with a directory path and optional tier
- profiles allow grouping services for batch operations
- tiers determine startup ordering (services in earlier tiers start first)
- each service runs `make run` in its specified directory
- services must have a Makefile with a `run` target
- environment files (`.env.development`) are automatically detected and passed via ENV_FILE

## Example Configuration

Based on the complex microservices example provided, fuku can handle large-scale service orchestration with:
- multiple API services
- tier-based startup ordering
- service grouping via profiles
- centralized logging configuration

The tool is particularly useful for development environments where you need to start multiple services with a single command.
