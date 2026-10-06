# Backlog

## High

- [ ] **BL-002 – Clear the current `goconst` findings**
  - Why: `make lint` reports 50 repeated string literals, mostly in tests, so the full lint check cannot pass
  - Added: 20260902
  - Source: [Instance identity plan](20260902-instance-identity/plan.md)

- [ ] **BL-003 – Remove stale `exhaustive` suppression directives**
  - Why: `make lint` reports 6 `nolintlint` findings because the directives no longer suppress active findings
  - Added: 20260902
  - Source: [Instance identity plan](20260902-instance-identity/plan.md)

- [ ] **BL-025 – Stop another local user from posing as the project's fuku**
  - Why: the project socket path in `/tmp` is predictable. A hostile local user who binds it first can answer `fuku stop` with a fake PID
  - Fix: put the socket in a per-user `0700` directory, or check the peer's credentials on connect
  - Added: 20261005
  - Source: [Agent skill](20261004-agent-skill/feature.md)

## Medium

- [ ] **BL-001 – Define and enforce the package-level variable policy**
  - Why: `gochecknoglobals` is not enabled, so implementation and review cannot reliably catch unwanted package-level variables
  - Added: 20260902
  - Source: [`internal/adapters/instance/identity_test.go`](../../internal/adapters/instance/identity_test.go)

- [ ] **BL-008 – Add CLI controls for individual services**
  - Why: reuse core service controls without requiring TUI or duplicating start/stop/restart logic
  - Commands: proposed `fuku start <service-name>`, `fuku stop <service-name>` and `fuku restart <service-name>`
  - Boundary: deferred beyond the architecture refactor; target the owning runtime without starting another service runtime
  - Compatibility: resolve the conflict with existing `fuku stop [profile]` syntax before implementation
  - Added: 20260916
  - Source: [Application architecture](20260915-event-driven-architecture/feature.md)

- [ ] **BL-009 – Compare Fuku's bus with Watermill GoChannel**
  - Why: find whether Watermill can improve reliability or reduce maintenance without weakening required behavior
  - Options: keep our bus, switch behind Fuku interfaces or adopt useful Watermill ideas in our implementation
  - Compare: ordering, bounded memory, overload, slow consumers, publishing from handlers and shutdown under the same tests
  - Quality: check race and leak tests, maintenance, dependencies and the custom wrapper code each option would require
  - Boundary: in-memory and in-process only; no external broker or service, no migration approved and no change to the current refactor
  - Decision: keep our bus if the complete library-plus-wrapper solution is not a clear improvement
  - Added: 20260916
  - Source: [Application architecture](20260915-event-driven-architecture/feature.md) and [Watermill GoChannel](https://watermill.io/pubsubs/gochannel/)

- [ ] **BL-010 – Enforce the criticality table by construction**
  - Why: `contracts.MessageType.Critical()` reads a map and its test compares two hand-kept lists, so a new type added to neither passes and becomes droppable
  - Fix: write `Critical()` as a `switch` with no `default` over every constant, so the `exhaustive` linter fails on a missing one
  - Added: 20260926
  - Source: [Event-driven architecture code review](20260915-event-driven-architecture/code-review.md) (CODE-3)

- [ ] **BL-013 – Poll the process group, not the leader, in the preflight kill loop**
  - Why: after a group SIGTERM, `kill` polls the matched pid only; a leader that exits while a group child with another working directory ignores SIGTERM returns before the SIGKILL fallback
  - Boundary: present at `e45386f`; the scan already targets children that keep the service directory
  - Added: 20260926
  - Source: [Event-driven architecture code review](20260915-event-driven-architecture/code-review.md) (Codex pass)

- [ ] **BL-014 – Signal the process group when a service leader exits and leaves a child behind**
  - Why: a command such as `sleep 30 & exit 0` exits while its child keeps the streams; `Wait` returns `ErrWaitDelay` after `ShutdownTimeout`, the exit watcher untracks the service and shutdown never signals the group, so the child outlives `fuku`
  - Boundary: present at `e45386f`, where the child was orphaned at once; the branch delays it by `ShutdownTimeout`, and the next `fuku run` or `fuku stop` kills it through preflight
  - Decision: a group kill in `Factory.wait` also kills a script's backgrounded server while `fuku` still runs, so it needs a decision on that behaviour first
  - Added: 20260926
  - Source: [Event-driven architecture code review](20260915-event-driven-architecture/code-review.md) (Codex pass)

- [ ] **BL-015 – Reject a self-referencing merge key instead of overflowing the stack**
  - Why: `services: { api: &b { <<: *b, dir: api } }` makes `flattenMergeKeys` recurse forever through `tierOf`, so `fuku run` dies with `fatal error: stack overflow` instead of a config error; `replaceAliases` and `copyNode` likely share the cycle for `a: &x [*x]`
  - Boundary: present at `e45386f`; found while covering `merge.go`
  - Added: 20260926
  - Source: [Event-driven architecture code review](20260915-event-driven-architecture/code-review.md) (coverage pass)

- [ ] **BL-016 – Spare bystanders in the preflight cleanup**
  - Why: preflight picks the processes to kill by working directory, so a service with `dir: .` gets every process in the project root killed on `fuku run` and `fuku stop`
  - Boundary: present at `e45386f`; found by an e2e scenario that was not kept because it can only fail
  - Done: `fuku` and its ancestors are spared, so the launching shell or IDE survives
  - Left: any other process in the project root, such as an editor or the `tee` of `fuku run | tee log`; matching children by an environment marker instead of the directory would close it.
    A matched bystander that leads `fuku`'s own process group, as `tail -f x | fuku run` does, takes `fuku` down with the group signal; skipping `fuku`'s own group in the scan would cover that case
  - Added: 20260926
  - Source: [Event-driven architecture code review](20260915-event-driven-architecture/code-review.md) (coverage pass)

- [ ] **BL-019 – Print plain `fuku logs` and `fuku doctor --summary` output on a pipe**
  - Why: both emit colour escapes when stdout is not a terminal and ignore `NO_COLOR`; in an agent's context 56% of a 14.5 KB log read was escape codes. `fuku run -d` already prints plain lines
  - Added: 20261004
  - Source: [Agent skill end-to-end tests](20261004-agent-skill-e2e/feature.md), spike

- [ ] **BL-020 – Keep fuku's own debug lines off the output of `fuku logs` and `fuku stop`**
  - Why: at `logging.level: debug` the client's `[FX]` and `[BUS]` bootstrap lines go to stdout, about 156 lines per `fuku logs` call that `--tail` does not bound
  - Added: 20261004
  - Source: [Agent skill end-to-end tests](20261004-agent-skill-e2e/feature.md), spike

- [ ] **BL-021 – Add a compact doctor form or a single-check query**
  - Why: `fuku doctor --json` is 5.1 KB and an agent reads one key, `runtime.instance`; `--summary` is 1.2 KB but coloured and has no stable status word
  - Added: 20261004
  - Source: [Agent skill end-to-end tests](20261004-agent-skill-e2e/feature.md), spike

- [ ] **BL-022 – Do not advise removing the socket on a permission error**
  - Why: for a socket that answers `operation not permitted`, `runtime.instance` suggests `rm /tmp/fuku-….sock`; under a sandbox the fuku behind it is alive, and the agent skill forbids the removal
  - Added: 20261004
  - Source: [Agent skill end-to-end tests](20261004-agent-skill-e2e/feature.md), spike

- [ ] **BL-023 – Remove the project lock file when fuku stops**
  - Why: every run leaves `/tmp/fuku-<fingerprint>.lock` behind; a developer machine held 81 stale ones
  - Added: 20261004
  - Source: [Agent skill end-to-end tests](20261004-agent-skill-e2e/feature.md), spike

- [ ] **BL-024 – Put a build identifier in `fuku version`**
  - Why: a build from master and the 0.21.0 release both print `0.21.0`, so a caller cannot tell whether `fuku run -d` exists; a Codex run picked the release from a login-shell `PATH` and failed
  - Added: 20261004
  - Source: [Agent skill end-to-end tests](20261004-agent-skill-e2e/feature.md), spike

- [ ] **BL-017 – Make the API token optional behind `Host` and `Origin` checks**
  - Why: the token cannot stop a local process, which can read `fuku.yaml`, and it forces every API client to handle a secret; it is still the only defence against a browser page, because `adapters/rest` checks neither header
  - Fix: reject a request whose `Host` is not a loopback name and any request that carries an `Origin`, then let `server.listen` work without `server.auth.token`
  - Boundary: a config contract change; the JetBrains plugin and the agent skill both send the token today
  - Added: 20261004
  - Source: [Agent skill](20261004-agent-skill/feature.md)

- [ ] **BL-026 – Restore the phase when the stop of every service cannot be published**
  - Why: `services.Control.StopAll` closes admission before it publishes. When the critical publish fails with bus overload, the phase stays `stopping` with no command in flight. A later stop is acknowledged and does nothing
  - Fix: put the phase back when the publish fails, as `admit` releases its reservation
  - Added: 20261005
  - Source: [Agent skill](20261004-agent-skill/feature.md)

- [ ] **BL-030 – Say on the agents page which Codex sandbox setting reaches the fuku socket**
  - Why: a plain Codex workspace profile refused fuku's unix socket in `/tmp`. The e2e suite sets `sandbox_workspace_write.network_access=true`
  - Boundary: verify the setting against a real Codex before documenting it
  - Added: 20261005
  - Source: [Agent skill end-to-end tests](20261004-agent-skill-e2e/feature.md)

## Low

- [ ] **BL-005 – Add a test toolchain to the JetBrains plugin**
  - Why: the plugin has no test sources or test dependencies, so its API models and socket connection logic are verified
    only by ktlint and `buildPlugin`; a regression in either passes CI
  - Added: 20260913
  - Source: [Project-scoped sockets](20260913-project-scoped-sockets/plan.md)

- [ ] **BL-011 – Keep one list of log levels**
  - Why: config validation accepts four levels while `platform/logging` still lists seven and parses `trace`, `fatal` and `panic`, reachable only from tests
  - Added: 20260926
  - Source: [Event-driven architecture code review](20260915-event-driven-architecture/code-review.md) (CODE-5)

- [ ] **BL-012 – Remove the test-only `options` field from `tui.Program`**
  - Why: only tests write it; try injecting stdin and stdout as named values with `tea.WithInput` and `tea.WithOutput` and delete the field if the program then runs headless under `go test`
  - Added: 20260926
  - Source: [Event-driven architecture code review](20260915-event-driven-architecture/code-review.md) (CODE-7)

- [ ] **BL-027 – Show the shutdown loader when a stop arrives over the socket**
  - Why: a stop requested over the socket does not show the TUI's "shutting down all services" loader. Only `q` and a signal start it
  - Added: 20261005
  - Source: [Agent skill](20261004-agent-skill/feature.md)

- [ ] **BL-028 – Tell the user to restart an older fuku that a sandbox cannot stop**
  - Why: an instance started by an older fuku has no stop request. From a sandbox, `fuku stop` falls back to SIGTERM, which is denied. The error could say to restart that instance once
  - Added: 20261005
  - Source: [Agent skill](20261004-agent-skill/feature.md)

- [ ] **BL-029 – Clean up leftover processes when `fuku stop <profile>` runs in a sandbox**
  - Why: inside a sandbox the leftover-process cleanup is denied, and `fuku stop` only warns
  - Added: 20261005
  - Source: [Agent skill end-to-end tests](20261004-agent-skill-e2e/feature.md)

## Done

- [x] **BL-018 – Do not clean up when `fuku stop` cannot reach the running fuku**
  - Why: a dial error other than `ECONNREFUSED`, such as `EPERM` in an agent sandbox, prints `Cannot reach the running fuku` and returns nil; `fuku stop` then kills the processes in the service directories under a live fuku, removes its socket and exits 0
  - Fix: return the error from `detach.Stopper.Stop` so the cleanup and the socket removal do not run and the exit code is 1
  - Added: 20261004
  - Source: [Agent skill](20261004-agent-skill/feature.md), step 2 review

- [x] **BL-004 – Refuse a second foreground `fuku run` of the same project when the API is disabled**
  - Why: the second instance only warns that the socket is in use and continues, so its preflight cleanup can kill the
    first instance's services; once the socket is scoped to the project it can serve as the lock the API guard cannot
    provide without `server.listen`
  - Added: 20260913
  - Source: [Project-scoped sockets](20260913-project-scoped-sockets/feature.md)

## Won't implement

- [ ] **BL-006 – Make the single-instance guard atomic**
  - Why: the guard probes the project socket and the API before the relay server and the API bind, so two `fuku run`
    of one project launched within the same few milliseconds both pass; a kernel lock (`flock` on a per-project lock
    file) would close that window
  - Status: Won't fix – the window is a few milliseconds and the same one the API guard has had since it shipped; a
    developer does not launch two runs of one project at once, so the lock would add a mechanism for a case nobody
    meets
  - Added: 20260913
  - Source: [Project-scoped sockets code review](20260913-project-scoped-sockets/code-review.md)

- [x] **BL-007 – Point the doctor runtime checks at an injectable socket directory**
  - Why: `checkStaleSockets` globs and dials the host's `/tmp/fuku-*.sock`, so `Test_checkStaleSockets` reads whatever
    sockets the machine holds and can only assert OK-or-Warn; a socket directory the test can set makes the check
    deterministic
  - Status: Won't do – the check reads `Runtime.Sockets()`, an injected observer. `Test_Runner_checkStaleSockets`
    drives the mock and asserts the exact severity. `diagnostics.Test_scanSockets` binds its own socket in a directory
    of its own. The one-line `Sockets()` wrapper over `instance.SocketDir` stays untested. A settable directory would
    add a knob nothing reads
  - Added: 20260914
  - Source: [`internal/app/doctor/runtime_test.go`](../../internal/app/doctor/runtime_test.go)
