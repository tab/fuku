# Project-scoped sockets plan

Feature: [feature.md](feature.md)

Status: implemented

Phase: code review

Current step: PR preparation

## Approach

Replace the profile in the socket name with the project fingerprint and hand the fingerprint from `instance.Identity`,
which FX already provides, to every place that builds the path.

Identity: `relay.SocketPathForProfile(dir, profile)` becomes `instance.SocketPath(dir, fingerprint)`, next to
`Fingerprint`, because the guard needs the path and `relay` imports `instance`.

Relay: `FindSocket(dir, fingerprint)` stats that one path and returns `ErrNoInstanceRunning` when it is absent or not
a socket; the glob, `ErrSocketSearchFailed`, `ErrInstanceNotFound` and `ErrMultipleInstancesRunning` go away with it.
`Cleanup` keeps its `fuku-*.sock` glob.
The server builds its path from the fingerprint it already holds.

Guard: `RegisterGuard` registers the check for every `run`, not only when `server.listen` is set.
`guard.Check` first dials `SocketPath(config.SocketDir, fingerprint)` with `SocketDialTimeout`; an answering socket is
another instance of this project, and the refusal names it as `socket <path>`.
A stale socket file refuses the connection and is ignored, which keeps `Test_Lifecycle_PreflightCleansUpOrphans`
working.
The API probe runs after the socket, only when `server.listen` is set, and its refusal names `API at <address>`.

Logs: `NewScreen` receives `instance.Identity` through FX and looks up `FindSocket(config.SocketDir, identity.Fingerprint)`;
a missing socket is reported as "No fuku is running for project '<dir>'", because the console writer prints only the
message.
The profile expectation lives in the screen handler.
`Handler.HandleStatus` returns an error, the client stops the stream on a non-nil result in both the acknowledgement path
and `dispatch`, and the screen handler logs both profile names and returns `ErrProfileMismatch` when `--profile` names
another profile.
`streamLogs` skips its generic stream error log for that error so the message is printed once.

Doctor: `bootstrap.DoctorCLI` runs the doctor command inside a small FX container with
`fx.Provide(instance.NewInstance)` and an `fx.Invoke` that calls `cli.RunDoctor(cmd, identity)` and captures the exit
code; a container error prints `Error: <dig.RootCause(err)>` and returns 1, so an unresolvable working directory shows
the lstat failure rather than the dig construction trace.
`doctor.Options` and `Env` gain `Fingerprint`.
`checkInstance` looks the socket up through `relay.FindSocket` and reports "no other fuku running for this project" or
"another fuku is running for this project"; `checkStaleSockets` keeps its glob.

Plugin: `Probe` gains `fingerprint: String? = null` and `PluginState` gains `fingerprint: String? = null`.
The poll reads `/live` once per instance: it keeps the instance id from `/status` with the fingerprint it answered and
asks again only when the id changes, so a tick costs two requests; a failed probe leaves the fingerprint empty for
that tick, which keeps the state connected and the socket unconnected until the next poll.
A settings change drops the cached pair and publishes a disconnected state before the poll restarts against the new
client, so the log stream of the old target closes at once.
`LogStreamClient` connects only when the state is connected and the fingerprint is non-blank, reconnects when the
fingerprint changes and disconnects when it becomes blank, so a missing fingerprint never builds a socket path.
It remembers the fingerprint of the latest state; a queued connection for an older state does nothing, and a
connection that completes after the wanted fingerprint changed is closed instead of installed.

e2e: `suite.go` gains a `SocketPath(t, dir)` helper that resolves the directory's symlinks, hashes that resolved path
with SHA-256 and takes the first 16 hex characters, because the suite must not import `fuku/internal`.
The two-project test runs `testdata/tier` and `testdata/default-tier`, which already share the `default` profile with
distinct service names.

## Steps

- [x] Replace `SocketPathForProfile` with `SocketPath`, simplify `FindSocket`, remove the three orphaned errors, add
  `ErrProfileMismatch` and bind the server to the fingerprint path – AC1
- [x] Let `Handler.HandleStatus` return an error, stop the stream on it in the acknowledgement path and `dispatch`, and
  regenerate `client_mock.go` – AC4
- [x] Inject `instance.Identity` into `NewScreen`, look up the project socket and enforce the `--profile` expectation in
  the screen handler – AC3, AC4, AC5
- [x] Run doctor inside an FX container, pass the fingerprint through `Options` and `Env` and make `checkInstance`
  report per project – AC6
- [x] Add `fingerprint` to the plugin `Probe` and `PluginState` and `instance` to `Status`, read the probe once per
  instance in the poll and connect `LogStreamClient` only on a non-blank fingerprint – AC7
- [x] Move `SocketPath` into `instance`, dial the project socket in the guard ahead of the API probe and register the
  guard for every `run` – AC10
- [x] Update the relay socket, server and client tests, the logs screen tests, the doctor runtime tests and the
  bootstrap doctor and service tests – AC8
- [x] Add the e2e `SocketPath` helper, point `Test_Lifecycle_SocketCleanup` at it, add `Test_Logs_ProjectScoped`,
  `Test_Logs_ProfileMismatch`, `Test_Logs_NoInstance` and `Test_Instance_RefusesSecondRunWithoutAPI` – AC2, AC8, AC10
- [x] Update the socket path in `ARCHITECTURE.md`, describe the project socket on the log-streaming page and describe
  `--profile` as an expectation in the CLI help, the flag description, `README.md` and the docs site – AC9

## Verification

Unit tests follow the repository's table-driven, mocks-once pattern.

- `internal/app/instance/instance_test.go` – `SocketPath` format
- `internal/app/instance/guard_test.go` – a live project socket without the API refuses the run and names the socket;
  a stale socket file lets it proceed; the API cases as before
- `internal/app/app_test.go` – `RegisterGuard` registers the hook for `run` whether or not the API is configured
- `internal/app/relay/socket_test.go` – `FindSocket` for a live socket, an absent path and a regular file
- `internal/app/relay/server_test.go` – the bound path is the fingerprint path
- `internal/app/relay/client_test.go` – `Stream` returns the handler's status error before dispatching any log frame,
  in both the acknowledgement path and the plain path
- `internal/app/logs/screen_test.go` – lookup uses the identity fingerprint and a miss logs the project once; a
  mismatched `--profile` exits 1, logs both names once and renders no banner or log line; a matching or absent flag
  streams without an error log
- `internal/app/doctor/runtime_test.go` – `checkInstance` by fingerprint for an absent socket, a live socket and a
  stale socket
- `internal/bootstrap` – `DoctorCLI` exits 1 for a deleted working directory, where the identity container fails;
  `ServiceCLI` `logs` without a profile and without an instance runs the container to its exit and returns 1
- `e2e/lifecycle_test.go` – socket cleanup with the fingerprint path
- `e2e/instance_test.go` – `Test_Instance_RefusesSecondRun` expects the socket in the refusal, since the socket answers
  before the API probe runs; `Test_Instance_RefusesSecondRunWithoutAPI`: a second `fuku run` in `testdata/default-tier`
  exits 1 naming the socket, the first instance still serves `fuku logs` and stops no service
- `e2e/logs_test.go` – `Test_Logs_ProjectScoped`: both fixtures run on `default`, a bounded `fuku logs --no-ui
  --no-follow` in each directory contains only that fixture's service names; `Test_Logs_ProfileMismatch`: `--profile
  core` against a `default` instance exits 1 and the output names both profiles; `Test_Logs_NoInstance`: `fuku logs`
  in a directory without an instance exits 1 and names the project it looked for

Checks, in the repository's order: `make fmt`, `make vet`, `make lint`, `make test`, `make build`, `make test:e2e`,
`make test:race`, then `make lint:plugin`, `make build:plugin` and `npm run build` in `docs/`.

## Gates

- [x] Plan review – PASS (round 4 focused recheck, codex, gpt-5.6-luna, medium; rounds 1–3 with gpt-5.6-terra, high:
  CHANGES NEEDED, PASS, PLAN-3; PLAN-1, PLAN-2 and PLAN-3 resolved)
- [x] Code review – PASS (round 15 focused recheck of the CODE-9 fix, codex, gpt-5.6-luna, medium; round 14 focused recheck of the added bootstrap tests: PASS WITH FOLLOW-UPS, CODE-9 fixed; round 13 full review of the bootstrap package against 0afcd3e, codex, gpt-5.6-luna, high; round 12 focused recheck of the bootstrap package, codex, gpt-5.6-luna, medium; round 11 focused recheck, codex, gpt-5.6-luna, medium; round 10 full review against 0afcd3e with gpt-5.6-luna, high: CHANGES NEEDED, CODE-8 fixed, CODE-6 backlog BL-006, CODE-7 disputed and accepted; round 9 focused recheck, codex, gpt-5.6-luna, medium; round 8 full review against 0afcd3e
  with gpt-5.6-luna, high: PASS WITH FOLLOW-UPS, CODE-4 and CODE-5 resolved; round 7: PASS; rounds 4–5: PASS WITH
  FOLLOW-UPS, CODE-2 and CODE-3 resolved; rounds 2–3: PASS; round 1: PASS WITH FOLLOW-UPS, CODE-1 resolved)
- [ ] PR review – not run
