# Event-driven application architecture

## Goal

Give every feature one core package in `internal/app`, with its IO in adapters beside it.
Each command runs in its own Fx container around one bus.
The core publishes what happened, and the observers react.

## Context

- `internal/app` was one flat tree of 25 packages. Workflows, the bus, the worker pool and the TUI sat side by side
- `internal/config` also held the logger and Sentry (`config/logger`, `config/sentry`)
- `runner/service.go` mixed the retry loop with process creation and pipe handling
- The TUI wiring started the update checker (`go params.Checker.Run(ctx)` in `ui/wire/ui.go`)
- `registry/store.go` and `ui/services/update.go` each projected the lifecycle events into their own copy

## Scope

### In

- The five rings of [`ARCHITECTURE.md`](../../../ARCHITECTURE.md#rings) replace `internal/app` and `internal/config`
- One composition per command in `internal/bootstrap/modules`
- The coordinator and the outcome arbiter in `internal/bootstrap/lifecycle`
- One read model, `app/registry`, for the TUI, REST and the socket
- One admission policy, `services.Control`, for the TUI and REST
- Behavior changes:
  - the shared admission policy
  - `fuku logs` failures on stderr as `Error: <cause>`
  - the REST API sends no CORS headers
  - `logging.level` accepts only `debug`, `info`, `warn` and `error`
  - an override file fuku cannot stat fails the load, also under `--config`
  - stopping and restarting rows are amber in the TUI
  - the Sentry `path` tag reads the route pattern, with `{id}` for a service
  - an HTTP readiness request runs until the readiness deadline, not the interval
  - the log readiness probe reads lines up to 4 MiB
  - a service still waiting for a worker when the run ends is published as stopped, not failed

### Out

- CLI start, stop and restart of one service ([BL-008](../backlog.md))
- An external broker or a bus library ([BL-009](../backlog.md))
- Update polling and automatic installation
- Doctor or log endpoints in REST

## Expected behavior

- `cmd/main.go` calls `bootstrap.Run`. It parses the command, loads `fuku.yaml` and picks `modules.Run`
- `modules.Run` builds one `fx.App` with one bus. It binds each adapter to the interface its core package declares
- The composition hands `lifecycle.Coordinator` a guard, ordered consumers and producers, and one command
- The coordinator runs the guard, subscribes the consumers, starts the producers and runs the command
- `services.Runtime` runs the profile and publishes the phase, tier and service events
- The bus copies each event into every matching queue. A critical event needs a free slot in every required queue
- `registry.Store` applies each event to its snapshot and publishes `SnapshotChanged` when the snapshot changed
- `tui.Bridge` forwards `SnapshotChanged` to the view. The view updates and renders inside `Store.Read`
- REST serializes inside one `Store.Read` per request. The socket server reads the profile and names the same way
- A key or a `POST` calls `services.Control`. It publishes the admitted command, and `services.Runtime` acts on it
- On stop the coordinator stops the producers newest first, drains the consumers twice and closes the bus
- `lifecycle.Arbiter` keeps the first outcome. `lifecycle.Run` returns its exit code

### Acceptance criteria

Unit test paths are under `internal/`.

- **AC1** – Each composition validates and lacks the types its command does not need. `bootstrap/modules/base_test.go`
- **AC2** – Start runs in order and unwinds a failure. Stop runs in reverse. `bootstrap/lifecycle/hooks_test.go`
- **AC3** – The first outcome sets the exit code. A signal alone exits 0. `bootstrap/lifecycle/shutdown_test.go`
- **AC4** – A full required queue rejects a critical message. `platform/bus/bus_test.go`
- **AC5** – Every message type has one fixed criticality and wire string. `contracts/message_test.go`
- **AC6** – The registry announces only a change. Reads are race-free. `app/registry/store_test.go`, `query_test.go`
- **AC7** – The TUI and REST map the same `Control` outcomes. `app/services/control_test.go`,
  `adapters/rest/handler_test.go`, `adapters/tui/commands_test.go`
- **AC8** – Child output lines go to `process.LogSink`. `adapters/process/streams_test.go`
- **AC9** – An update notice published before the view attaches still reaches it. `adapters/tui/bus_test.go`
- **AC10** – Help, version and init load no config. Doctor without a config exits 2. `bootstrap/bootstrap_test.go`
- **AC11** – `fuku logs` without an instance prints `Error: <cause>` on stderr and exits 1. e2e `Test_Logs_NoInstance`
- **AC12** – The built binary passes the e2e suite, which imports nothing from `internal/`. `e2e/*_test.go`
- **AC13** – The ring import rules hold. No test proves it. The `depguard` audit checks it

## Assumptions

- `depguard` can express every import rule, so no Go test repeats them
- The JetBrains plugin reads `GET /api/v1/*` and `/tmp/fuku-<fingerprint>.sock`. Those are the compatibility surface

## Contracts

- Rings: five rings. An inner ring never imports an outer one. See [ARCHITECTURE.md](../../../ARCHITECTURE.md#rings)
- Participants: `Consumer`, `Producer` and one `Command`. See [lifecycle](../../../internal/bootstrap/lifecycle/README.md)
- Named subscriptions: a subscription carries a `Name` for its diagnostics. The bus does not refuse an empty one. See [bus](../../../internal/platform/bus/README.md)
- `Required`: a critical publish fails while a required matching queue is full. An optional queue drops
- `Types`: a subscription receives only the listed types. `nil` means every type
- Criticality: `MessageType.Critical()` reads one fixed table. See [contracts](../../../internal/contracts/README.md)
- IDs: packages pass a service by `ID`, not `Name`. See [CLAUDE.md](../../../CLAUDE.md#service-identifier-convention)

## Decisions

- Built one `fx.App` and one bus per invocation, so no state outlives a command
- Declared each interface in its consumer and bound it in `bootstrap/modules`, so no core package imports an adapter
- Handed the coordinator ordered participant slices, because dig shuffles a value group on purpose
- Made criticality a property of the message type, so no publisher picks a delivery class per call
- Kept raw child output off the bus. Streams write to `process.LogSink`, bound to `logs.Hub`, which keeps the history
- Read the registry snapshot in place under the store's read lock, so no frontend copies it
- Let no `Read` callback keep a pointer past its return, call `Control` or publish, so the lock never waits on the bus
- Kept `Tier.ID` for upcoming work. `profiles` assigns it and no code acts on it yet
- Kept the wire values `signal`, `preflight_kill`, `resource_sample` and `api_request`, because log clients read them
- Reserved a queue slot with check-then-send under the publish lock, because only the publisher fills a queue
- Routed every service action through `Control`. Only `Control` publishes a service command. The runtime runs it on the admission's token
- Listed the socket server before the runtime, so it stops after the runtime's last events
- Drained the consumers in two fixed passes. A publish back to an earlier consumer in the second pass is missed
- Set `fx.StopTimeout` to the default plus one `process.ShutdownTimeout` per service, so every child can stop gracefully
