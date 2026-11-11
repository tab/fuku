# adapters/rest

The HTTP API of a running instance, the server and the client. Read routes answer from the registry snapshot.
The three service actions go through `services.Control`. This package decides nothing about services.
The response shapes are the contract in [`spec/openapi.yaml`](../../../spec/openapi.yaml).
The drift check in CI fails when this package moves and the spec does not.

## Lifetime

The server is the last producer of the run composition.
It is left out when the API is off: `server.listen` is `""` or `none`, or `FUKU_API_DISABLED=1`.

- `Start` binds the address. When the port is taken, it tries the next nine. A bind failure is logged, and the run continues without the API
- `Start` then publishes `APIStarted` with the bound address. The registry puts it into `Snapshot.API`
- `Stop` shuts the server down and publishes `APIStopped`

## Routes

`guardMiddleware` wraps every route, the probes included, and runs before the token check.
It answers `403` `forbidden` when the `Host` is not loopback, the request target is absolute-form, or the request carries an `Origin` header, even an empty one.
A browser sends `Origin` on a cross-origin `fetch` and on every `POST`, so a web page can neither read a response nor trigger an action.

The token is optional. Without `server.auth.token`, every route is open to a loopback caller.
`GET /api/v1/live` and `GET /api/v1/ready` never need the token. `ready` answers `503` until the profile is resolved.

With a token, every other route needs `Authorization: Bearer <server.auth.token>`:

| Route                                | Answer                                                         |
| ------------------------------------ | -------------------------------------------------------------- |
| `GET /api/v1/status`                 | version, instance, project, profile, phase, uptime, the counts |
| `GET /api/v1/services`               | every service in tier order, then by name                      |
| `GET /api/v1/services/{id}`          | one service or `404`                                           |
| `POST /api/v1/services/{id}/start`   | `202` with the predicted status `starting`, or a rejection     |
| `POST /api/v1/services/{id}/stop`    | `202` with `stopping`, or a rejection                          |
| `POST /api/v1/services/{id}/restart` | `202` with `restarting`, or a rejection                        |

`202 Accepted` means admitted, not completed. The next `GET` shows what happened.

The optional JSON body `{"wait": <seconds>}` holds the request until the action ends: `Control.Wait`, then `Registry.WaitSettled` from `Admission.At`.
The request then answers `200` with the service. It answers `202` when the wait ends first or the server shuts down.
An empty body answers `202` at once.

- the handler never reads the clock itself. A read before admission would count the outcome of the action that held the token just before
- known limit: a child that exits on its own right after admission ends the wait early, with the stop or failure its exit reported
- a body that is not one JSON object with only `wait`, or larger than 1 KiB, is `400` `body must be a JSON object with an optional wait`
- a `wait` outside 1 to 60 is `400` `wait must be whole seconds from 1 to 60`. Neither `400` admits anything

A rejection maps the control error:

| Control error                                    | Status | Text                                |
| ------------------------------------------------ | ------ | ----------------------------------- |
| `ErrServiceNotFound`                             | `404`  | `service not found`                 |
| `ErrNotAccepting`, `ErrBusClosed`                | `409`  | `instance is not accepting actions` |
| `ErrActionNotAllowed`, `ErrServiceBusy` on start | `409`  | `service cannot be started`         |
| the same on stop                                 | `409`  | `service is not running`            |
| the same on restart                              | `409`  | `service cannot be restarted`       |
| anything else (`ErrBusOverloaded`)               | `500`  | `instance is overloaded`            |

Every response from a route above is JSON. An error is `{"error": "<text>"}`.
With a token set, a missing or wrong one is `401` `unauthorized`, also on an `OPTIONS` or a wrong method.
With the token, the mux answers a plain-text `405` on an authenticated route and a plain-text `404` on a probe.
An unmatched path gets the mux's plain-text `404`. Under `/api/v1/` it needs the token first.

## The client

`Client` calls the API of another running fuku. `mcp` uses it through its own `Client` interface, bound in `bootstrap/modules/mcp.go`.
It decodes into the serializers of this package and returns `model` values.

- `NewClient` accepts only a loopback address. The host `ip6-localhost`, which the config accepts, maps to `::1` before the URL is built
- `NewHTTPClient` never follows a redirect, so the token never leaves the address it was given for
- the token goes as `Authorization: Bearer` only when it is set. A caller that has not checked the address yet passes no token and calls `Live` first
- `Live` fails with `ErrClientNotFuku` when the answer names another product
- `Services` turns each `uptime` back into `Process.StartedAt`
- `Act` sends `{"wait": <seconds>}`. A `202` returns settled false. The request may run for the wait plus 5 seconds.
  Every other request gets 3 seconds
- an answer maps onto the `contracts` sentinels: no connection is `ErrAPIUnreachable`, `401` is `ErrAPIUnauthorized`, `404` and `409` are `ErrAPIRejected` with the REST text, `500` on a service route is `ErrAPIOverloaded`.
  Any other status is `ErrClientFailed`
- a transport error drops the request URL, because the URL carries the service ID. Only an address that cannot form a URL shows it.
  A validated `server.listen` or an announced listener address never does

## Changing it

- a new route reads the snapshot or calls a typed `Control` method. No rule about services is written here
- a response shape changes in `serializer.go` and in `spec/openapi.yaml` together
- the codes and texts above are the public contract. Map a new rejection onto an existing row before adding one
- the guard wraps the whole mux, inside `telemetryMiddleware`, so it covers the probes and a rejection is still published
- the guard and `server.listen` validation accept the same loopback hosts. A valid `listen` stays reachable
- an empty token mounts the routes without `authMiddleware`. A set token guards every route but the probes
- the client reads the same serializers the server writes, so a shape change reaches both
