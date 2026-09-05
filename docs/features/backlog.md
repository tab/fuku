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
