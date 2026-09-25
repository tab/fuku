# adapters/telemetry

The Sentry side: one client, a metrics collector and a tracer. The collector and the tracer are bus subscribers.
No other package calls Sentry.

## The client

`Start` sets up the SDK when telemetry is enabled. Otherwise it does nothing. `NewClient` only keeps the options.

- enabled when `FUKU_TELEMETRY_DISABLED` is not `1` and a DSN exists: `SENTRY_DSN`, else the DSN built into the release binary
- the user is an anonymous UUID in `$UserConfigDir/fuku/telemetry.id`
- `stripPII` clears the server name and every user field but the ID
- traces are sampled at 10%

The coordinator calls `Start` after the guard and `Stop` after the command returns. It calls `Recover` on a panic.
`Stop` unbinds the SDK, flushes it and closes the transport. Before `Start` and after `Stop`, `Recover` sends nothing.
A flush that timed out leaves the transport to the process exit. Its `Close` would wait out every send on a dead network.

## The collector

An optional subscription named `metrics`. It is a consumer only when telemetry is enabled. One event, one measurement:

| Event                                                  | Measurement                                                 |
| ------------------------------------------------------ | ----------------------------------------------------------- |
| `CommandStarted`                                       | `app_run`; `command` and `profile` tags                     |
| `ProfileResolved`                                      | `service_count`, `tier_count`, `discovery_duration`         |
| `PreflightComplete`                                    | `preflight_killed`, `preflight_duration`                    |
| `TierReady`                                            | `tier_startup_duration`                                     |
| `ReadinessComplete`                                    | `readiness_duration` by type                                |
| `ServiceReady`                                         | `service_startup_duration`                                  |
| `ServiceFailed`, `ServiceRestarting`, `WatchTriggered` | `service_failed`, `service_restart`, `watch_restart`        |
| `ServiceStopped{Unexpected}`                           | `unexpected_exit`                                           |
| `PhaseChanged{running}`, `PhaseChanged{stopped}`       | `startup_duration`, `shutdown_duration`                     |
| `ResourceSampled`                                      | `fuku_cpu`, `fuku_memory`                                   |
| `APIStarted`, `APIStopped`                             | `api_enabled` 1 or 0                                        |
| `APIRequested`                                         | `api_requests`, `api_request_duration`, `api_auth_failures` |

The API path is normalized. The service ID becomes `:id`. So the tag set stays small.

## The tracer

An optional subscription named `tracer`. It is a consumer only when telemetry is enabled.

- `CommandStarted{run}` opens one transaction, `fuku run`
- `discovery`, `preflight`, one `tier_startup` per tier and `shutdown` are child spans
- `watch_restart`, `service_stop` and `service_restart` are zero-length marks
- `PhaseChanged{stopped}` finishes the transaction as ok. `Drain` finishes one still open as cancelled, before the coordinator flushes

## Changing it

- a new measurement is a new event on the bus first. The collector reads it. No `sentry.NewMeter` call anywhere else
- a metric name is a constant in `measurements.go`. A span op is a constant in `spans.go`
- keep the attributes bounded. No service names, no raw paths, no IDs
