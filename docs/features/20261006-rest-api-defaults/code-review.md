# Code review

Mode: standard
Gate: code
Status: passed
Round: 4
Target: `ff515ab..7071c62` on `feature/rest-api-defaults`
Reviewer: codex
Model: gpt-6.1-sol
Effort: high

## Findings

### CODE-1 – OpenAPI still requires bearer authentication

Severity: medium
Status: fixed
Location: `spec/openapi.yaml:14`

Finding: The top-level `security` array contains only `bearerAuth: []`. This still declares authentication mandatory for every operation except the probes. The new security-scheme description does not change that requirement, so specification consumers receive a contract inconsistent with the default tokenless server and the approved optional-bearer contract. OpenAPI represents optional authentication by including an empty requirement object as an alternative. [OpenAPI 3.1 security requirements](https://spec.openapis.org/oas/v3.1.0.html#security-requirement-object)

Suggested fix: Add `- {}` alongside `- bearerAuth: []` in the top-level `security` array. Keep the probes’ `security: []` overrides and the description explaining when configured instances require a token.

Reply: Fixed by the round 2 commit. The top-level `security` lists `bearerAuth: []` and `{}`; the probes keep `security: []`. Redocly reports the spec valid with the one pre-existing warning, and `openapi.sh master` reports no drift.

Recheck: Resolved. `7a413e6` adds the empty alternative required for optional authentication. Both probes retain `security: []`, and the bearer description preserves the configured-token requirement.

### CODE-2 – HTTP/1.0 can pass the guard without a Host header

Severity: medium
Status: fixed
Location: `internal/adapters/rest/middleware.go:81`

Finding: The guard validates `r.Host`, which is not always the wire Host header. Go populates it from an absolute request target before consulting the header. Consequently, `GET http://127.0.0.1:3858/api/v1/status HTTP/1.0\\r\\n\\r\\n`, with no Host or Origin header, passes the guard and reaches the tokenless status handler. This contradicts the approved assumption that a missing Host gets `403`. The existing test manually clears `req.Host`, so it misses this parsing behavior. Source tracing confirms the trigger; no browser action exploit was identified.

Suggested fix: Reject absolute/authority-form request targets for this direct loopback API before trusting `r.Host`, or preserve and validate the original wire Host. Add a regression using `http.ReadRequest` to parse a raw HTTP/1.0 request without Host, and require `403` without calling the downstream handler.

Reply: Fixed by the round 2 commit. The guard now rejects `r.URL.IsAbs()` before the Origin and Host checks (`middleware.go:81`); in go1.27 `readRequest` sets `URL.Scheme` only for an absolute-form target. `Test_GuardMiddleware_AbsoluteTarget` parses `GET http://127.0.0.1:3858/api/v1/status HTTP/1.0` with `http.ReadRequest`, with and without a `Host` header, and requires `403` with `next` not called; removing the clause fails it. Authority-form (`CONNECT`) is left alone on purpose: `URL.Path` is empty, so no `/api/v1/` route matches and the mux answers `404` before any handler. The rest README and the feature.md assumption name the absolute-form rule.

Recheck: Resolved. The first guard condition rejects the original parsed request before downstream dispatch. The new test covers absolute-form targets with and without Host and checks status, response body and downstream exclusion. Leaving authority-form CONNECT unchanged is justified: Go parses its authority into `URL.Host` with an empty path and scheme; the current mux returns `404` without invoking an API route or authentication handler.

### CODE-3 – JetBrains can control another project with the new defaults

Severity: major
Status: accepted risk
Location: `plugins/jetbrains/src/main/kotlin/com/fuku/plugin/settings/Settings.kt:17`; `plugins/jetbrains/src/main/kotlin/com/fuku/plugin/PluginService.kt:104`; `plugins/jetbrains/src/main/kotlin/com/fuku/plugin/toolwindow/ServiceToolWindow.kt:323`

Finding: Start project A and then project B without server blocks. A binds 3858 and B walks to 3859. The plugin’s default client still connects to 3858. `PluginService` is application-wide, marks that connection usable after successful status and service-list calls, and never compares the server’s identity with the open project. Its fingerprint lookup records the connected server’s fingerprint without checking an expected one. Consequently, B’s tool window displays A’s services, and its start, stop and restart controls send A’s service IDs to A. The header shows only the profile, so two projects using `default` provide no project distinction. This path was source-traced, not reproduced in a running IDE.

The application-wide connection existed before this feature, but the matching default port and default tokenless API now make the wrong-project connection succeed without configuring either side. The per-project instance lock does not protect these actions: both runs are valid, and the IDs came from A’s own API.

Suggested fix: Keep port 3858 as the default, but verify the connected project before exposing service controls in each project window. The existing `/status` response includes `project`; decode it and compare canonical paths with the IDE project, or compare an independently derived project fingerprint. Suppress mismatched service controls, explain the mismatch, and let the user select the intended run’s bound port. Cover two concurrent default runs and verify that B’s window cannot dispatch an action to A.

Reply: Accepted as a risk by the human, not fixed here. The finding holds: `/status` carries `project`, the plugin's `Status` model drops it, and the application-level service never compares it with the open project. The plugin change is outside the approved scope, which names only the two default-port constants, and the human decided not to mix a plugin fix into this feature. It is tracked as BL-031 in `docs/features/backlog.md` (High), with BL-005 as its test prerequisite. The REST API, the config defaults and the plugin port stay as delivered.

Recheck: Accepted risk confirmed. The Reply matches the source and the human's explicit decision to defer the plugin fix as separate work. BL-031 (High), committed in `7071c62`, faithfully records the wrong-project display and action risk, project identity comparison, suppressed mismatched controls, mismatch explanation and bound-port selection. It names the plugin boundary, BL-005 and the code review source. The diff changes only the backlog; no evidence makes the risk materially worse than recorded. The defect remains unfixed at major severity, but its human-approved disposition does not block this gate.

## Checked

- Read `CLAUDE.md` first, then the requested preparation, checking, findings, reporting, code-review writing and template rules. Read the relevant docs and JetBrains area guides.
- Focused round 4 recheck covered only CODE-3, its Reply and disposition, and `7a413e6..7071c62`. CODE-1 and CODE-2 remain unchanged; their resolved findings were not reopened.
- Confirmed branch `feature/rest-api-defaults` and HEAD `7071c62`. The range contains one commit and touches only `docs/features/backlog.md`; the diff excluding that file is empty.
- Verified `/status` serializes `identity.Project` as `project`. The plugin's `Status` model omits it, and the client ignores unknown fields. `PluginService` remains application-scoped, accepts successful status and service-list calls without checking the IDE project, and dispatches service actions through the shared client.
- Checked the approved feature scope and plan: JetBrains implementation work covers the two default-port constants; project identity checking is separate plugin work. The human explicitly accepted the risk and deferred that fix to avoid mixing two fixes into one atomic feature.
- Verified BL-031 is under High and records the finding, the proposed repair, the plugin boundary, the absent test suite with BL-005, and the source link to this feature's code review. The unchanged Suggested fix retains canonical-path comparison and concurrent-run regression coverage.
- Verified BL-017 moved to Done with its existing text preserved and its checkbox checked. The feature contract explicitly states that this feature delivers BL-017.
- `git diff --check 7a413e6..7071c62` passed. No production code changed in this range, so prior automated checks were not rerun.
- No repository file changed. Git status and content hashes confirm the modified `plan.md` and untracked `code-review.md` remain as found. This is the complete replacement content for the read-only handoff.

## Verdict

PASS
