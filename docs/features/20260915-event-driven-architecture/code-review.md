# Code review

Mode: standard
Gate: code
Status: passed
Round: 2
Target: e45386f..ed532d4
Reviewer: Claude Code, Fable 5.1 orchestrating eight opus area reviewers, plus a Codex adversarial pass
Model: claude-fable-5-1
Effort: not reported

## Findings

None open.

## Resolved

- CODE-1 – resolved: AC4 no longer says unnamed subscriptions fail (`feature.md:67`). The Contracts line says a `Name` serves diagnostics and the bus does not refuse an empty one (`feature.md:88`). This matches `bus.go:45-58`, the name's two uses at `bus.go:81,112` and the bus README
- CODE-2 – resolved: `runParams` holds no checker (`run.go:99-109`). `newViewParticipants` appends it after the API server (`run.go:126-132`). An Fx constructor probe runs no `updater` or `github` constructor headless, and runs `updater.NewChecker` and `github.NewClient` under the view. Scope lists CORS, the four levels and the override stat, each true (`rest/server_test.go:72`, `config/validate.go:112-118`, `config/loader.go:61-64`). The Decisions line is true: `control.go:69` is the only publisher of a service command. Every prose site now says the check runs under the TUI only. Its smaller changes carried on as CODE-8
- CODE-3 – backlog: BL-010
- CODE-4 – resolved: `worker.Pool.Acquire` errors only on `ctx.Done()` (`pool.go:18-25`). The guard is one `if err != nil` that publishes stopped (`run.go:298-304`), the same path `ed532d4` took on a cancelled context. No reference to `ErrFailedToAcquireWorker` remains. The deleted row was the only one that asserted the branch. The row at `run_test.go:404` still covers the guard. Services coverage is 100%
- CODE-5 – backlog: BL-011
- CODE-6 – resolved: `lifecycle.astro:95` names the exclusive lock and the older-instance caveat, true against `guard.go:47` and `README.md:21-22`. `api.astro:329` says the instance shuts down and the run exits 1, true against `bus.go:93` and `Arbiter.decide` (`shutdown.go:60-76`)
- CODE-7 – backlog: BL-012
- CODE-8 – resolved: "Behavior changes" lists the five as bullets (`feature.md:32-36`). Each holds: `tui/animations.go:6-10` with `row.go:252`; `telemetry/metrics.go:198` tags `data.Route` where the base used `:id`; `readiness/http.go:14,25`; `readiness/log.go:36` with `process.MaxLineSize`; `services/run.go:298-304`

## Checked

- Round 2 is a focused follow-up. It covers the staged fixes (`git diff --cached`, 14 files) on `ed532d4`, and `docs/features/backlog.md` for the BL ids
- This recheck ran on claude-opus-5-5, not the header's claude-fable-5-1
- Constructor probe: `fx.New` over `Run(...)` with an `fxevent` logger that records `Run` events, loaded through `go test -overlay` so no file entered the tree
- Producer order: `Test_runParams_participants` expects Announcer, Socket, Watcher, Runtime, Sampler, then the server and the checker under the view. Headless has no checker. Both match `run.go:112-132`
- Idle providers: the headless graph still provides the checker and the GitHub client from the `runtime` block and never builds them. Every composition has idle providers (stop provides `services.NewRuntime`, logs provides `logsocket.NewServer`). AC1 holds as before
- The CLAUDE.md example `append(participants.Producers, checker)` is `run.go:129` verbatim. It keeps the updater thread and shows the checker appended after the producers before it. The `runParams` line shows more of the order but drops the checker. Either is faithful. Not a finding
- The new `newViewParticipants` godoc says the store and bridge "subscribe first". They are the last consumers, but they subscribe before any producer starts. True on that reading. Not a finding
- Update-check prose: a grep for claims that the check runs without the TUI finds only the corrected sites (`README.md:245`, `ARCHITECTURE.md:202`, the updater README, `modules/README.md:43`, `privacy.astro:78`)
- CODE-8 pass: the staged `feature.md` diff against `ed532d4` adds only the five bullets beyond the earlier hunks (Scope, AC4, the Contracts line, the Decisions line). Each bullet is short and holds one idea
- `go test -race -count=5 -timeout 300s` over `internal/bootstrap/...`, `internal/app/services` and `internal/contracts`: pass
- `golangci-lint run` on the same packages and the depguard audit: `0 issues.`. The comments script and the deadcode audit print nothing. `make docs`: `All links resolve.`. The docs site builds 20 pages
- Not rerun: the full `make check`, `make test:race` and `make test:e2e`

## Verdict

PASS

## Backlog candidates

Not findings of this change. One backlog item each, for triage after the verdict:

- the preflight kill test's signal race (`exec sleep 60` fixes it)
- the tracer `Subscribe` test row that makes telemetry coverage flip
- `rest/middleware_test.go` closures that assert on the parent `t` inside subtests
- the `.env`-set `GO_ENV` second read on doctor's error path
- an error kind on the snapshot instead of substring matching in `renderError`
- a null `tier:` splitting the topology from the model
- Sentry stamps `sentry.server.address` with the hostname on every metric and `stripPII` does not cover it
- the telemetry `Subscribe` tests compare `Types` against the same variable the code uses

## Follow-up passes

After the verdict. Neither reopens the gate.

- Codex `gpt-6-sol`, xhigh, on `e45386f..df7a736`: four findings. The phase overwrite after `StopAll` is fixed in `74f3e2c`.
  The orphaned descendant is BL-014, present at the base. The discarded `Terminate` error needs an EPERM child and is already logged. The hidden socket cleanup warning is harmless, because the next `start` removes a dead stale socket
- Coverage pass on the same head: unit coverage 99.3% to 99.7%, e2e 59 to 88 tests.
  It found two pre-existing bugs, BL-015 (a self-referencing merge key overflows the stack) and BL-016 (the preflight kills bystanders in the project root).
  The worst case of BL-016 is fixed: the preflight spares `fuku` and its ancestors, so the launching shell or IDE survives
- Codex `gpt-6-sol` again, on the head with the fixes: one finding, a child that starts between `StopAll` and the cancel.
  Dropped: the child is tracked under the launch lock and `shutdown` stops every tracked child, the same as a SIGINT during startup
- Codex `gpt-6-sol`, last pass, on `579c357`: three findings on the preflight, none new to the branch.
  A matched bystander that leads `fuku`'s process group signals `fuku` too: pre-existing, recorded under BL-016.
  A partial ancestor walk keeps the kill pass on: dropped, `fuku`'s own PID is always in the set and skipping the pass would leave the orphans alive.
  A service that runs `fuku stop` is spared as an ancestor: dropped, killing it killed the stop command too, so sparing is never worse
