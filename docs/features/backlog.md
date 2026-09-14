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

## Medium

- [ ] **BL-001 – Define and enforce the package-level variable policy**
  - Why: `gochecknoglobals` is not enabled, so implementation and review cannot reliably catch unwanted package-level variables
  - Added: 20260902
  - Source: [`internal/app/instance/instance_test.go`](../../internal/app/instance/instance_test.go)

## Low

- [ ] **BL-005 – Add a test toolchain to the JetBrains plugin**
  - Why: the plugin has no test sources or test dependencies, so its API models and socket connection logic are verified
    only by ktlint and `buildPlugin`; a regression in either passes CI
  - Added: 20260913
  - Source: [Project-scoped sockets](20260913-project-scoped-sockets/plan.md)

- [ ] **BL-007 – Point the doctor runtime checks at an injectable socket directory**
  - Why: `checkStaleSockets` globs and dials the host's `/tmp/fuku-*.sock`, so `Test_checkStaleSockets` reads whatever
    sockets the machine holds and can only assert OK-or-Warn; a socket directory the test can set makes the check
    deterministic
  - Added: 20260914
  - Source: [`internal/app/doctor/runtime_test.go`](../../internal/app/doctor/runtime_test.go)

## Done

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
