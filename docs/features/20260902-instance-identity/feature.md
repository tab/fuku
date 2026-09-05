# Instance identity

## Goal

Let a running fuku process report which process it is and which project directory it serves.
A client that finds a fuku API or log socket can then tell whether it belongs to the project it cares about.

## Context

A fuku process is anonymous today.
`GET /api/v1/status` reports the version, profile, phase and uptime.
The relay status message reports the version, profile and service list.
Two processes started from different project directories look identical in both, and a process that replaced another looks like the one it replaced.

A tool that finds a JSON endpoint on loopback needs to answer three questions before it sends anything: is this fuku, is it my project, and is it still the same process.
It has no token at that point.
A relay client attaching to a log socket needs the same answers.
Handing either one the absolute project path would leak the developer's directory layout to anything that can reach loopback.

## Scope

### In

- A new `internal/app/instance` package that holds the identity of one fuku process
- One identity per process, wired through FX to the API server and the relay server
- A new `ErrFailedToResolveProject` in `internal/app/errors/errors.go`
- Identity fields on `GET /api/v1/live`, `GET /api/v1/status` and the relay status message
- The matching `spec/openapi.yaml` update and a test that asserts the two response schemas against it
- The `/live` and `/status` examples on `docs/src/pages/docs/api.astro`
- The relay status example and the `/live` and `/status` rows in `ARCHITECTURE.md`, which quote the exact wire format

### Out

- The single-instance guard that refuses a second `fuku run` for the same project
- Instance scanning and discovery across ports: this feature reports identity, it does not find instances
- Service revision counters on the service serializer
- Log history endpoints, tail and since queries, and relay history replay
- CLI, doctor, agent plugin files and distribution
- The documentation site beyond the two response examples on the API page
- Unrelated lint or config cleanup

## Expected behavior

### Main flow

1. `cli.ChangeToConfigDir` moves the process into the project directory
2. The process builds one `Identity` and supplies it to the FX graph
3. The API server and the relay server read that identity and report its fields

### Identity fields

- `ID` – a fresh UUID, new on every process start
- `Project` – the working directory as a canonical absolute path with symlinks evaluated, which `New` resolves once
- `Fingerprint` – the first 16 lowercase hex characters of the SHA-256 of `Project`

### Where each value appears

- `GET /api/v1/live`, unauthenticated, keeps `status` and gains `product` (always `fuku`), `instance` with the UUID and `fingerprint` with the digest
- `GET /api/v1/status`, authenticated, gains `instance` with the UUID and `project` with the canonical path
- The relay status message gains `instance` with the UUID and `fingerprint`
- `GET /api/v1/ready` does not change

### Acceptance criteria

- **AC1** – two identities created in the same directory have different `ID` values and the same `Fingerprint`
- **AC2** – `Fingerprint` returns a fixed 16-character lowercase hex digest for any path
- **AC3** – `Fingerprint` is deterministic and matches fixed SHA-256 prefix test vectors
- **AC4** – two spellings of one directory that differ only by a symlink produce the same fingerprint through `New`
- **AC5** – `New` returns an error wrapping `errors.ErrFailedToResolveProject` when the working directory cannot be read or canonicalized
- **AC6** – `GET /api/v1/live` returns `status`, `product` set to `fuku`, the UUID and the fingerprint, and no absolute path
- **AC7** – `GET /api/v1/status` returns the UUID and the canonical absolute path
- **AC8** – `GET /api/v1/ready` returns the body it returns today
- **AC9** – the relay status message carries the UUID and the fingerprint
- **AC10** – the `Live` and `Status` schemas in `spec/openapi.yaml` list each new field as required and declare its type and promised value constraints
- **AC11** – against a running instance, `/live` and `/status` report the same UUID, and the `/live` fingerprint is the SHA-256 prefix of the path on `/status`

## Assumptions

- `github.com/google/uuid` is already a direct dependency, so no new module is needed
- `go.yaml.in/yaml/v3` is already a direct dependency, so the schema test needs no new module
- The doctor command returns before identity creation and keeps its current behavior
- Only the API server and the relay server consume the identity in this feature

## Contracts

Each field name carries one meaning on every surface.
`instance` is always the UUID, `fingerprint` is always the digest, and `project` is always the canonical absolute path.
No surface reports a path under a name that carries a digest elsewhere.

The fingerprint is a fixed 16-character lowercase hex digest and does not return or encode the raw path.
It is derived from that path, so anyone who can guess the directory can compute the digest and confirm the guess.
A caller that knows its own directory can check for a match.
A caller that does not know it learns only a hex string.
Treat it as a match key, not as a security boundary.
Different project paths can theoretically share a fingerprint because the digest is truncated.

The UUID is random per process and says nothing about the machine, the user or the project.
Unauthenticated callers get it so they can tell one process from the process that replaced it.

Every change adds a JSON or wire field.
A client that ignores unknown fields keeps working: `status` still appears on `/live`, `version`, `profile` and `services` still appear on the relay status message, and no existing field is renamed, retyped or removed.

## Decisions

- `/live` carries the UUID as well as the fingerprint, so an unauthenticated caller can tell a restarted process from the one it saw a moment ago; the UUID discloses nothing on its own
- `/live` carries `product` from `config.AppName`, so a client probing a loopback port can tell fuku from any other service that answers JSON on that route
- The digest field is `fingerprint` and the path field is `project`, because a single name returning 16 hex characters on one endpoint and an absolute path on another would carry two meanings
- 16 hex characters instead of the full digest, because a collision is unlikely across the project directories on one machine and a short value stays readable in logs and probe output
- `New` owns the working directory and symlink resolution while `Fingerprint` hashes the path it is handed, so canonicalization happens once and the hash stays a pure function that tests can drive with fixed input
- The path is canonicalized with symlinks evaluated, so `/tmp/app` and `/private/tmp/app` on macOS do not become one directory with two fingerprints
- The identity is created in `cmd/main.go` instead of an FX provider, because it has to exist after `ChangeToConfigDir` and before the FX graph is built
