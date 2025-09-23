# Fuku Development Guide

## Project Overview

**Fuku** is a lightweight CLI orchestrator for running and managing multiple local services in development environments. It's designed for speed, simplicity, and readability. Key features include:

- Service orchestration with dependency resolution
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
   - **cli/** - Command-line interface parsing and command handling
   - **runner/** - Service orchestration, process management, and dependency resolution
   - **config/** - Configuration loading, parsing, and data structures
   - **errors/** - Application-specific error definitions

### Key Interfaces and Abstractions

1. **runner.Runner** - Core abstraction for service orchestration:
   ```go
   type Runner interface {
       Run(ctx context.Context, scope string) error
   }
   ```

2. **cli.CLI** - Interface for command-line operations:
   ```go
   type CLI interface {
       Run(args []string) error
   }
   ```

3. **logger.Logger** - Structured logging interface using zerolog

### Execution Flow

1. **CLI Entry Point** (`cmd/main.go`)
   - Refactored with testable functions: `runApp()`, `loadConfig()`, `createApp()`, `createFxLogger()`
   - Loads configuration from `fuku.yaml` using Viper
   - Initializes FX container with all dependencies
   - Starts application lifecycle

2. **Application Container** (`internal/app/app.go`)
   - Manages application lifecycle with FX hooks
   - Coordinates CLI execution with dependencies
   - Handles graceful shutdown

3. **Command Processing** (`internal/app/cli/cli.go`)
   - Parses command-line arguments for run, help, version commands
   - Delegates service execution to runner with specified scope
   - Handles unknown commands with appropriate error messages

4. **Service Orchestration** (`internal/app/runner/runner.go`)
   - Resolves service dependencies using topological sort
   - Starts services in dependency order with 2-second delays
   - Manages process lifecycle with signal handling (SIGINT, SIGTERM)
   - Streams service logs with prefixed output format
   - Stops services in reverse order on shutdown

### Configuration Capabilities

1. **Service Definition**
   - Directory-based service configuration
   - Dependency specification with `depends_on` arrays
   - Automatic environment file detection (`.env.development`)
   - Makefile-based service execution (`make run`)

2. **Scope Management**
   - Logical grouping of services for batch execution
   - Include lists defining services per scope
   - Default scope support for common configurations

3. **Logging Configuration**
   - Console and JSON format support
   - Configurable log levels (debug, info, warn, error)
   - Service-specific log streaming with prefixes

4. **Dependency Resolution**
   - Topological sort for startup ordering
   - Circular dependency detection
   - Missing service validation

### Testing Patterns

1. **Mock Generation**
   - Uses `go.uber.org/mock` for interface mocking
   - Generated mocks with `//go:generate` directives
   - Separate mock files for each interface

2. **Test Structure**
   - Table-driven tests with subtests using testify
   - Comprehensive error case coverage
   - Output capturing for CLI command testing
   - Mock expectation setup and verification
   - Entry point testing with extracted testable functions
   - Integration test skipping for complex application lifecycle scenarios

3. **Test Coverage**
   - CLI command parsing and execution: ~68.3%
   - Service dependency resolution algorithms: ~87.1%
   - Main application entry point: ~50.0%
   - Application container lifecycle: ~77.8%
   - Configuration loading: 100.0%
   - Logger implementation: ~29.2%
   - Error handling and edge cases
   - Mock-based isolation testing

### Current Test Files
- `cmd/main_test.go` - Tests for entry point functions and FX application creation
- `internal/app/app_test.go` - Application container and lifecycle testing
- `internal/app/cli/cli_test.go` - CLI argument parsing and command execution
- `internal/app/runner/runner_test.go` - Service orchestration and dependency resolution
- `internal/config/config_test.go` - Configuration loading and parsing
- `internal/config/logger/logger_test.go` - Logger implementation testing
- `internal/app/errors/` - Error definitions (no test file - contains only constants)

## Primary Guidelines

- provide brutally honest and realistic assessments of requests, feasibility, and potential issues. no sugar-coating. no vague possibilities where concrete answers are needed.
- always operate under the assumption that the user might be incorrect, misunderstanding concepts, or providing incomplete/flawed information. critically evaluate statements and ask clarifying questions when needed.
- don't be flattering or overly positive. be honest and direct.
- we work as equal partners and treat each other with respect as two senior developers with equal expertise and experience.
- prefer simple and focused solutions that are easy to understand, maintain and test.

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
- use `go:generate` for generating mocks, never modify generated files manually
- mocks are generated with `go.uber.org/mock` and stored alongside source files
- after important functionality added, update README.md accordingly
- when merging master changes to an active branch, make sure both branches are pulled and up to date first
- don't leave commented out code in place
- if working with github repos use `gh`
- avoid multi-level nesting
- avoid multi-level ifs, never use else if
- never use goto
- avoid else branches if possible
- write tests in compact form by fitting struct fields to a single line (up to 130 characters)
- before any significant refactoring, ensure all tests pass and consider creating a new branch
- when refactoring, editing, or fixing failed tests:
  - do not redesign fundamental parts of the code architecture
  - if unable to fix an issue with the current approach, report the problem and ask for guidance
  - focus on minimal changes to address the specific issue at hand
  - preserve the existing patterns and conventions of the codebase

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

### Error Handling
- return errors to the caller rather than using panics
- use descriptive error messages that help with debugging
- use error wrapping: `fmt.Errorf("failed to process request: %w", err)`
- check errors immediately after function calls
- return early when possible to avoid deep nesting

### Variable Naming
- use descriptive camelCase names for variables and functions
  - good: `serviceProcess`, `dependencyGraph`, `scopeConfig`
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
- include usage examples for complex functions
- document any non-obvious behavior or edge cases
- all comments should be lowercase, except for godoc public functions and methods
- IMPORTANT: all comments except godoc comments must be lowercase, test messages must be lowercase, log messages must be lowercase

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
  - aim for functions around 50-60 lines when possible
  - don't break down functions too small as it can reduce readability
  - maintain focus on a single responsibility per function
- keep lines under 130 characters when possible
- avoid if-else chains and nested conditionals:
  - never use long if-else-if chains; use switch statements instead
  - prefer early returns to reduce nesting depth
  - extract complex conditions into separate boolean functions or variables
  - use context structs or functional options instead of multiple boolean flags

### Testing
- write thorough tests with descriptive names (e.g., `Test_Runner_ResolvesComplexDependencies`)
- prefer subtests or table-based tests, using testify
- use table-driven tests for testing multiple cases with the same logic
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
  - generate mocks using `go.uber.org/mock` with: `//go:generate mockgen -source=file.go -destination=file_mock.go`
  - the mock should be located alongside the source file
  - always use mockgen-generated mocks, not testify mock

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
- configuration: `github.com/spf13/viper`
- logging: `github.com/rs/zerolog`
- testing: `github.com/stretchr/testify`
- mock generation: `go.uber.org/mock`

## Formatting Guidelines
- always use `go fmt` for code formatting
- run `go generate` for mock generation

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
    depends_on: [dependency1, dependency2]

scopes:
  scope-name:
    include:
      - service1
      - service2

logging:
  format: console
  level: info
```

## Working with Services

- services are defined with a directory path and optional dependencies
- scopes allow grouping services for batch operations
- dependencies ensure services start in the correct order
- each service runs `make run` in its specified directory
- services must have a Makefile with a `run` target
- environment files (`.env.development`) are automatically detected and passed via ENV_FILE

## Example Configuration

Based on the complex microservices example provided, fuku can handle large-scale service orchestration with:
- multiple API services
- complex dependency chains
- service grouping via scopes
- centralized logging configuration

The tool is particularly useful for development environments where you need to start multiple interdependent services with a single command.
