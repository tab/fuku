# Code review

Mode: standard
Gate: code
Status: passed
Round: 15
Target: working tree against 0afcd3e (uncommitted feature diff on fix/project-scoped-sockets)
Reviewer: codex
Model: gpt-5.6-luna
Effort: medium

## Resolved

- CODE-1 – resolved: queued connections are invalidated when the fingerprint changes or becomes blank.
- CODE-2 – resolved: `Test_PublishSignal` expectations match the signal and no-signal cases.
- CODE-3 – resolved: focused struct literal reorderings align with declarations and repository rules.
- CODE-4 – resolved: architecture and feature documents now describe the lightweight doctor container and identity injection.
- CODE-5 – resolved: in-flight stale connections are revalidated after connect, discarded when obsolete, and allow the queued current-fingerprint connection to proceed; null fingerprints and close failures are handled without installing a channel.
- CODE-6 – resolved: backlog disposition accepted as BL-006; the cited guard/API and relay startup windows already exist on master, and an atomic kernel lock is correctly deferred for separate work.
- CODE-7 – resolved: disputed disposition accepted; the cited cross-user socket case cannot signal another user’s processes or remove its sticky-directory socket, and the relay continues as on master.
- CODE-8 – resolved: settings changes clear the cached live identity and publish disconnected state before polling the replacement client; recorded plugin checks pass.
- CODE-9 – resolved: service tests now clear `SENTRY_DSN` and disable updater checks before running, while the empty DSN causes `TelemetryEnabled()` to remain false and Sentry initialization to return immediately.

## Findings

No findings.

## Assessment

- The three CLI objects earn their place for the explicitly requested uniform `New<X>CLI(cmd).Run()` dispatch shape. Their split matches the dependency stages: no container, identity-only FX, and full application FX. Without that uniform dispatch requirement, I would use private functions for the two stateless runners.
- `internal/bootstrap` is the correct composition-root package. The `bootstrap → app → cli` direction is coherent, and `cli` remains the user-facing command/parser, direct-output, doctor, config-generation, and TUI boundary.
- `ChangeToConfigDir` is duplicated only at the two command boundaries that need it. Extracting a callback abstraction would add indirection without reducing meaningful complexity. The broad `forbidigo` exclusion is intentional for direct bootstrap CLI output, and the godoc comments satisfy the repository convention.
- Master behavior is preserved for parsing, standalone `--config` rejection, config-directory changes, config loading, service validation, Sentry DSN fallback, signal exit codes, stderr errors, and silent `ErrInstanceAlreadyRunning` handling.
- The bootstrap tests cover the planned dispatch, exit-code, container, logging, and placement cases. Exact stderr text and the DSN fallback are not directly asserted, but the implementation is a surgical move of master behavior and the recorded checks passed.

## Checked

- Handoff, feature contract, plan, root `CLAUDE.md`, and the e2e/docs/JetBrains area guides
- Full working-tree status and `git diff HEAD` across the 38 modified files, plus untracked `run.go`, `run_test.go`, and feature documents
- `cmd/main.go` against `git show HEAD:cmd/main.go`; bootstrap behavior is preserved apart from the intended doctor/FX changes
- FX module and lifecycle ordering, guard, relay binding/cleanup, preflight, stale sockets, doctor, logs, CLI, tests, e2e coverage, and old socket-reference scans
- Recorded checks: formatting, vet, lint, unit tests, build, e2e twice, race, plugin lint/build, and docs build
- Round-11 diff for `PluginService.kt` and `docs/features/backlog.md`
- Current plugin polling and log-stream disconnect behavior, including settings-change publication
- Master guard, API registration, relay startup sequence, preflight signaling, and unchanged socket/server behavior relevant to CODE-6 and CODE-7
- Round-12 bootstrap move: command dispatch, standalone/doctor/service paths, exit codes, stderr behavior, config-directory changes, identity resolution, Sentry DSN default, import direction, struct and constructor ordering, test placement/conventions, and current documentation references
- Round-13 full inspection of `internal/bootstrap/*.go`, bootstrap tests, retained `internal/app/cli` APIs and parser tests, master CLI implementation, focused documentation, package placement, and repository wiring rules
- Round-14 changes in `ARCHITECTURE.md`, the feature plan, `internal/bootstrap/standalone_test.go`, and `internal/bootstrap/service_test.go`
- Read-only directory permissions, cleanup ordering, test table structure, command-ignore assertion, service-container lifecycle, relay socket cleanup, logs no-follow behavior, and inherited telemetry environment handling
- Round-15 focused follow-up: `internal/bootstrap/service_test.go`, `internal/config/loader.go` `initConfig`, `internal/config/sentry/sentry.go`, and `ServiceCLI.Run` telemetry and DSN behavior

## Verdict

PASS