# Tier-Based Startup – Implementation Notes

## 1. Objectives
- Introduce tiered startup for services to reduce simultaneous load and enforce sensible ordering without numeric priority tuning.
- Keep configuration readable; tiers are optional and default behaviour should remain “start everything” if omitted.

## 2. Config Schema Changes
- Extend `internal/config/config.go`:
  - Add `Tier` field on `Service` struct (`yaml:"tier"`).
  - Accepted enum values: `foundation`, `platform`, `edge` (ordered):
    - `foundation` – infrastructure/auth/bootstrap components everything depends on (databases, identity, discovery).
    - `platform` – domain services and core business logic that power internal workflows.
    - `edge` – API gateways, web front-ends, or external adapters exposed to clients.
  - Services with no `tier` stay in the default parallel group.
- Support group/profile defaults:
  ```yaml
  defaults:
    tier: platform
  services:
    auth-service:
      tier: foundation
    payments-service:
      tier: business
    admin-api:
      tier: edge
  ```
- Enforce validation: reject unknown tier values, circular dependencies remain errors, but tier itself does not override dependencies.

## 3. Readiness Strategy
- Extend service configuration to accept an optional `readiness` block (per service) with two first-class probes:
  1. **HTTP endpoint** – specify `readiness:
       type: http
       url: "http://localhost:8000/v1/status"
       timeout: 30s
       interval: 500ms`
     Runner polls the endpoint until it returns a 2xx status (timeout/failure aborts the service and reports error).
  2. **Log pattern** – specify `readiness:
       type: log
       pattern: "HTTP server started"
       timeout: 30s`
     Runner watches stdout/stderr for regex match; once seen, service marked ready.
- Allow multiple shorthand names if common (`pattern: "RPC server started"`, `pattern: "SQS listener started"`), but keep configuration declarative; provide anchors for reuse, e.g.:
  ```yaml
  x-readiness-http: &readiness-http
    type: http
    timeout: 30s
    interval: 500ms

  services:
    api:
      readiness:
        <<: *readiness-http
        url: http://localhost:8000/v1/status
    auth:
      readiness:
        type: log
        pattern: "RPC server started"
        timeout: 30s
  ```
- Provide sane defaults (timeout=30s, interval=500ms) so minimal configs only set `type` + `pattern`/`url`.
- Validation rules:
  - `type` must be `http` or `log` (extendable later).
  - `url` required for HTTP readiness; pattern required for log readiness.
  - Timeout/interval strings parsed via `time.ParseDuration`; on parse failure, surface config error.
- Runner responsibilities:
  - Begin readiness probe immediately after process start.
  - Retry failed services up to `ReadinessRetries` (constant in `config.go`, default 3). Between retries, back off briefly (e.g., 2s) and relaunch the process.
  - If readiness still fails after exhausting retries, mark the service failed, stop the process, block progression to lower tiers, and surface the error via logs/events (UI later).
  - When a probe succeeds, mark the service ready and emit readiness events so downstream consumers can render state transitions and durations.

## 4. Runner Enhancements
- After resolving dependencies, group services by tier order:
  1. foundation
  2. platform
  3. edge
  4. default (no tier)
- For each tier:
  - Maintain dependency readiness (a lower-tier service can start earlier if dependencies already running).
  - Apply configurable concurrency cap (e.g., default 3). Start next service in tier only when cap allows and dependencies ready.
  - Wait for readiness success/failure before moving to next tier; propagate errors immediately.
- Surface tier info via event stream for UI (e.g., part of `ServiceState`).
- Tests: ensure ordering respects dependencies plus tiers, concurrency cap works, services without tiers remain unaffected, and readiness retries behave as expected.

## 5. CLI/Docs Updates
- Document tier usage in README or dedicated doc.
- Provide examples showing how core/auth services map to `foundation`, domain services to `business`, etc.
- Clarify that tiers are optional; legacy configs continue to work without edits.
- Update config samples (`fuku.yaml`, docs) to drop any outdated keys once the schema changes land.

## 6. Future Considerations
- Allow per-profile overrides (e.g., `profiles.production.tier_order`).
- Expose runtime configuration for concurrency cap (`--max-tier-workers` flag).
- Consider metrics around start duration per tier for observability.
