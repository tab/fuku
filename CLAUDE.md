# Fuku Development Guide

## Project Overview

**Fuku** is a CLI orchestrator. It runs and manages local services in development. It is built for speed and simplicity.

- service startup in tiers
- process lifecycle with signal handling
- one YAML config
- structured logging with `log/slog` over a zerolog handler
- dependency injection with Uber FX
- clean architecture with interfaces and mocks

`ARCHITECTURE.md` explains the system. This file holds the rules.

## Area guides

Four directories carry their own guide. It loads when you work there.

- `e2e/CLAUDE.md`: the subprocess suite, its fixtures, and why no test runs in parallel
- `docs/CLAUDE.md`: the Astro site and its Pages deployment
- `plugins/jetbrains/CLAUDE.md`: the Kotlin plugin, ktlint and the version properties
- `examples/bookstore/CLAUDE.md`: the playground the root `fuku.yaml` drives, and why nothing checks it

## Skills

Workflows live in `.claude/skills/`. They load on demand.

- `add-test`: write tests with TDT and the mocks-once pattern
- `generate-mock`: generate or regenerate a gomock mock
- `verify`: format, lint, vet, test, race, e2e, docs before a commit
- `config`: the `fuku.yaml` reference

## Hooks

`.githooks/` holds the two checks that run before code leaves the machine. Turn them on once per clone:

```
git config core.hooksPath .githooks
```

- `commit-msg` rejects a subject that is not a scoped Conventional Commit.
  The form is `feat(ui): Add the aside panel`: imperative, capitalized, no trailing period. It also rejects any AI attribution
- `pre-push` runs `make check` when Go moved, `make lint:plugin` when the plugin moved, the Astro build when `docs/` moved, `make docs` when `ARCHITECTURE.md`, the backlog or a package README moved, then the spec drift check.
  The hook skips the race detector and the e2e suite. `verify` and CI run them
- push with `--no-verify`, or set `SKIP_VERIFY=1`, to skip it on purpose
- `Conventions` in `conventions.yaml` and `Spec` in `checks.yaml` repeat the checks on the pull request.
  The title check has its own workflow because it must run on a title edit without cancelling the code jobs

## Primary Guidelines

- be brutally honest about requests, feasibility and problems. No sugar-coating. Give concrete answers
- assume the user may be wrong or missing information. Check statements. Ask when needed
- state assumptions when you proceed without asking. Name the other readings instead of picking one silently
- make surgical changes. Do not improve code, comments or formatting outside the task. Remove only what your own edit orphaned
- do not flatter. Be direct
- we are two senior developers. Equal partners
- prefer simple, focused solutions
- do not overthink. Implement the simplest thing that works, then iterate

## Architecture

### Layers

`internal/` is five rings. Every package belongs to one. The table is in [`ARCHITECTURE.md`](ARCHITECTURE.md#rings).

A feature's rule lives in its `app` package. The adapter beside it does the IO.
An adapter keeps only technical rules: config defaults, path safety, probe polling, debouncing.
`ARCHITECTURE.md` lists the features and their packages.

Frontends (`cli`, `tui`, `rest`) parse input, call typed core methods and present results.
They never decide health, eligibility, retries or replay rules. A new rule is an edit in the core package.
Change an adapter only for new IO, input, transport or presentation.

### Dependency rules

`depguard` in `.golangci.yaml` enforces these:

- `internal/model` imports the standard library only. `internal/contracts` adds `internal/model`. Neither imports Fx
- `internal/app/**` imports no `internal/adapters/**` and no `internal/platform/**`. It declares a consumer-owned interface.
  `bootstrap/modules` binds the implementation
- `internal/platform/**` imports no `internal/app`, `internal/adapters` or `internal/bootstrap`
- `internal/adapters/**` imports no `internal/bootstrap`. `adapters/tui/**` imports no `adapters/rest`.
  `adapters/process/**` imports no `adapters/logsocket`
- `internal/adapters/cli` imports no `adapters/tui`. The composition selects the view.
  `internal/bootstrap/*` children do not import the `bootstrap` root
- production packages import no Gomock. `*_test.go` may
- Audit (should print `0 issues.`): `golangci-lint run --enable-only depguard ./...`

### Compositions and lifetime

- one `fx.App` and one bus per invocation.
  `bootstrap.Run` parses the command, loads the config where needed and picks one composition in `bootstrap/modules`
- a component that runs during the application's lifetime is a lifecycle participant (`internal/bootstrap/lifecycle`, satisfied structurally): a `Consumer` with `Subscribe`/`Drain`, a `Producer` with `Start`/`Stop`, or the one `Command` with `Run`.
  The coordinator runs guard, telemetry, consumers, producers, command. It stops in reverse and drains the consumers before the bus closes.
  Telemetry is the one exception: a coordinator stage, not a participant.
  It starts after the guard and stops after the command is joined, just before the bus closes
- a composition hands the coordinator explicit ordered slices. Fx value groups are shuffled, so they cannot carry the order.
  A package that is both a consumer and a producer is listed in both slices
- constructors start no goroutines and register no hooks. `Start` acquires what `Stop` releases.
  The coordinator is the one Fx hook
- `lifecycle.Arbiter` records the first terminal cause and derives the exit code. It is bound to every `Reporter`/`FailureReporter` interface

## Architecture Guidelines

### Dependency Injection with FX

- **always use Uber FX** for dependency injection. Non-negotiable
- wire every component through FX modules (`fx.Provide`, `fx.Invoke`)
- never construct a dependency by hand in application code
- a package's `module.go` provides its concrete constructors.
  The composition binds them to the consumers' interfaces and projects their options. A package never binds itself:

  ```go
  // internal/app/updater/module.go — the provider
  var Module = fx.Options(
      fx.Provide(NewChecker),
  )

  // internal/bootstrap/modules/runtime.go — the interface bindings and the options projection
  fx.Provide(
      func(c *github.Client) updater.ReleaseSource { return c },
      func(log *slog.Logger) updater.Logger { return log },
      func(p model.Project) updater.Options {
          return updater.Options{Enabled: p.Updater.Enabled, Version: buildinfo.Version}
      },
  ),

  // internal/bootstrap/modules/run.go — a producer participant, listed in order
  participants.Producers = append(participants.Producers, p.Socket, p.Runtime, p.Sampler, p.Watcher, p.Checker)
  ```

- a value that exists once but serves two consumers is bound twice from the one concrete type (`*worker.Pool` → `services.Pool` and `process.Pool`).
  A second instance is a named provider with `fx.ResultTags`

### `module.go` Holds Wiring Only

- **`module.go` contains the FX `Module` var and nothing else**: `fx.Provide` of the package's constructors, `fx.Annotate` for a named instance
- logic never lives in `module.go`. A constructor that builds state or makes a decision belongs in its own file
- **never write a `module_test.go`**. It would pin the declaration order of a file meant to be reshuffled.
  The compositions are validated once, in `bootstrap/modules/base_test.go`
- test the logic in the file it lives in: `server.go` → `server_test.go`
- an ordering that carries a guarantee is an FX dependency or a position in the participant slice. Never a position in `Module`
- Audit (should return zero matches): `find . -name 'module_test.go' -not -path './vendor/*'`

### Interfaces and Mocks

- **always define interfaces for dependencies**. FX injection and tests need them
- interfaces are defined on the consumer side. The package that calls `Latest` declares `ReleaseSource`.
  The adapter that implements it never sees the interface
- never prefix an interface with `I`. Prefer capability names (`Runner`, `Pool`, `Logger`)
- constructors return concrete types (`*github.Client`, `*services.Runtime`). The composition binds them
- an interface a test mocks gets a generated `*_mock_test.go` in the consumer's package, and only then.
  A dependency no test asserts on takes a no-op stand-in. See `generate-mock` and "Mocks: one layer down" in `add-test`
- mocks are generated in package mode into the `<file>_mock_test.go` beside the `<file>_test.go` that uses them.
  They name only the interfaces those tests mock. The shared bus interfaces are mocked into `contracts_mock_test.go`.
  No `*_mock.go` exists. No package imports another package's mock
- Audit (should return zero matches): `find . -name '*_mock.go' -not -path './vendor/*'`

### Event Bus as the Communication Backbone

- **every cross-cutting concern subscribes to the bus. Never inline it into business logic**. Non-negotiable
- the bus (`platform/bus`) is the record of what happened. Business logic publishes events. Observers react
- a feature that reacts to something elsewhere (metrics, logging, UI updates) is a bus subscriber. Never add it to the code that triggers the event
- before a new event type, check whether an existing event carries the data. Extend that struct
- every event carries enough data for a subscriber to act without calling back into the publisher
- examples: `adapters/telemetry` (metrics and spans from one place) and `adapters/eventlog` (writes events to the debug log and forwards them to the log clients)
- inline a cross-cutting call only when no bus exists yet at that point (CLI code before the bus), or when the data is purely local
- a package never imports `platform/bus`.
  It takes `contracts.Publisher` (`Publish(contracts.Message) error`) and/or `contracts.Subscriber` (`Subscribe(ctx, contracts.SubscribeOptions) (contracts.Subscription, error)`).
  `base.go` binds `*bus.Bus` to both once.
  A subscription is named, marked `Required` when the package must not miss a critical message, filtered with `Types`, and driven by `contracts.Run`.
  Its `Loop.Drain` is the shutdown barrier
- criticality is a property of the message type (`contracts.MessageType.Critical()`, a fixed table with an exhaustive test). Never a per-call flag
- a critical `Publish` fails with `contracts.ErrBusOverloaded` (a required queue is full; the bus logs it and calls its `FailureReporter`) or `contracts.ErrBusClosed`.
  A core publisher routes the error to its `Reporter` (the `lifecycle.Arbiter`).
  A frontend maps it (REST → `500`/`409`, TUI → a log line).
  A non-critical publish never fails, so those sites carry `//nolint:errcheck // a non-critical publish never fails`
- raw child output stays off the bus. Process streams write to `process.LogSink`, bound to `logs.Hub`

### Keep It Simple

- **no abstraction until it is needed** (YAGNI)
- **never use the Factory pattern to choose an implementation**. There is one implementation per interface.
  The one permitted "factory" creates many values of one kind (`adapters/process.Factory` starts a child per launch).
  Fx constructs it once
- one interface = one implementation, plus a mock where a test asserts on it
- tempted to add a factory, a base class or a generalization? Stop and ask if it is needed now
- concrete code over clever abstractions
- solve the current problem. Do not build for hypothetical requirements
- no error handling, validation or fallback for a case that cannot happen

### Configuration Reaches Packages as Options

- **no package outside `adapters/config` and `bootstrap` imports the config adapter or sees the YAML structs**.
  `config.LoadPath(path)` returns `model.Config{Path, OverridePath, Project, Topology, Error}`.
  Run compositions fail on `Error` in bootstrap. Doctor reports it as findings
- a package that reads a few settings declares them in its `options.go` (`rest.Options{Listen, Token}`, `services.Options{RetryAttempts, RetryBackoff}`).
  The composition projects it from `model.Project`.
  A package that needs the service catalog takes `model.Project` and calls `Project.Service(name)`
- config defaults (the command, readiness timeouts, tier names) are applied by the adapter's projection.
  A `model.Service` has its generated ID and complete configuration when a package receives it.
  Technical constants (socket paths, probe timeouts, port retries) live in the package that owns the rule. Never in the config adapter
- Audit (should list only `internal/bootstrap` files): `grep -rl '"fuku/internal/adapters/config"' --include='*.go' internal/ | grep -v internal/adapters/config/`

### Styles Live in `terminal` Only

- **never call `lipgloss.NewStyle()` outside `internal/adapters/terminal/theme.go` or `styles.go`**
- theme-dependent styles (any `lipgloss.LightDarkFunc` value or palette color) belong in `theme.go` as fields on `Theme`
- theme-independent styles (spacing, padding, margins, fixed-color borders) belong in `styles.go` as package `var`s
- **never wrap a render with an inline style**. `lipgloss.NewStyle().MarginTop(1).Render(x)` is forbidden outside the two files
- the rule applies to tests. A test that needs a style fixture reuses an existing `var`
- a missing style is added to `theme.go` or `styles.go` with a semantic name (`SelectionBgStyle`)
- `lipgloss.Style` as a field type and `lipgloss.Width(...)` are fine anywhere. The rule is about constructing styles
- Audit (should return zero matches): `grep -rn 'lipgloss\.NewStyle()' --include='*.go' internal/ cmd/ | grep -v 'terminal/theme.go\|terminal/styles.go'`

## Code Style Guidelines

### Import Organization

Stdlib first, blank line, third-party, blank line, project imports:

```go
import (
    "context"
    "fmt"
    "log/slog"

    "go.uber.org/fx"

    "fuku/internal/adapters/config"
)
```

### Error Handling

- return errors. Do not panic
- descriptive messages, wrapped: `fmt.Errorf("failed to process request: %w", err)`
- a sentinel that crosses a layer lives in `internal/contracts/errors.go`.
  An adapter-local sentinel lives in an `errors.go` beside its package. Match with `errors.Is`/`errors.As`
- check errors right after the call
- return early. Avoid deep nesting
- log errors with context: `c.log.Error(fmt.Sprintf("Failed to run profile '%s'", profile), "error", err)`

### Variable Naming

- descriptive camelCase (`serviceProcess`, not `sp`)
- consistent abbreviations
- short names are fine in a small scope (`cfg`)

### Constant Naming

- a value of a named type is `<Type><Value>`: `StatusRunning`, `PhaseStartup`, `FlagConfig`.
  The type part drops a trailing `Type`/`ID`/`Kind` (`CommandType` → `CommandRun`, `CheckID` → `CheckSystem`)
- an untyped group uses its noun the same way: `MetricServiceCount`, `TagArch`, `FieldCommand`, `LevelInfo`.
  Never the suffix form (`CommandField`, `InfoLevel`)
- unexported values follow the same shape (`actionStart`, `stateSettle`)
- an event is `Event<Object><State>` with a past participle or adjective (`EventServiceStopped`, `EventTierReady`).
  A command is `Command<Verb><Object>` (`CommandStartService`, `CommandStopAll`).
  The wire string is the identifier minus the kind, in snake_case (`service_stopped`, `cmd_stop_all`).
  A wire string is frozen once it ships, because the event log writes it and log clients read it.
  A renamed identifier keeps its old value with a `frozen wire value` comment (`EventSignalReceived` is `signal`, `EventPreflightKilled` is `preflight_kill`, `EventResourceSampled` is `resource_sample`, `EventAPIRequested` is `api_request`)

### Function Parameters

- group related parameters
- 3+ parameters → consider a parameter struct. Never put `context` in a struct. FX constructors are exempt
- 3+ return values → consider a result struct

### Service Identifier Convention

- **identify a service by its `ID` (UUID) across package boundaries**. Never by `Name`
- `ID` is the identity. `Name` is the label from `fuku.yaml`, for display only
- an API that identifies a service (registry lookups, snapshots, command dispatch, `environment.Store.Env(id)`) takes and returns the `ID`
- the one exception is `model.Project.Service(name)`, keyed by `Name`.
  Translate `ID` → `Name` at the boundary with `model.Service.Name` from the event payload
- the parameter is `id string`

### Documentation

- every exported function, type and method has a godoc comment. An unexported one may carry one, in the same shape
- it is one line of at most 120 characters. It begins with the name. One short sentence. Capital letter. No period at the end.
  A second line never exists. Detail belongs in the package README
- code carries no comments. A name or a smaller function says what a comment would
- a comment inside a function body names its case and stays on one line: `no-op:` for an empty branch kept on purpose,
  `sync:` for concurrency that misreads without it, `perf:` for an optimisation that must survive a cleanup,
  `ponytail:` for a known ceiling and its upgrade path
- a trailing comment on a field or constant is its godoc. It goes when the name says it
- a package `README.md` is the change guide of that package.
  It answers four questions: why it exists and where its boundary is; how it works; how a consumer uses it; what to keep true when changing it.
  A Mermaid diagram only where the flow branches. No file tables, method signatures, mocks or test lists
- docs use short, simple sentences. One idea per sentence. Bullets for lists. `README.md` is for users. `ARCHITECTURE.md` is the map.
  This file holds the rules
- Audit (should print nothing): `go run ./.github/scripts/comments`. `make lint` runs it before `golangci-lint`

### Code Structure

- modular, focused responsibilities
- files of 300-500 lines when possible
- order struct fields the way the constructor receives them: a `context.Context` first where a struct holds one, injected dependencies next, own state after, `log Logger` last.
  Constructor parameters follow the same order
- keep related fields together (a channel and the function that closes it, a client and its address)
- pass interfaces, return concrete types
- do not keep old functions for imaginary compatibility
- nested functions are fine when they simplify a complex function

### Code Layout

- cyclomatic complexity under 30
- break a 100+ line function into logical pieces. Avoid tiny functions that hurt readability
- **never nest `if` blocks**. Flatten with guard clauses
- **never use `else if`**. Use `switch` or guard clauses
- never use `goto`
- prefer early returns. `else` is fine when it reads better
- for multi-condition CLI dispatch, prefer `case cmd == "help" || cmd == "--help" || cmd == "-h":`
- extract complex conditions into named booleans
- prefer context structs or functional options over several boolean flags
- return exit codes and errors. Never call `os.Exit()` from application code

### Testing

- see `add-test` for TDT, coverage, mocking and test-file conventions
- never disable a test without a reason and approval
- never add a special case to production code to make a test pass
- Audit (should return zero matches, `go install golang.org/x/tools/cmd/deadcode@latest` once): `deadcode ./cmd/... | grep 'internal/'`
- Audit (should return zero matches): `grep -rn 'time.Sleep' --include='*_test.go' internal`

## Logging Guidelines

- the logger is `log/slog`-shaped.
  Every package declares the subset it uses of `Debug/Info/Warn/Error(msg string, args ...any)` as its own `Logger` interface.
  `*slog.Logger` satisfies it. Packages never import `internal/platform/logging` for the logger.
  The level and format constants are the one thing they take from it. Only `internal/bootstrap/modules` builds the handler with it
- the component is set at wiring, not in constructors.
  `internal/bootstrap/modules` binds `log.With("component", "PROCESS")` to each package's interface
- `internal/platform/logging` provides the zerolog-backed `slog.Handler`.
  It keeps the `component`, `message` and `service` keys the terminal writer in `internal/adapters/output` renders.
  A message is the full line. Structured args are for JSON output only
- keep messages as they are today: `log.Info(fmt.Sprintf("Started service '%s'", name))` and `log.Warn("Preflight cleanup failed", "error", err)`.
  The error goes under the `error` key
- tests pass `slog.New(slog.DiscardHandler)`. A `Logger` mock exists only where a test asserts a log call
- never use `fmt.Printf` for logging
- `fmt.Print*` and `fmt.Fprint*` are fine for output that is not logging: direct CLI output (`internal/adapters/cli/`), the init command's output on its injected stdout (`internal/adapters/config/create.go`), pre-logger bootstrap output (`internal/bootstrap/`, `internal/adapters/telemetry/client.go`), the guard's refusal on its injected stderr (`internal/adapters/instance/guard.go`), the log writer itself (`internal/adapters/output/`) and buffer formatting in the TUI (`internal/adapters/tui/`)
- metrics go through the bus-driven collector (`internal/adapters/telemetry`) only. No `sentry.NewMeter` elsewhere
- respect `FUKU_TELEMETRY_DISABLED`

## Concurrency & Resource Safety

- every goroutine has a clear exit: context cancellation or a channel signal
- shared state is synchronized with a mutex or a channel
- prefer the bounded worker pool (`internal/platform/worker`) over ad-hoc goroutines
- pass `context.Context` through call chains. Never store it in a struct (exceptions: UI components under `internal/adapters/tui/`, the run-wide context of `lifecycle.Coordinator` and the work context of `services.Runtime`)
- close what you open: files, sockets, channels, connections
- a producer's `Stop` releases what its `Start` acquired. The coordinator is the one Fx hook
- no sends on closed channels. Watch for unbuffered-channel deadlocks
- trap SIGINT and SIGTERM. SIGKILL is a last resort we send to children, never something we handle

## Security

- validate external input: CLI arguments, config values, environment variables
- timeouts on external operations
- retries with backoff where needed (`retry.attempts`, `retry.backoff`)
- avoid command injection and path traversal

## Important Workflow Notes

- always run `verify` before committing
- never put AI attribution in a commit message: no `Co-Authored-By` or `Claude-Session` trailer, no "Generated with" line, no robot emoji.
  Naming a path is not attribution, so `docs(claude):` for a change under `.claude/` is fine
- never include a "Test plan" section in a PR description
- comments describe the current code, never its history
- after important functionality is added, update `README.md`, `ARCHITECTURE.md` or the package `README.md`
- when merging master into a branch, pull both first
- do not leave commented-out code
- use `gh` for GitHub work
- `//nolint` directives go on the line above, name the exact linter and carry a reason: `//nolint:errcheck // Close errors are non-actionable in cleanup`
- before a significant refactor, make sure all tests pass. Consider a new branch
- when refactoring or fixing tests: no redesign, minimal changes, existing patterns. If stuck, report and ask

## Handling Files with Formatting Issues

When a file has mixed tabs and spaces or other formatting problems:

- do not read it and wait for a manual fix
- fix it with Edit. For pervasive issues, rewrite with Write
- run `make fmt` after edits
- include the formatting fix in the same commit as the code change

## Formatting Guidelines

- always use `make fmt` (wraps `gofmt`, then `golangci-lint fmt`, whose `gci` formatter groups imports stdlib, third-party, `fuku`)
- respect `.editorconfig`: tabs in Go files, `tab_width = 2` for display, UTF-8, LF, final newline, no trailing whitespace
- when using Edit, keep the existing indentation

## Commonly Used Libraries

- dependency injection: `go.uber.org/fx`
- CLI framework: `github.com/spf13/cobra`
- configuration: `github.com/spf13/viper`
- environment files: `github.com/joho/godotenv`
- logging: `log/slog` over a `github.com/rs/zerolog` handler (`internal/platform/logging`)
- error tracking: `github.com/getsentry/sentry-go`
- testing: `github.com/stretchr/testify`
- mock generation: `go.uber.org/mock`
- TUI framework: `charm.land/bubbletea/v2`
- TUI components: `charm.land/bubbles/v2`
- TUI styling: `charm.land/lipgloss/v2`
- process monitoring: `github.com/shirou/gopsutil/v4`
- semver comparison: `golang.org/x/mod/semver`
