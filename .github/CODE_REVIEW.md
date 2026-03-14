# Mandatory Pull Request Review Policy

## Purpose

This document defines **mandatory AI code review and code generation rules** for the **fuku** repository — a Go CLI orchestrator for managing local development services.

AI tools (GitHub Copilot, Copilot Chat, or any LLM-based assistant) **MUST** follow these rules when:

- Reviewing pull requests
- Generating new code
- Refactoring existing code
- Writing tests
- Suggesting architectural changes

If a violation is detected, the AI **MUST explicitly report it**.

---

# 1. Severity Model

All findings MUST be classified as:

- **BLOCKER** – Must be fixed before merge
- **MAJOR** – Should be fixed before merge
- **MINOR** – Improvement suggestion
- **OPTIONAL / RECOMMENDATIONS** – Valuable improvement but not required for merge

Pull requests **must not be approved if BLOCKERS exist**.

---

# 2. Pull Request Hygiene

## 2.1 PR Summary / Intent

Every PR must contain a **Summary** section describing the **intent of the change**.

The summary must explain:

- what the PR aims to achieve
- the meaningful changes introduced by the PR

Reviewers MUST compare the summary with the **actual diff** and detect:

- missing description of meaningful changes
- changes that go beyond the stated intent of the PR
- unclear or ambiguous PR summaries
- unrelated functionality not covered by the stated intent

Meaningful changes include: CLI behavior changes, configuration format changes, process lifecycle changes, event bus message changes, concurrency logic, error handling, logging or observability, TUI behavior, and new or removed tests.

If the intent is unclear, reviewers must **request clarification instead of guessing**.

---

## 2.2 PR Naming (MAJOR)

PR title must follow conventional commit format:

```
type(scope): Short description
```

Types: `feat`, `fix`, `refactor`, `chore`, `docs`, `test`, `perf`

Rules:

- type must be lowercase
- scope is optional but recommended
- description must be concise (under 70 characters total)
- do not use branch-style names as PR titles

---

## 2.3 Branch Naming (OPTIONAL)

Branch format:

```
type/short-description
```

Types: `feature`, `fix`, `chore`, `refactor`, `docs`, `hotfix`

Rules:

- lowercase words
- hyphen separated
- do not copy full PR title as branch name

---

## 2.4 Commit Quality (MAJOR)

Commits must:

- follow conventional commit format: `type(scope): description`
- be atomic (one logical change per commit)
- contain meaningful messages (not generic `fix`, `update`, `wip`)

---

# 3. Project Layout (MAJOR)

The project follows this structure:

- `cmd/` – application entrypoint
- `internal/app/` – application packages (bus, cli, runner, watcher, ui, etc.)
- `internal/config/` – configuration loading, data structures, logger, sentry
- `internal/app/errors/` – application-specific error definitions
- `e2e/` – end-to-end tests

Rules:

- all application code lives under `internal/`
- avoid circular dependencies
- group related functionality into focused packages
- new packages must follow existing naming conventions

---

# 4. Architecture

## 4.1 Dependency Injection (BLOCKER if violated)

All components MUST be wired through **Uber FX** modules (`fx.Provide`, `fx.Invoke`).

Rules:

- never instantiate dependencies manually in application code
- use FX lifecycle hooks (`fx.OnStart`, `fx.OnStop`) for initialization and cleanup
- new components must be registered in their package's FX module

Manually wiring dependencies outside FX → **BLOCKER**.

---

## 4.2 Event Bus (BLOCKER if violated)

The event bus (`internal/app/bus`) is the backbone for cross-cutting concerns.

Rules:

- **all cross-cutting concerns must subscribe to the bus, never inline into business logic**
- business logic publishes events; observers react to them
- metrics, logging side-effects, UI updates, and notifications must be bus subscribers
- every bus event must carry enough data for any subscriber to act without calling back into the publisher
- check if an existing event already carries the needed data before creating a new event type

Inlining cross-cutting logic into business code → **BLOCKER**.

Exception: code that runs before the bus is created (e.g., CLI parsing) or purely local data with no corresponding event.

---

## 4.3 Interfaces (MAJOR)

Rules:

- interfaces must be defined on the **consumer side** (idiomatic Go)
- do **not** prefix interfaces with `I`
- one interface = one implementation = one mock
- do not use the Factory pattern — direct construction only
- prefer capability-based naming (`Runner`, `Logger`, `Pool`)

---

# 5. Go Development Guidelines

## 5.1 General Principles (MAJOR)

- Prefer clarity and simplicity
- Keep the happy path left-aligned
- Use early returns and guard clauses
- Use zero values where possible
- Prefer descriptive names
- Document exported types and functions with godoc (single sentence starting with the element name, no ending period)

---

## 5.2 Control Flow (BLOCKER if violated)

Rules:

- **never nest if blocks** — `if { if { } }` is forbidden; use guard clauses to flatten
- **never use goto**
- prefer early returns to reduce nesting
- `else` is acceptable only when it improves readability
- **never use `else if`** — use `switch` statements or guard clauses instead
- for multiple conditions or state-based logic, prefer `switch` over chained conditionals
- for CLI command processing, use `switch` with multiple conditions per case

Nested if blocks → **BLOCKER**.

---

## 5.3 Function Design (MAJOR)

Rules:

- keep functions focused on a single responsibility
- break down large functions (100+ lines) into smaller logical pieces
- cyclomatic complexity must stay under 30
- if a function accepts 4+ parameters, consider a parameter struct (never add context to structs)
- if a function returns 3+ values, consider a result struct
- consider nested functions when they simplify complex logic

---

# 6. Error Handling (BLOCKER)

## 6.1 Error Wrapping

Errors must be wrapped with context using `fmt.Errorf`:

```go
return fmt.Errorf("failed to load config: %w", err)
```

The project provides `internal/app/errors` for application-specific errors — import it directly as `errors` (never alias as `apperrors`).

Rules:

- errors must always be checked immediately after function calls
- errors must not be suppressed or silently ignored
- prefer fail-fast error handling
- error messages must describe the **intent of the failed operation**, not just restate the method name
- return errors to the caller rather than using panics

Suppressing or ignoring errors → **BLOCKER**.

---

## 6.2 Error Assertions in Tests

In tests, error assertions must be checked **before** result assertions:

```go
assert.Error(t, err)      // first
assert.Nil(t, result)     // then
```

---

# 7. Logging (BLOCKER)

## 7.1 Logging Library

The project uses **zerolog** for structured logging.

Rules:

- always use zerolog — never `fmt.Print*` for logging (enforced by `forbidigo` linter)
- include contextual information in log entries: `.Err(err).Msgf("context: %s", detail)`
- use appropriate log levels: `Debug`, `Info`, `Warn`, `Error`
- `fmt.Print*` is allowed **only** in `internal/app/cli/` for direct CLI output

Using `fmt.Print*` outside CLI output → **BLOCKER**.

---

## 7.2 Observability

The project uses **Sentry** for error tracking and metrics.

Rules:

- metrics must be emitted through the bus-driven metrics collector (`internal/app/metrics`)
- never scatter metric emission calls across unrelated packages
- respect `FUKU_TELEMETRY_DISABLED` for telemetry opt-out

---

# 8. Import Organization (MAJOR)

Imports must be organized in three groups separated by blank lines:

```go
import (
    "context"
    "fmt"

    "github.com/rs/zerolog"
    "go.uber.org/fx"

    "fuku/internal/config"
)
```

Order: standard library → third-party → project imports.

Never alias `fuku/internal/app/errors` — import it directly and use as `errors`.

---

# 9. Security and Resilience (BLOCKER)

Rules:

- validate and sanitize external input (CLI arguments, configuration values, environment variables)
- use timeouts for external operations
- implement retries with backoff where necessary (configured via `retry.attempts` and `retry.backoff`)
- do not introduce command injection or path traversal vulnerabilities

---

# 10. Concurrency (BLOCKER)

Concurrency issues are always **BLOCKERS**.

Reviewers must flag:

- goroutine leaks (goroutines without clear exit paths)
- unsafe shared state (data accessed from multiple goroutines without synchronization)
- missing context propagation
- potential deadlocks
- channel misuse (sends on closed channels, unbuffered channel blocking)

Shared state must be synchronized using **mutexes or channels**.

The project provides a bounded worker pool (`internal/app/worker`) for concurrent task execution — prefer using it over ad-hoc goroutine management.

---

# 11. Resource Management (MAJOR)

Reviewers must check for:

- unclosed files, sockets, or connections
- leaked goroutines
- unbuffered or unclosed channels
- missing cleanup in FX lifecycle hooks (`fx.OnStop`)
- process handles not properly terminated

---

# 12. Breaking Change Detection (BLOCKER)

A change is breaking if it:

- removes or renames CLI commands or flags
- changes CLI output format that scripts may depend on
- modifies `fuku.yaml` configuration schema in incompatible ways
- changes signal handling behavior
- alters service startup/shutdown ordering
- modifies event bus message structures
- changes public interface method signatures

If a breaking change is detected but **not declared in the PR**, it must be marked as **BLOCKER**.

If uncertain whether a change is breaking → assume it **may be breaking** and flag it.

---

# 13. Testing

## 13.1 Table-Driven Tests (MAJOR)

Use **table-driven test (TDT)** format when testing multiple scenarios:

```go
tests := []struct {
    name   string
    before func()
    input  string
    expect bool
}{
    {
        name:  "success case",
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
```

Rules:

- use TDT whenever testing multiple scenarios for the same function
- do not use multiple standalone `t.Run()` blocks inside a single test function — use TDT instead
- for single test cases where TDT is not possible, use `Test_<MethodName>_<TestCase>` naming
- always use multi-line format for table entries (never inline)
- mocks are created at test function level; expectations set in `before()` per case
- do not add comments before subtests — `t.Run("description")` is sufficient
- never use godoc-style comments on test functions

---

## 13.2 Test Package Convention

Tests use the **same package** as the source code:

```go
package runner  // not package runner_test
```

---

## 13.3 Mock Generation (MAJOR)

Mocks must be generated using `go.uber.org/mock`:

```bash
mockgen -source=internal/path/file.go -destination=internal/path/file_mock.go -package=pkgname
```

Rules:

- mocks are stored alongside source files as `*_mock.go`
- never modify generated mock files manually
- never add `//go:generate` directives to source files
- always use mockgen, never testify mock
- testify is for **assertions only** (`assert`, `require`)

---

## 13.4 Test Data (MAJOR)

Rules:

- prefer deterministic test inputs
- do not generate invalid inputs using random generators
- test names must be descriptive and explain the scenario being tested

---

# 14. Nolint Directives (MAJOR)

`//nolint` directives must:

- be placed on the **line above** the code they apply to (never inline)
- include an **explanation** of why the lint is suppressed
- specify the **exact linter** being suppressed

```go
//nolint:errcheck // Close errors are non-actionable in cleanup
file.Close()
```

Enforced by the `nolintlint` linter.

---

# 15. Code Organization (MAJOR)

Rules:

- group code by domain responsibility
- avoid mixing unrelated functionality in the same package
- reuse existing constants and error definitions
- limit file sizes to 300-500 lines when possible
- do not leave commented-out code in place
- do not add comments describing changes or history — comments describe current state only

---

# 16. Change Scope (MAJOR)

PRs should **minimize blast radius**.

Rules:

- avoid modifying unrelated code
- avoid adding functionality not required to achieve the stated goal
- do not add docstrings, comments, or type annotations to unchanged code
- do not refactor surrounding code unless it is the stated purpose of the PR

---

# 17. Simplicity

Prefer **simpler solutions over clever implementations**.

Rules:

- do not create abstractions unless they are needed (YAGNI)
- three similar lines of code is better than a premature abstraction
- do not build for hypothetical future requirements
- do not add error handling for scenarios that cannot happen
- if a problem can be solved in a simpler way, recommend the simpler approach

---

# 18. Code Review Priorities

Review focus priority:

1. correctness
2. security
3. reliability
4. performance
5. style

Hotfix PRs should prioritize correctness over stylistic feedback.

---

# 19. AI Review Output Format (MANDATORY)

When reviewing a PR, the output must follow this structure:

```
## PR METADATA
- (Rule 2.2) PR title: ...
- (Rule 2.3) Branch name: ...
- (Rule 2.4) Commit quality: ...

## BLOCKERS
- [file:line] (Rule X.Y) Description
  → Suggested fix

## MAJOR ISSUES
- [file:line] (Rule X.Y) Description
  → Suggested fix

## MINOR IMPROVEMENTS
- [file:line] Description
  → Suggestion

## OPTIONAL / RECOMMENDATIONS
- [file:line] Description
  → Suggestion

## PR SUMMARY / INTENT MISMATCH
- ...

## BREAKING CHANGE DETECTED
Yes / No
If yes → explanation of what changed and potential impact

## TEST COVERAGE IMPACT
Increased / Same / Decreased / Unknown

## VERDICT
APPROVE / REQUEST CHANGES / NEEDS DISCUSSION
```

Rules:

- code findings must reference a specific file and line number
- PR metadata findings (title, branch, commits) use the **PR METADATA** section without file:line references
- every finding at BLOCKER or MAJOR level must include the violated rule number
- group findings by file when multiple issues exist in the same file
- include a concrete fix suggestion for all BLOCKERS and MAJOR issues
- if no issues are found in a section, write "None" instead of omitting the section

---

# Final Rule

If uncertain about risk:

- assume risk exists
- flag for review
- never silently approve unsafe changes
