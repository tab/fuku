# Fuku Development Guide

## Project Overview

**Fuku** is a lightweight CLI orchestrator for running and managing multiple local services in development environments. Designed for speed, simplicity, and readability. Key features:

- Service orchestration with tier-based startup ordering
- Concurrent service execution with proper startup ordering
- Process lifecycle management with signal handling
- Simple YAML configuration format
- Structured logging with zerolog
- Dependency injection with Uber FX
- Clean architecture with interfaces and mocks

Package layout, interfaces, and execution flow are derivable from the code — read the source.

## Area guides

This file covers the Go application. Four directories carry their own guide,
loaded when you work in them:

- `e2e/CLAUDE.md`: the subprocess suite, its fixtures, and why no test runs in parallel
- `docs/CLAUDE.md`: the Astro site and its Pages deployment
- `plugins/jetbrains/CLAUDE.md`: the Kotlin plugin, ktlint and the version properties
- `examples/bookstore/CLAUDE.md`: the playground the root `fuku.yaml` drives, and why nothing checks it

## Skills

Procedural workflows live in `.claude/skills/`, loaded on demand:

- `verify` — verification loop (format, lint, vet, test, race, e2e) before committing
- `generate-mock` — generate or regenerate a gomock mock
- `config` — `fuku.yaml` configuration reference
- `add-test` — write tests using TDT with the mocks-once pattern

## Hooks

`.githooks/` holds the two checks that run before the code leaves the machine.
Turn them on once per clone:

```
git config core.hooksPath .githooks
```

- `commit-msg` rejects a subject that is not a scoped Conventional Commit (`feat(ui): Add the aside panel`, imperative, capitalized, no trailing period) and any AI attribution in the message
- `pre-push` runs `make check` when Go moved, `make lint:plugin` when the plugin moved, the Astro build when `docs/` moved, then the spec drift check; the race detector and the e2e suite stay in CI, where nobody is waiting on them
- push with `--no-verify`, or set `SKIP_VERIFY=1`, when you mean to skip it
- the `Title & commits` and `Spec drift` jobs in `checks.yaml` repeat both on the pull request, so a clone without the hooks installed still gets caught

## Primary Guidelines

- provide brutally honest and realistic assessments of requests, feasibility, and potential issues. no sugar-coating. no vague possibilities where concrete answers are needed
- always operate under the assumption that the user might be incorrect, misunderstanding concepts, or providing incomplete/flawed information. critically evaluate statements and ask clarifying questions when needed
- state assumptions explicitly when proceeding without asking, and surface multiple interpretations rather than silently picking one
- make surgical changes only — do not "improve" adjacent code, comments, or formatting that isn't part of the task; remove only imports or variables your own edits orphaned, not pre-existing dead code
- don't be flattering or overly positive. be honest and direct
- we work as equal partners and treat each other with respect as two senior developers with equal expertise and experience
- prefer simple and focused solutions that are easy to understand, maintain, and test
- don't overthink solutions — implement the simplest thing that works, then iterate

## Architecture Guidelines

### Dependency Injection with FX
- **always use Uber FX for dependency injection** — non-negotiable
- all components must be wired through FX modules (`fx.Provide`, `fx.Invoke`)
- never instantiate dependencies manually in application code; let FX handle the wiring
- use FX lifecycle hooks (`fx.OnStart`, `fx.OnStop`) for component initialization and cleanup

### Interfaces and Mocks
- **always define interfaces for dependencies** — required for FX injection and testability
- interfaces should be defined on the consumer side (idiomatic Go)
- never prefix interfaces with `I`; prefer capability-based names (`Runner`, `Pool`, `Logger`)
- every interface must have a corresponding mock; see `generate-mock`

### Event Bus as the Communication Backbone
- **all cross-cutting concerns must subscribe to the bus, never inline into business logic** — non-negotiable
- the event bus (`app/bus`) is the single source of truth for what happened in the system; business logic publishes events, observers react
- when adding a feature that reacts to something happening elsewhere (metrics, logging, UI updates, notifications), create a bus subscriber — never add the logic directly to the code that triggers the event
- before inventing a new event type, check whether an existing bus event already carries the data you need; extend the existing event struct
- every bus event struct must carry enough data for any subscriber to act without calling back into the publisher
- canonical examples: `app/metrics` (Sentry metrics emitted from one place) and `app/relay` (forwards bus events to the log broadcaster)
- only inline cross-cutting calls when no bus exists yet at that point in the lifecycle (e.g., CLI commands that run before the bus is created) or when the data is purely local and has no corresponding event

### Keep It Simple
- **do not create abstractions unless they are needed** — YAGNI
- **never use the Factory pattern** — we always have exactly one implementation per interface, so factories add unnecessary indirection
- one interface = one implementation = one mock
- if tempted to add a factory, abstract base class, or generalization, stop and ask if it's actually needed right now
- prefer concrete, straightforward code over clever abstractions
- don't build for hypothetical future requirements; solve the current problem
- don't add error handling, validation, or fallbacks for scenarios that cannot happen

### Styles Live in `components` Only
- **never call `lipgloss.NewStyle()` outside `internal/app/ui/components/theme.go` or `internal/app/ui/components/styles.go`** — these two files are the single source of truth for all styles
- theme-dependent styles (reading any `lipgloss.LightDarkFunc` value, palette color, or `theme.Bg*`/`theme.Fg*`) belong in `theme.go` and are exposed as fields on `Theme`
- theme-independent styles (pure spacing, padding, margins, fixed-color borders) belong in `styles.go` as package-level `var`s
- **never wrap a render with an inline style** — `lipgloss.NewStyle().MarginTop(1).Render(x)`, `lipgloss.NewStyle().Background(m.theme.BgSelection).Render(x)`, and the like are forbidden outside the two allowlisted files
- the rule applies to tests too — if a test needs a foreground-bearing style fixture, reuse an existing `var` (e.g., `SpinnerStyle`) instead of constructing one inline
- if the style you need does not exist, add it to `theme.go` or `styles.go` with a descriptive semantic name (e.g., `SelectionBgStyle`, `ContentTopMarginStyle`) and reference it from the call site
- `lipgloss.Style` as a struct field type and `lipgloss.Width(...)` measurement calls are fine anywhere — the rule is only about *constructing* styles via `NewStyle()`
- Audit (should return zero matches): `grep -rn 'lipgloss\.NewStyle()' --include='*.go' internal/ cmd/ | grep -v 'components/theme.go\|components/styles.go'`

## Code Style Guidelines

### Import Organization
- stdlib first, blank line, third-party, blank line, project imports
- example:
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
- never alias `fuku/internal/app/errors` as `apperrors` — the package re-exports `errors.Is`, `errors.As`, and `errors.New`; import it as `"fuku/internal/app/errors"` and use `errors.Is(...)`, `errors.ErrFoo`, etc.

### Error Handling
- return errors to the caller rather than panicking
- use descriptive error messages
- wrap with `fmt.Errorf("failed to process request: %w", err)`
- check errors immediately after function calls
- return early to avoid deep nesting
- when logging errors, include context: `c.log.Error().Err(err).Msgf("Failed to run profile '%s'", profile)`

### Variable Naming
- descriptive camelCase for variables and functions (`serviceProcess`, not `sp`)
- be consistent with abbreviations
- local-scope variables can be short (`cfg` instead of `configuration`)

### Function Parameters
- group related parameters logically
- use descriptive names
- 4+ parameters → consider a parameter struct
- 3+ return values → consider a result struct
- 3+ input parameters → consider an input struct (but never include `context` in a struct); FX constructors are exempt — named deps are clearer than a `Params` struct

### Service Identifier Convention
- **always identify a service by its `ID` (UUID) across package boundaries** — never by `Name`
- canonical identity is the `ID` field of `bus.Service{ID, Name}`; `Name` is the human-readable label from `fuku.yaml`, display only
- consumers that identify a service in an API (registry lookups, store snapshots, command dispatch, cross-package caches like `dotenv.Loader.Env(id)`) MUST accept and return the `ID`
- the only exception is `*config.Config.Services`, which is YAML-keyed by `Name` — translate `ID` → `Name` at the package boundary (using `bus.Service.Name` from the event payload) before reading config
- parameter naming: `id string` (lowercase, no `service` prefix) — matches `registry.Store.Service(id string)`, `bus.Service.ID`, etc.

### Documentation
- all exported functions, types, and methods must have clear godoc comments
- begin with the name of the element being documented
- single sentence, capital letter, no period at end
- include extra details in parentheses within the single sentence
- internal comments only when they add value; avoid restating what code already says

### Code Structure
- modular, focused responsibilities
- file sizes 300-500 lines when possible
- group related functionality in the same package
- use interfaces to define behavior; pass interfaces, return concrete types when possible
- don't keep old functions for imaginary compatibility
- consider nested functions when they simplify complex functions

### Code Layout
- cyclomatic complexity under 30
- break down 100+ line functions into smaller logical pieces; avoid over-tiny functions that hurt readability
- **never nest `if` blocks** — `if { if { } }` is forbidden; flatten with guard clauses (early return/continue)
- **never use `else if`** — use `switch` statements or guard clauses
- never use `goto`
- prefer early returns; `else` is acceptable when it improves readability
- for multi-condition CLI dispatch, prefer `case cmd == "help" || cmd == "--help" || cmd == "-h":`-style switches
- extract complex conditions into well-named boolean functions or variables
- prefer context structs or functional options over multiple boolean flags
- separate return values from system calls — return exit codes and errors instead of calling `os.Exit()` directly

### Testing
- see `add-test` for TDT structure, coverage target, mocking, and test-file conventions
- never disable tests without a reason and approval
- never modify code with special conditions just to make tests pass

## Logging Guidelines
- use structured logging with zerolog
- never use `fmt.Printf` for logging — only log methods
- `fmt.Print*` and `fmt.Fprint*` are fine for non-logging uses: direct CLI output (`internal/app/cli/`), pre-logger bootstrap stderr (`cmd/main.go`, `internal/config/sentry/`), the log-writer implementation itself (`internal/app/render/`), and buffer/string formatting (TUI layout in `internal/app/ui/`)
- metrics are emitted only through the bus-driven collector (`internal/app/metrics`) — never scatter `sentry.NewMeter` calls across packages
- respect `FUKU_TELEMETRY_DISABLED` for telemetry opt-out

## Concurrency & Resource Safety
- every goroutine must have a clear exit path — context cancellation or channel signal
- shared state accessed from multiple goroutines must be synchronized with mutexes or channels
- prefer the bounded worker pool (`internal/app/worker`) over ad-hoc goroutine management
- propagate `context.Context` through call chains; never store context in structs (exception: UI components under `internal/app/ui/`)
- close files, sockets, channels, and connections you open
- `fx.OnStop` hooks must clean up what `fx.OnStart` allocates
- no sends on closed channels; watch for unbuffered-channel deadlocks
- signal handling for process management must trap SIGINT and SIGTERM; SIGKILL is a last-resort escalation we send to child processes, never something we handle

## Security
- validate and sanitize external input (CLI arguments, configuration values, environment variables)
- use timeouts for external operations
- implement retries with backoff where necessary (configured via `retry.attempts` and `retry.backoff`)
- avoid command injection and path traversal vulnerabilities

## Important Workflow Notes
- always run `verify` before committing
- never put any mention of Claude or Claude Code in commit messages
- never include "Test plan" sections in PR descriptions
- comments describe the current state and purpose of the code, never its history or evolution
- after important functionality is added, update `README.md` or `ARCHITECTURE.md`
- when merging master into an active branch, make sure both branches are pulled and up to date first
- don't leave commented-out code in place
- use `gh` for GitHub work
- `//nolint` directives go on the line above (never inline), must include an explanation, and specify the exact linter (e.g., `//nolint:errcheck // Close errors are non-actionable in cleanup`)
- before significant refactoring, ensure all tests pass; consider a new branch
- when refactoring or fixing failing tests: don't redesign architecture, focus on minimal changes, preserve existing patterns; if stuck, report and ask for guidance

## Handling Files with Formatting Issues
When a file has mixed tabs/spaces or other formatting problems:
- do NOT just read and wait for manual fixing
- use Edit to fix formatting directly; for pervasive issues, rewrite with Write
- run `make fmt` after edits
- always include formatting fixes in the same commit as the code change

## Formatting Guidelines
- always use `make fmt` for code formatting (wraps `gofmt`)
- respect `.editorconfig`:
  - Go files use tabs for indentation (`indent_style = tab`)
  - `tab_width = 2` (display only)
  - UTF-8, LF line endings, final newline, trim trailing whitespace
- when using Edit, preserve existing formatting and indentation

## Commonly Used Libraries
- dependency injection: `go.uber.org/fx`
- CLI framework: `github.com/spf13/cobra`
- configuration: `github.com/spf13/viper`
- environment files: `github.com/joho/godotenv`
- logging: `github.com/rs/zerolog`
- error tracking: `github.com/getsentry/sentry-go`
- testing: `github.com/stretchr/testify`
- mock generation: `go.uber.org/mock`
- TUI framework: `charm.land/bubbletea/v2`
- TUI components: `charm.land/bubbles/v2`
- TUI styling: `charm.land/lipgloss/v2`
- FSM: `github.com/looplab/fsm`
- process monitoring: `github.com/shirou/gopsutil/v4`
- semver comparison: `golang.org/x/mod/semver`
