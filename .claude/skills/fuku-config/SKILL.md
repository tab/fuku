---
name: fuku-config
description: Reference for fuku.yaml configuration format — services, tiers, profiles, concurrency, retry, logs, watch. Use when editing fuku.yaml, adding a service, or explaining a config field.
---

# fuku.yaml reference

## Structure

```yaml
version: 1

services:
  service-name:
    dir: path/to/service
    tier: foundation
    command: go run cmd/main.go   # optional, defaults to "make run"
    logs:
      output: [stdout, stderr]    # optional, defaults to both
    watch:
      include: ["**/*.go"]
      ignore: ["**/*_test.go"]
      shared: ["../shared/**"]
      debounce: 500ms

profiles:
  default: "*"
  backend: [service1, service2]

logging:
  format: console                # console | json
  level: info                    # debug | info | warn | error

concurrency:
  workers: 5                     # max concurrent service starts (default 5)

retry:
  attempts: 3                    # default 3
  backoff: 500ms                 # default 500ms

logs:
  buffer: 1000                   # socket log streaming buffer (default 1000)
  history: 5000                  # replay history (default 5000)
```

## Services

- `dir` — directory the service runs in.
- `tier` — startup ordering bucket. Services in earlier tiers start before later tiers; services within a tier start concurrently (up to `concurrency.workers`).
- `command` — start command. Defaults to `make run`; a service without `command` must have a `Makefile` with a `run` target.
- `logs.output` — which streams to capture: `stdout`, `stderr`, or both (default).
- `watch.*` — file-change hot-reload (see below).

The child process **inherits fuku's environment**. Fuku does not inject per-service env vars; each service loads its own `.env` files. The `app/dotenv` loader reads `.env` files for display in the UI info aside only — values are not exported to the child process.

## Profiles

- `"*"` — all services.
- `[a, b, c]` — explicit list.
- `default` is used when no profile is named on the CLI.

## Watch (hot-reload)

- `include` — glob patterns to watch.
- `ignore` — glob patterns to exclude.
- `shared` — cross-service paths that also trigger restart in this service.
- `debounce` — coalesce rapid changes to avoid restart storms.

## Top-level `exclude`

Services or paths listed at the top level are excluded from profile resolution. See `internal/config` for the resolution order.
