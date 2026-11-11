# Code review

Mode: risk review
Gate: code
Status: pass with follow-ups
Target: `bcd0923..ea68a8f` on `feature/mcp` (round 1); staged fix on `ea68a8f` (round 2)
General status: pass with follow-ups
General round: 2
General reviewer: claude
General model: opus
General effort: high
Risk perspective: agent control boundaries
Risk status: pass
Risk round: 2
Risk reviewer: codex
Risk model: gpt-6-astra
Risk effort: high (read-only sandbox)

## General

### CODE-G1 – A waited start of a watched service answers `watching: false`

Source: general

Severity: medium

Location: `internal/adapters/rest/handler.go:150` (`handleAction` → `settle` → `handleGetService`), `internal/app/registry/query.go:28` (`WaitSettled`). It reaches the agent through `internal/adapters/mcp/control.go:153` (`apply`) and `control.go:37` (`handleRun` → `serviceStates`).

Finding: a waited `start` of a watched service settles on `ServiceReady` and answers right away. The watcher publishes `WatchStarted` later, asynchronously, so the answer says `watching: false`. Reproduced on a scratch build of ea68a8f: `POST .../start {"wait":10}` returned `"status":"running","watching":false`; a `GET` one second later returned `"watching":true` with the same PID. A restart returned `watching: true`. `run` reads its service states right after `fuku run -d` returns, so it can hit the same race.

Impact: the `service` and `run` results can tell the agent that a watched service is not watching. The agent then restarts it by hand after a code edit instead of waiting for the new PID. The cost is an extra restart or a misleading report, not data loss.

Suggested fix: when the service has a `Watch` config and the settled status is `running`, also wait in `WaitSettled` until `Watching` is true. Or add a `ponytail:` comment naming the race, plus a line in the `service`/`run` description that `watching` can lag one event after a start.

Rollout: can wait

Reply: Fixed (option A, chosen by the human). `WaitSettled` treats a running service with a `watch` block as settled only once `Watching` is true, so a waited `start` or `restart` answers `watching: true`. If the watcher never starts, the wait ends at its limit and the answer carries `inProgress: true`. Two `Test_Store_WaitSettled` rows cover both cases; the registry README states the rule. The `run` result still reads the states right after `fuku run -d` returns and can show `watching: false` for a moment.

Recheck (round 2): Resolved. `model.Service.SettledSince` holds a running service with `Watch != nil` until `Watching` is true, and `WaitSettled` calls it. `WatchStarted` wakes the waiter; `applyServiceStopped` clears `Watching`, so a stale `true` cannot settle a fresh start early. Live on the staged tree: 5 of 5 waited stop/start cycles and a waited restart answered `watching: true` with a new PID. Four mutants on the rule were killed. The leftover `run` lag is stated in the reply and does not keep this open.

### CODE-G2 – Plugin 0.2.0 needs a fuku binary that is not released yet

Source: general

Severity: medium

Location: `plugins/agents/.mcp.json`, `plugins/agents/skills/fuku/SKILL.md` ("If the fuku tools are not available, tell the user to install fuku…"), `docs/src/pages/plugins/agents.astro`. The marketplace in `.claude-plugin/marketplace.json` serves `./plugins/agents` from the repository.

Finding: plugin 0.2.0 removes `scripts/api.sh` and works only through `fuku mcp`, which no released binary has (latest tag v0.22.0). Nothing states a minimum fuku version. With v0.22.0 on PATH the MCP server fails to start, and the skill tells the user to install fuku, which they already have.

Impact: once this merges to master, anyone who updates the plugin before the new binary reaches them loses all fuku control from the agent.

Suggested fix: release the binary with `fuku mcp` (tag, release, Homebrew) before or with the merge that publishes plugin 0.2.0. Change the skill and docs line to "install or upgrade fuku to vX.Y.Z or later".

Rollout: fix today (the release order); the wording can follow

Reply: Accepted risk by the human: no user drives fuku through an agent today, so the release order needs no gate.

Recheck (round 2): Resolved as accepted risk. The staged diff does not touch the plugin, the skill or the docs.

### CODE-G3 – `stop` reports a plain failure when `fuku stop` outlives the tool timeout

Source: general

Severity: minor

Location: `internal/adapters/mcp/control.go:54-60` (`handleStop`, `commandTimeout` 50s), against `internal/bootstrap/modules/run.go:55` (`stopTimeout` = 15s + 5s × services).

Finding: with 8 or more services, the wait `fuku stop` allows exceeds 50s. On a slow exit the tool kills `fuku stop` at 50s, its SIGKILL fallback never runs, and the tool reports "signal: killed" although the stop is still going on. `handleRun` handles the same case as `inProgress`.

Impact: only with a hung or very slow shutdown. The agent sees a failed stop and may retry or report wrongly.

Suggested fix: handle `DeadlineExceeded` in `handleStop` the way `handleRun` does, or cap the stop wait below `commandTimeout` for MCP calls.

Rollout: can wait

Reply: Deferred. The human runs ~60 services that stop in ~5s, so the 50s limit only matters for a hung stop; a long start already returns `inProgress`.

Recheck (round 2): Resolved as deferred. In `handleStop` only the argument order changed; the timeout handling is the same.

### CODE-G4 – `run` and `service` are marked destructive, but the approved tool table says write

Source: general

Severity: minor

Location: `internal/adapters/mcp/tools.go:262,264`, `internal/adapters/mcp/server_test.go:37,39`, against `docs/features/20261108-mcp-server/feature.md:55,57`.

Finding: the staged diff changes `run` and `service` to `DestructiveHint: true`. The feature contract still says `write` for both, with `destructive` only on `stop()`. The code no longer matches the approved contract.

Impact: no runtime defect. An MCP client may ask for confirmation on every `run` and `service` call. The contract and the code disagree until resolved.

Suggested fix: ask the human. Either approve the change and update the two table rows in `feature.md` to `destructive`, or go back to `DestructiveHint: false` for both tools.

Rollout: fix today (one table edit or a revert, after the human decides)

Reply: The human approved the destructive hint for CODE-R2; the `run` and `service` rows in `feature.md` now say `destructive`.

### Notes

- The guard's `release` now calls `close(entry.idle)`, so a second release on one token would panic. Every caller (`control.go:96`, `bus.go:103/113`, `run.go:296`) pairs one release with one `take()`. Any future release path must keep this pairing
- `parseWait` (`DisallowUnknownFields`) returns 400 for any non-empty unknown body. The JetBrains plugin and the docs send no body, so no caller breaks
- The two `ponytail:` ceilings (early wait end in `handler.go`, dark-only theme in `modules/mcp.go`) are recorded on purpose

### Checked

- feature.md, plan.md, the diff `bcd0923..ea68a8f` (167 files): `adapters/mcp` in full, `rest/client.go`, `tui/mcp_console.go`, `bootstrap/modules/mcp.go`; the REST, services, registry, logsocket, config, instance, doctor, cli, bootstrap and detach hunks; the plugin manifests, `.mcp.json`, the `fuku` skill, the marketplace and `e2e/mcp_test.go`
- Instructions: 1,135 characters; the first 512 are the rules block and stand alone
- `go test -race -count=1` on mcp, rest, services, registry, config, logsocket, bootstrap/... and tui: ok
- `python3 -m unittest discover -s plugins/agents/tests`: 126 ok (7 skipped without `FUKU_BIN`, 0 with it)
- `FUKU_BIN=<scratch build> go test -run 'MCP|API|Detached' ./e2e/...`: ok
- A live reproduction of CODE-G1 on a scratch project
- Six mutants through `go test -overlay` (wait query, wait bound, JSON-escaped redaction, watch flag on stop, `withConfig`, `handleRun` deadline): all killed
- Not rerun: `make docs`, `npm run build`, depguard, the live sweep. `plugins/agents/e2e` not reviewed line by line. BL-037/BL-038 not re-raised

### Round 2 checked

- The staged diff on `ea68a8f` (16 files) and the connected store, watcher and CLI code; `code-review.md` not read
- `go test -race -count=1` on model, registry, cli, detach, mcp and rest: ok
- `FUKU_BIN=<scratch build of the staged tree> go test -count=1 -run 'MCP|API|Detached|Doctor|Stop' ./e2e/...`: ok
- Live: 5 waited stop/start cycles and 1 waited restart of a watched service, each answering `watching: true`
- Ten mutants (the settled rule, the old condition, `--` dropped in each of the four builders, the `run` annotation): all killed
- Not rerun: make lint, make docs, the full e2e suite, the live host sweep

### Verdict

PASS WITH FOLLOW-UPS (round 2). No blocker or major is open; CODE-G4 changes the approved contract and needs the human's decision.

## Agent control boundaries

### CODE-R1 – A flag-like profile reaches the fuku CLI as a flag

Source: agent control boundaries

Severity: major

Location: `internal/adapters/mcp/control.go:23`, `internal/adapters/mcp/control.go:52`, `internal/adapters/mcp/tools.go:352`

Finding: profile values are passed as CLI arguments without an option terminator. `run({"profile":"--config=/other/fuku.yaml"})` passes schema validation and executes `fuku run --config=/other/fuku.yaml --detached`. Cobra interprets this as a config flag and runs the other project's default profile.

Impact: a profile argument can start processes and perform preflight cleanup in another project. MCP then reads status and selects redaction tokens using the original target. Doctor and stop build arguments with the same weakness.

Suggested fix: put flags before `--` and the profile after it. Keep this separation in the detached launcher too. Add regression coverage that passes flag-like profiles through the actual CLI parser.

Rollout: fix today

Reply: Fixed. `run`, `stop` and `doctor` put every flag first and the profile after `--`; the detached launcher does the same for its child. A `Test_Parse` row proves `run --detached -- --config=/other/fuku.yaml` parses as a profile with no config file. The mcp and launcher expectations follow the new order.

Recheck (round 2): Resolved. All four argument builders place flags before `--` and the profile after it. `Binary.Execute` keeps separate arguments through `exec.CommandContext`. The parser regression asserts the literal profile, detached mode and an empty `ConfigFile`; the MCP and launcher tests check the order.

### CODE-R2 – `run` and `service` advertise `destructiveHint: false`

Source: agent control boundaries

Severity: medium

Location: `internal/adapters/mcp/tools.go:257`, registrations at lines 264 and 266

Finding: the SDK defines `destructiveHint: false` as only additive updates. `service` stops or restarts processes, and `run` invokes preflight cleanup that kills existing processes in service directories.

Impact: hosts receive incorrect safety metadata when presenting or evaluating these calls.

Suggested fix: set the destructive hint to true for both tools and update the annotation assertions.

Rollout: can wait

Reply: Fixed. `run` and `service` use the destructive annotation; the `additive` value is gone and the annotation test expects true.

Recheck (round 2): Resolved. Both registrations use `DestructiveHint: true`. `Test_NewServer` checks the annotations through the SDK client's `ListTools` response.

### Checked

- `bcd0923..ea68a8f`, the feature contract, plan, repository rules and area guides
- MCP schemas, handlers, subprocess arguments, config resolution, token redaction, console output, REST authentication and discovery, process cleanup
- MCP, config, REST and e2e tests, plus the installed Cobra/pflag and MCP SDK code
- Both findings verified against code; no runtime reproduction
- Took the lead's `make check` pass; other checks keep their pre-rewrite limits
- BL-037 and BL-038 not re-raised

### Round 2 checked

- The staged diff on `ea68a8f`, the contract, plan and follow-up rules; `code-review.md` not read
- The argument builders, CLI parsing, config-path forwarding, subprocess execution, redaction and annotation assertions
- `Service.SettledSince`, the registry wait and its REST and MCP callers
- `git diff --cached --check` passed. No tests or live sweep rerun

No new verified findings within this perspective.

### Verdict

PASS (round 2)

## Combined verdict

PASS WITH FOLLOW-UPS

No blocker or major is open. CODE-G1, CODE-R1 and CODE-R2 are resolved. CODE-G2 is an accepted risk, CODE-G3 is deferred, and CODE-G4 is resolved by the human's approval of the destructive hint.
