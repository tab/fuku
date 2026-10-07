# REST API defaults

## Goal

Every `fuku run` serves the REST API on `127.0.0.1:3858` without any config.
A tool or an agent reads service state and acts on one service without editing `fuku.yaml` or handling a secret.

## Why

Today the API runs only when `server.listen` is set, and `listen` requires `server.auth.token`.
An agent or an IDE plugin that meets a project must ask the user to edit the config and then handle the token.
The token cannot stop a local process, which can read `fuku.yaml`. It only stops a browser page,
and `Host` and `Origin` checks do that without a secret ([BL-017](../backlog.md)).

## Scope

In:

- The API on by default at `127.0.0.1:3858`, with the existing walk over the next 9 ports
- The opt-outs `listen: ""`, `listen: none` and `FUKU_API_DISABLED=1`, and an optional `server.auth.token`
- A browser guard on every `/api/v1/` request: a loopback `Host` and no `Origin`
- Unit tests in `adapters/config` and `adapters/rest`, and e2e scenarios beside the existing API suite
- `README.md`, the `config` skill, `api.astro`, `configuration.astro`, the agents plugin page, `spec/openapi.yaml`,
  and the `rest` and `config` package READMEs
- The agent skill's `SKILL.md`, `references/control-api.md` and `scripts/api.sh`, with its structure check in `tests/test_plugin.py`
- The JetBrains plugin's default port, from 9876 to 3858, in `Settings.kt`, `ApiClient.kt`, `jetbrains.astro` and `plugins/jetbrains/CLAUDE.md`

Out:

- A CORS allowlist (`server.cors.*`), and a listen address that is not loopback
- The API address in `fuku doctor --json`, and a generated or persisted token
- The `fuku init` template, which has no `server` block
- `CHANGELOG.md` and the JetBrains plugin version. The release bump writes both

## How it works

1. Without a `listen` key, the listen address is `127.0.0.1:3858`. A `server` block with only `auth.token` keeps it
2. A loopback `host:port` in `server.listen` replaces that address
3. `listen: ""` or `listen: none`, in the config or the override, turns the API off. `FUKU_API_DISABLED=1` wins over the file
4. The server binds the address or the first free one of the next 9 ports. `fuku run -d` prints `API <host:port>`
5. Every `/api/v1/` request passes the browser guard first. A `Host` that is not loopback, or any `Origin`, gets `403`
6. With a token set, the authed routes need the bearer exactly as today. Without one, they need no header

A loopback host is `localhost`, `ip6-localhost` or an IP that `net.ParseIP(host).IsLoopback()` accepts.

## Acceptance criteria

- **AC1** – without a `server` block, the projected listen address is `127.0.0.1:3858`
- **AC2** – a `server` block with only `auth.token` keeps the default address
- **AC3** – a loopback `server.listen` replaces the default address
- **AC4** – `server.listen` without `server.auth.token` passes validation
- **AC5** – a `server.listen` that is not loopback still fails validation
- **AC6** – `listen: ""` in `fuku.yaml` turns the API off
- **AC7** – `listen: none` in `fuku.yaml`, quoted or not, turns the API off
- **AC8** – `listen: ""` or `listen: none` in `fuku.override.yaml` or `fuku.override.yml` turns the API off while `fuku.yaml` sets an address
- **AC9** – `None`, `off` and `false` fail validation with the existing invalid-listen error
- **AC10** – `FUKU_API_DISABLED=1` turns the API off when the file sets an address
- **AC11** – `FUKU_API_DISABLED` with any value other than `1` leaves the API on
- **AC12** – with the API off, `fuku run -d` prints no `API` line
- **AC13** – with a token set, an authed route answers `401` to a missing or wrong bearer, as today
- **AC14** – without a token, every authed route answers a request that has no `Authorization` header
- **AC15** – a `/api/v1/` request whose `Host` is not loopback gets `403`, even on a probe and with a valid token
- **AC16** – `localhost`, `ip6-localhost` and every loopback IP, such as `127.0.0.2` and `[::1]`, pass the guard, with and without a port
- **AC17** – a `/api/v1/` request with an `Origin` header, whatever its value, gets `403`, even with a loopback `Host` and a valid token
- **AC18** – a run with no `server` block prints `API` with port 3858 or the next free port, and `GET /api/v1/status` without a bearer answers `200`
- **AC19** – the docs and the `config` skill name the default address, the off spellings `""` and `none`, the env var and the optional token. None says the API is off by default or needs a token
- **AC20** – `api.astro` says once, next to the default address, that 3858 spells F-U-K-U on a phone keypad
- **AC21** – the agent skill says the API is on by default, the summary's `API` line gives the address and the token is optional, with no `listen:` grep as the sign
- **AC22** – `scripts/api.sh` sends the request without an `Authorization` header when the effective config sets no `server.auth.token`
- **AC23** – `scripts/api.sh` still sends nothing and exits 1 when the token has a form it cannot read
- **AC24** – the JetBrains plugin's default port equals fuku's default port, 3858

AC1 to AC11 are `config` unit tests, and AC13 to AC17 are `rest` unit tests. AC12, AC17 and AC18 run in the e2e suite.
AC22 and AC23 are structure-check tests. The plugin has no test suite, so the review checks AC24.

## Assumptions

- A rejected request gets `403` with `{"error": "forbidden"}`. The guard runs before the token check, so a browser never sees `401`
- A request without a `Host` header, which HTTP/1.0 allows, counts as not loopback, and so does an absolute-form request target
- `server.listen` is validated even when `FUKU_API_DISABLED=1`, so the env var does not hide a typo
- With `--config <file>`, fuku reads no override, as the loader does today. An override's `listen` then has no effect
- A `listen` with no value or `~` is YAML null, not `""`. It keeps the default, and in the override it deletes the key as every null does

## Contracts

- `server.listen` takes a loopback `host:port`, `""` or the lowercase word `none`, quoted or not. Anything else fails validation
- Absent and empty differ. The loader pre-fills `127.0.0.1:3858` before viper decodes, so a missing key keeps the default and `""` clears it
- The override merge keeps an explicit `""` from `fuku.override.yaml` or `.yml` and does not treat it as unset
- `FUKU_API_DISABLED=1` is only an off-switch, like `FUKU_TELEMETRY_DISABLED` and `FUKU_UPDATER_DISABLED`. No environment variable sets a YAML key
- `model.Server.Listen` is empty exactly when the API is off, so the run composition keeps its `Listen != ""` gate
- `spec/openapi.yaml` documents the `403` on every route and marks the bearer as optional

## Decisions

- Port 3858 spells "fuku" on a phone keypad. It stays clear of common developer ports: 3000–3010, 4000, 5000, 5173, 8000, 8080 and 9000
- The default binds `127.0.0.1`, not `localhost`: no resolver, and the plugin and `api.sh` call `127.0.0.1`
- The browser guard replaces the token as the default defence and applies with or without a token.
  This feature delivers BL-017. Its backlog entry is closed outside this feature's files
- The guard and the `server.listen` validation accept the same loopback hosts, so a valid `listen` is always reachable
