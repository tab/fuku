# adapters/rest

The HTTP API of a running instance. Read routes answer from the registry snapshot.
The three service actions go through `services.Control`. This package decides nothing about services.

The response shapes are the contract in [`spec/openapi.yaml`](../../../spec/openapi.yaml).
The drift check in CI fails when this package moves and the spec does not.

## Lifetime

The server is a producer of the run composition. It is added last, and only when `server.listen` is set.

- `Start` binds the address. When the port is taken, it tries the next nine. A bind failure is logged, and the run continues without the API
- `Start` publishes `APIStarted` with the bound address. The registry puts it into `Snapshot.API`
- `Stop` shuts the server down and publishes `APIStopped`

## Routes

`GET /api/v1/live` and `GET /api/v1/ready` need no token.
The single-instance guard takes an exclusive `flock` on the project's lock file. The socket only names the owner in the refusal.
`ready` answers `503` until the profile is resolved.

Every other route needs `Authorization: Bearer <server.auth.token>`:

| Route                                | Answer                                                         |
| ------------------------------------ | -------------------------------------------------------------- |
| `GET /api/v1/status`                 | version, instance, project, profile, phase, uptime, the counts |
| `GET /api/v1/services`               | every service in tier order, then by name                      |
| `GET /api/v1/services/{id}`          | one service or `404`                                           |
| `POST /api/v1/services/{id}/start`   | `202` with the predicted status `starting`, or a rejection     |
| `POST /api/v1/services/{id}/stop`    | `202` with `stopping`, or a rejection                          |
| `POST /api/v1/services/{id}/restart` | `202` with `restarting`, or a rejection                        |

`202 Accepted` means admitted, not completed. The next `GET` shows what happened. A rejection maps the control error:

| Control error                                    | Status | Text                                |
| ------------------------------------------------ | ------ | ----------------------------------- |
| `ErrServiceNotFound`                             | `404`  | `service not found`                 |
| `ErrNotAccepting`, `ErrBusClosed`                | `409`  | `instance is not accepting actions` |
| `ErrActionNotAllowed`, `ErrServiceBusy` on start | `409`  | `service cannot be started`         |
| the same on stop                                 | `409`  | `service is not running`            |
| the same on restart                              | `409`  | `service cannot be restarted`       |
| anything else (`ErrBusOverloaded`)               | `500`  | `instance is overloaded`            |

Every response from a route above is JSON. An error is `{"error": "<text>"}`. A missing or wrong token is `401` `unauthorized`.
An `OPTIONS` or a wrong method needs the token too. With it, the mux answers a plain-text `405` on an authenticated route and a plain-text `404` on a probe.
An unmatched path gets the mux's plain-text `404`. Under `/api/v1/` it needs the token first.

## Changing it

- a new route reads the snapshot or calls a typed `Control` method. No rule about services is written here
- a response shape changes in `serializer.go` and in `spec/openapi.yaml` together
- the codes and texts above are the public contract. Map a new rejection onto an existing row before adding one
