# Code review

Mode: stress
Gate: code
Status: passed
Target: working tree on `feature/single-instance-guard` against `origin/master@0f0acbc`, including untracked files
General status: passed
General round: 2
General reviewer: codex
General model: gpt-5.6-luna
General effort: medium
Risk perspective: startup process safety
Risk status: passed
Risk round: 2
Risk reviewer: codex
Risk model: gpt-5.6-luna
Risk effort: medium

## General

Open finding: none

### Resolved

- CODE-G1 – resolved: bounded complete-body read plus `json.Unmarshal` rejects trailing data and read errors

### Checked

- `internal/app/instance/guard.go`
- `internal/app/instance/guard_test.go`
- `go test -count=1 ./internal/app/instance ./cmd` – passed
- Trailing JSON, trailing garbage, oversized body and exact-limit body tests passed

### Verdict

PASS

### Disposition needed

- None

## Startup process safety

Open findings: none

### Resolved

- CODE-R1 – resolved: `matches` reads at most `ProbeResponseSize+1`, rejects oversized bodies and uses whole-body `json.Unmarshal`; regression tests cover trailing JSON, trailing garbage and size boundaries
- CODE-R2 – resolved: the API-shutdown window is explicitly out of scope. The contract only protects a reachable healthy instance and states that disabled or unreachable APIs are best effort. The current change does not alter shutdown ordering or violate an in-scope acceptance criterion

### Checked

- `feature.md` goal, scope, assumptions, contracts and acceptance criteria
- `internal/app/instance/guard.go` and `guard_test.go`
- `cmd/main.go`, `internal/app/app.go`, API lifecycle and runner teardown ordering
- Focused primary checks passed and Claude reports the full suite passed
- Local focused test rerun was blocked by the sandbox refusing the `httptest` IPv6 listener, not by a code assertion

### Verdict

PASS

### Disposition needed

- None

## Combined verdict

PASS
