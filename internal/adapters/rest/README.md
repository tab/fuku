# adapters/rest

The HTTP API of a running instance. Read routes answer from the registry snapshot.
The three service actions go through `services.Control`. This package decides nothing about services.

The response shapes are the contract in [`spec/openapi.yaml`](../../../spec/openapi.yaml).
The drift check in CI fails when this package moves and the spec does not.

## Lifetime

The server is a producer of the run composition. It is added last, unless the API is off: `server.listen` is `""` or `none`, or `FUKU_API_DISABLED=1`.

- `Start` binds the address. When the port is taken, it tries the next nine. A bind failure is logged, and the run continues without the API
- `Start` publishes `APIStarted` with the bound address. The registry puts it into `Snapshot.API`
- `Stop` shuts the server down and publishes `APIStopped`

## Routes

`guardMiddleware` wraps every route, the probes included, and runs before the token check.
It answers `403` `forbidden` when the `Host` is not loopback, the request target is absolute-form, or the request carries an `Origin` header, even an empty one.
A browser sends `Origin` on a cross-origin `fetch` and on every `POST`. A web page can neither read a response nor trigger an action.

The token is optional. Without `server.auth.token`, every route is open to a loopback caller.
`GET /api/v1/live` and `GET /api/v1/ready` never need the token.
The single-instance guard takes an exclusive `flock` on the project's lock file. The socket only names the owner in the refusal.
`ready` answers `503` until the profile is resolved.

With a token, every other route needs `Authorization: Bearer <server.auth.token>`:

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

Every response from a route above is JSON. An error is `{"error": "<text>"}`. With a token set, a missing or wrong one is `401` `unauthorized`.
With a token set, an `OPTIONS` or a wrong method needs the token too. With it, the mux answers a plain-text `405` on an authenticated route and a plain-text `404` on a probe.
An unmatched path gets the mux's plain-text `404`. Under `/api/v1/` it needs the token first.

## Changing it

- a new route reads the snapshot or calls a typed `Control` method. No rule about services is written here
- a response shape changes in `serializer.go` and in `spec/openapi.yaml` together
- the codes and texts above are the public contract. Map a new rejection onto an existing row before adding one
- the guard wraps the whole mux, inside `telemetryMiddleware`, so it covers the probes and a rejection is still published
- the guard and `server.listen` validation accept the same loopback hosts. A valid `listen` stays reachable
- an empty token mounts the routes without `authMiddleware`. A set token guards every route but the probes
