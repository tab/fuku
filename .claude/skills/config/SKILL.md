---
name: config
description: Reference for the fuku.yaml schema — every key with its default and validation rule, the environment flags and the override merge. Use when editing fuku.yaml, adding a service, a readiness probe or the API server, or explaining a config field.
---

# fuku.yaml reference

The schema is `internal/adapters/config/config.go`. Defaults live in `constants.go` and `project.go`. Rules live in `validate.go`.
`fuku.yaml`, then `fuku.yml`, in the working directory is read. `--config <path>` reads that file instead.
Unknown keys are ignored, so `x-*` keys can hold YAML anchors.

## Structure

```yaml
version: 1

services:
  service-name:
    dir: path/to/service            # default: the service name
    command: go run cmd/main.go     # default: make run
    tier: foundation                # default: defaults.tier, else "default"
    profiles: [backend]             # parsed, read by nothing
    readiness:
      type: http                    # http | tcp | log
      url: http://localhost:8080/health   # http: required
      address: localhost:5432       # tcp: required
      pattern: "listening on"       # log: required, a regexp
      timeout: 30s                  # default: 30s
      interval: 500ms               # default: 500ms
    logs:
      output: [stdout, stderr]      # default: both
    watch:
      include: ["**/*.go"]          # required when watch is set
      ignore: ["**/*_test.go"]      # default: none
      shared: ["pkg/common"]        # default: none
      debounce: 500ms               # default: 500ms
    env:
      files: [.env, .env.local]     # default: .env, .env.local, .env.development, .env.development.local

defaults:
  tier: default                     # tier of a service that sets none
  profiles: [default]               # parsed, read by nothing

profiles:
  default: "*"                      # default: "*"
  backend: [service1, service2]

exclude: [heavy-worker]             # default: none

logging:
  format: console                   # console | json, default: console
  level: info                       # debug | info | warn | error, default: info

concurrency:
  workers: 5                        # default: 5

retry:
  attempts: 3                       # default: 3
  backoff: 500ms                    # default: 500ms

logs:
  buffer: 1000                      # default: 1000
  history: 5000                     # default: 5000

server:
  listen: "127.0.0.1:9876"          # default: empty, the server is off
  auth:
    token: "my-dev-token"           # required with listen
```

## `services`

- `dir` is the working directory of the child. Defaults to the service name.
- `command` runs through `sh -c`. Defaults to `make run`. A command of only whitespace is invalid.
- `tier` is the startup bucket. Names are trimmed and lowercased. Tier order is the first appearance in the file.
  A service without a tier runs last, in `default`. An unknown tier falls back to `default`.
- `profiles` is parsed and filled from `defaults.profiles`. Nothing reads it.
- the child inherits fuku's environment. Fuku exports nothing per service.

### `readiness`

- `type` is required: `http`, `tcp` or `log`. Any other value is invalid.
- `http` requires `url`. `tcp` requires `address`. `log` requires `pattern`, a Go regexp matched against stdout and stderr.
- `timeout` and `interval` default to 30s and 500ms when unset.

### `logs`

- `output` lists the streams to capture. Each entry is `stdout` or `stderr`, case-insensitive. Default: both.

### `watch`

- `include` is required when `watch` is set. Globs relative to `dir`.
- `ignore` lists globs to skip.
- `shared` lists directories outside `dir` that also restart the service. Relative to fuku's working directory.
- `debounce` is the quiet window after a change. Default 500ms.

### `env`

- `files` lists `.env` files relative to `dir`. They feed the TUI env tab only. Nothing reaches the child.
- absent: the four defaults. An explicit `[]` loads nothing. A path that leaves `dir` is an error.

## `defaults`

- `tier` fills a service without one. Normalized like a service tier.
- `profiles` fills a service without its own. Nothing reads either.

## `profiles`

- `"*"` is every service. A list names services. Every name must exist in `services`. Any other value is invalid.
- `default` is seeded with `"*"`. It runs when the command names no profile.

## `exclude`

- names dropped at profile resolution. Trimmed and deduplicated. An unknown name is ignored.
  A profile may still list an excluded service.

## `logging`

- `format` is `console` or `json`. `level` is `debug`, `info`, `warn` or `error`. Not validated.

## `concurrency`

- `workers` must be positive. Services in one tier start up to `workers` at once.

## `retry`

- `attempts` must be positive. `backoff` must not be negative.

## `logs`

- `buffer` (per-client queue) and `history` (replay) must be positive.

## `server`

- an empty `listen` disables the REST API.
- a set `listen` requires `auth.token`. It must be `host:port` with a loopback host (`127.0.0.1`, `::1`, `localhost`, `ip6-localhost`) and a port in 1–65535.

## Environment flags

- `FUKU_UPDATER_DISABLED=1` turns the update check off.
- `FUKU_TELEMETRY_DISABLED=1` turns telemetry off.

## Override

`fuku.override.yaml`, or `.yml`, beside `fuku.yaml` merges on top. `--config` skips it.

- mappings merge by key. A scalar or a mismatched kind is replaced by the override.
- sequences are concatenated, override items after base.
- `null` deletes the key. Base anchors and `<<` merge keys resolve inside the override.
