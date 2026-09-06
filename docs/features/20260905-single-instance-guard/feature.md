# Single-instance guard

## Goal

Prevent a second `fuku run` from starting for a project that already has a reachable fuku instance.
The guard must protect the running services from the second process's preflight cleanup.

## Context

The API may bind to the configured port or one of the next nine ports.
A second `fuku run` can therefore start on another port for the same project.
Its preflight cleanup may then stop services owned by the first process.

[Instance identity](../20260902-instance-identity/feature.md) lets a caller identify a fuku process and compare its project fingerprint.
The new guard can use that contract as the first FX startup check, before the application and preflight cleanup start.

## Scope

### In

- Probe the configured API port range from the first FX lifecycle hook
- Accept only a healthy fuku liveness response with the same project fingerprint
- Create one identity through FX and inject it into the guard and existing consumers
- Refuse the second run with exit code 1 and a useful message
- Keep the first process and its services running
- Add focused unit and end-to-end tests
- Document the user-visible behavior

### Out

- A public API for listing every running fuku instance
- Doctor checks for running instances
- Agent plugin discovery or control
- Log history or service control changes
- Detecting an instance whose API is disabled or unreachable
- A lock that closes the race between two processes started at the same time
- Changes to `fuku logs`, `fuku stop`, doctor or standalone commands

## Expected behavior

### Main flow

1. Fuku changes to the project directory and loads its config
2. FX creates one instance identity and injects it into the guard
3. The first `run` lifecycle hook probes the configured API port and the next nine ports
4. A healthy fuku response with the same fingerprint returns a startup error before later hooks run
5. When no matching instance is found, the remaining application hooks start normally

### Acceptance criteria

- **AC1** – `fuku run` returns exit code 1 before application execution when the port range contains a healthy fuku instance with the same fingerprint
- **AC2** – the refusal message says that fuku is already running for the project, includes the API address and suggests reading logs or stopping the instance
- **AC3** – a refused run does not stop or restart services owned by the existing process
- **AC4** – an instance for another project, another product, a non-200 response, invalid JSON or an unreachable address does not block startup
- **AC5** – commands other than `run` keep their current behavior and do not use the guard
- **AC6** – probing is limited to the configured ten-port range, uses a short request timeout and reads a bounded response body
- **AC7** – the guard sends no authentication token and accepts only `GET /api/v1/live` responses where `product` is `fuku`
- **AC8** – FX supplies one identity value to the guard, API server, relay server and every other identity consumer

## Assumptions

- When enabled, `server.listen` remains a loopback address with a base port and up to nine fallback ports
- The liveness response keeps `product`, `instance` and `fingerprint`
- The guard is best effort when the API is disabled, unreachable or too old to report a fingerprint
- Two processes started at the same time may both finish probing before either API binds

## Contracts

The probe sends unauthenticated `GET /api/v1/live` requests only within the configured API port range.
It never sends the project token.

A response matches only when it has HTTP 200, valid JSON, `product` equal to `fuku` and `fingerprint` equal to the current project's fingerprint.
The absolute project path is not read from the unauthenticated response.

Probe failures are treated as no match because they do not prove that the address belongs to the same project.

The guard runs as the first FX `OnStart` hook.
A confirmed match returns an error, so FX does not run the application, API or preflight paths for the second process.

## Decisions

- Extract only the guard and its private probe path now, because generic discovery belongs with doctor or agent integration
- Keep identity creation and guard construction inside FX instead of calling them directly from `cmd/main.go`
- Register the guard before the application and API lifecycle hooks so a match stops startup before preflight
- Fail open when a probe cannot identify a matching instance, because the guard cannot safely claim that another process owns the project
- Guard only `run`, because logs, stop and diagnostic commands need to work while another instance is active
