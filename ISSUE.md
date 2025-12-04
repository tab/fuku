# Issue: Keep fuku running when a service fails to start

## Context
- Current startup flow (`internal/app/runner/runner.go`) returns an error on the first service failure, publishes `EventServiceFailed`, shuts everything down, and the CLI/UI exits. That prevents interacting with the failed service.
- Log-based readiness (`internal/app/runner/readiness.go`) waits for the full timeout even if the process has already died because it only watches the log pattern or timer, not the process exit. This makes the app appear stuck during startup.
- The bookstore example uses log readiness for every service in `fuku.yaml`, so a failing service blocks startup and triggers the exit path.

## Reproduction (after adding a failing sample service)
1. Add a `failed-service` under `examples/bookstore` with a `make run` that prints once and exits 1.
2. Add it to `fuku.yaml` with a log readiness pattern.
3. Run `fuku --run=default` (or a profile that includes the failing service).
4. Actual: startup waits through readiness timeouts and then exits; UI cannot restart the failing service.
5. Expected: other services start, the failing service is marked failed as soon as its process exits, and the app stays up so the user can stop/start/restart it.

---

## Research Findings

### Current Architecture

**Startup Flow** (`internal/app/runner/runner.go`):
1. `Run()` → publishes `EventPhaseChanged(Startup)` → resolves profile → calls `runStartupPhase()`
2. `runStartupPhase()` → launches `startAllTiers()` in goroutine, waits for completion or signals
3. `startAllTiers()` → iterates tiers sequentially, calls `startTier()` for each
4. `startTier()` → starts all services in tier concurrently, returns first error (line 396-398)
5. On any error: `startAllTiers()` returns error → `runStartupPhase()` calls `shutdown()` → `Run()` returns error → app exits

**Readiness Checking** (`internal/app/runner/readiness.go`):
- `CheckHTTP()`: polls HTTP endpoint until success or timeout; does NOT watch process exit
- `CheckLog()`: scans stdout/stderr for pattern until match or timeout; does NOT watch process exit
- `Check()`: dispatcher that calls the appropriate check method, then signals via `process.SignalReady(err)`
- **Problem**: If process exits before readiness completes, checks wait full timeout (30s default × 3 retries = 90s)

**Process Interface** (`internal/app/runner/process.go`):
- `Done() <-chan struct{}` - closes when process exits (exists but unused by readiness)
- `Ready() <-chan error` - receives readiness result
- `SignalReady(err)` - called by readiness checker to signal completion

**Registry** (`internal/app/runner/registry.go`):
- Only tracks successfully started processes via `Add(name, proc, tier)`
- Failed-at-startup services are NOT added to registry
- `restartService()` can restart any service from config, doesn't require registry entry

**UI FSM** (`internal/app/ui/services/state.go`):
- States: Stopped, Starting, Running, Stopping, Restarting, Failed
- Transitions already support: `Restart` from `Failed` state (line 46)
- `handleServiceFailed()` stores error in `ServiceState.Error` field
- View renders error message next to failed service status

**Events** (`internal/app/runtime/events.go`):
- `EventServiceFailed` with `ServiceFailedData{Service, Tier, Error}` - already published per service
- `EventTierStarting`, `EventTierReady` exist but no `EventTierFailed`

### Root Causes

1. **Exit on first failure**: `startTier()` line 396-398 returns first error; `startAllTiers()` line 486-487 propagates it up
2. **Readiness timeout hang**: `CheckLog()`/`CheckHTTP()` don't monitor `Process.Done()` channel
3. **No tier failure event**: UI has no way to know a tier partially failed

---

## Implementation Plan

### 1. Add failing example service
- Create `examples/bookstore/failed-service/Makefile` with `make run` that prints a message and exits 1
- Add to `fuku.yaml` in foundation tier with log readiness pattern that won't match

### 2. Fix readiness hang on early process exit
**Files**: `internal/app/runner/readiness.go`

- Modify `CheckHTTP()` signature to accept `done <-chan struct{}`
- Modify `CheckLog()` signature to accept `done <-chan struct{}`
- In both methods: add `case <-done:` to select statements, return `ErrProcessExited` (new error)
- Update `Check()` to pass `process.Done()` to check methods

**New error**: Add `ErrProcessExited` to `internal/app/errors/errors.go`

### 3. Add EventTierFailed event
**Files**: `internal/app/runtime/events.go`

- Add `EventTierFailed` event type
- Add `TierFailedData` struct with `Name string`, `FailedServices []string`, `TotalServices int`

### 4. Keep app alive on startup failures
**Files**: `internal/app/runner/runner.go`

**4a. Modify `startTier()` (lines 341-402)**:
- Instead of returning first error, collect all errors and failed service names
- Continue starting all services in the tier
- Return a new struct or use error wrapping to indicate partial failure vs total failure
- Still publish `EventServiceFailed` per failed service (already done)

**4b. Modify `startAllTiers()` (lines 474-501)**:
- Track which tiers had failures
- On tier failure: publish new `EventTierFailed` event
- Stop processing subsequent tiers (per requirement: foundation failure stops platform/edge)
- Return `nil` instead of error to allow transition to running phase
- Alternatively: return a "partial success" indicator that `runStartupPhase()` handles specially

**4c. Modify `runStartupPhase()` (lines 148-210)**:
- Handle partial startup success: don't call `shutdown()` and return error
- Transition to running phase even with failures
- Log which services failed but continue

### 5. UI handling for EventTierFailed (optional enhancement)
**Files**: `internal/app/ui/services/update.go`

- Add handler for `EventTierFailed` if we want to show tier-level failure indication
- Currently per-service `EventServiceFailed` is sufficient for basic functionality

### 6. Verification
- FSM already supports `Restart` from `Failed` state ✓
- `restartService()` already works for services not in registry ✓
- Error display in UI already implemented ✓

---

## Definition of done
- Running the bookstore profile with the failing service leaves fuku up, with other services running and the failed one shown as failed.
- Restarting the failed service from the UI/command bus works.
- Startup no longer hangs for the full readiness timeout when the process has already exited.
- Tests cover the new behavior (deferred to end).
- Linter and existing tests pass.
