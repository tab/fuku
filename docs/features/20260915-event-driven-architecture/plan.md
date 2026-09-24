# Event-driven application architecture plan

Feature: [feature.md](feature.md)

Status: implemented

Phase: code review

Current step: the code review gate

## Approach

- One branch, `feature/event-driven-architecture`. The new tree replaced the old code in place, with no aliases
- The order followed the dependencies: `model` and `contracts`, `platform`, the adapters and cores, then the frontends
- The compositions came last, once every participant existed
- No request/reply on the bus and no event state machine. Retry is a fixed count with a constant backoff
- A test mocks the layer below its subject and nothing beneath it

## Steps

- [x] Extracted `internal/model` and `internal/contracts`. Deleted `app/errors`
- [x] Moved the bus, the worker pool and the build info into `internal/platform`. Added `adapters/eventlog`
- [x] Moved logging into `platform/logging`. Packages log through `slog`-shaped `Logger` interfaces
- [x] Built the bus delivery API: named subscriptions, `Required`, `Types` and critical reservation
- [x] Moved config into `adapters/config`. Added `model.Project` and `app/profiles`
- [x] Projected each package's `Options` from `model.Project`. Only `bootstrap` imports the config adapter
- [x] Moved `instance`, `resources`, `readiness`, `watch` and `telemetry` into `adapters`
- [x] Split `environment`, `updater` and `doctor` into cores and the `envfiles`, `github` and `diagnostics` adapters
- [x] Extracted `adapters/process`
- [x] Extracted `app/services` with `Runtime`, `Guard` and `Control`
- [x] Built the registry snapshot
- [x] Split the log pipeline into `app/logs`, `logsocket`, `output`, `terminal` and the CLI and TUI log views
- [x] Moved REST and the CLI into `adapters`. REST actions go through `Control`
- [x] Moved the TUI into `adapters/tui`. Added `Bridge` and `Program`
- [x] Added the lifecycle participants, `Coordinator` and `Arbiter`
- [x] Split the compositions into `bootstrap/modules`
- [x] Put the TUI on the registry snapshot
- [x] Rewrote the TUI behavior tests
- [x] Enforced the rings with `depguard`
- [x] Wrote `ARCHITECTURE.md` and the package READMEs
- [x] Gave `model.Service` a UUID at config loading. Profile resolution keeps its ID
- [x] Made one read model: one `*model.Snapshot` under one lock, read in place
- [x] Tidied `internal/model` to one type per object
- [x] Fixed the final review's bugs, among them conflicting root flags, the stream copy and the stop timeout
- [x] Deleted the dead code and the test-only exports the review found. `Store.Update` became `update`
- [x] Fixed the rule violations. The coordinator became the one Fx hook. `make lint` reached `0 issues.`
- [x] Rewrote the tests that asserted too little
- [x] Fixed the doc drift. Added the `envfiles`, `diagnostics` and `profiles` READMEs
- [x] Cut this record to `feature.md` and `plan.md` and checked each line against the code

## Verification

- `make check`: exit 0. Lint prints `0 issues.`. 33 packages pass. `make fmt` changed no Go file
- `make test:race`: exit 0 with `GOFLAGS=-count=1`, so no result came from the test cache
- Total coverage after the race run is 99.4%: `go tool cover -func=coverage.out | tail -1`
- `make test:e2e`: 59 tests pass on a fresh `make build`
- `make docs`: `All links resolve.`
- `golangci-lint run --enable-only depguard ./...`: `0 issues.`

## Gates

- [x] Feature approval
- [x] Plan review – PASS, round 3
- [x] Plan approval
- [ ] Code review – open. Next: a `feature-review` run at the code gate. It writes a fresh `code-review.md`
- [ ] PR review – not run. It needs the work committed and a PR open
- Nothing is committed. `HEAD` is `e45386f`, the same commit as `origin/master`

## Follow-ups

The open review items are tracked in the [backlog](../backlog.md).
