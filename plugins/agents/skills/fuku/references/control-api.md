# fuku control API

Read this reference for service status and start, stop or restart actions

## Resolve the connection

The API is available only while `fuku run` is active and `server.listen` is set in the effective config
The listen address must be loopback and `server.auth.token` is required

If the configured port is busy, fuku binds one of the next 9 ports
The helper probes that whole range and uses the configured token to select the matching instance
Prefer the actual `API server listening on ...` output only when this agent owns the process

Keep the token out of command output, logs and the final response
The helper loads the token from the effective config inside its own process
Never read the token with a shell command or pass its value on the command line
`FUKU_API_TOKEN` may override the config token when another local secret source is required

In a managed sandbox, `operation not permitted` from loopback HTTP can be a permission failure
Retry with approved local access before concluding that the API is down

The API uses the config loaded when `fuku run` started
A service start or restart does not reload fuku config files
Restart the full profile after changing the base config or an override

## Use the helper

Resolve this skill directory from the loaded skill path, then run:

```bash
python3 scripts/control.py discover
python3 scripts/control.py logs --tail 100 [service...]
python3 scripts/control.py status
python3 scripts/control.py services api storage
python3 scripts/control.py restart api
```

For an explicit config, pass it before the command:

```bash
python3 scripts/control.py --config fuku.failed.yaml status
```

If startup reports a different bound port, pass its loopback URL through `--base-url`

The port fallback exists for a busy port, not for a second instance of the same project
Give each project its own `server.listen` range and token so instance selection stays unambiguous
The helper accepts only loopback HTTP URLs, disables HTTP proxies and rejects redirects so the token stays on the configured local endpoint

Actions wait up to 60 seconds by default
Use `--wait-seconds <seconds>` to change the bound or `--no-wait` only when the user does not need the final state

## API contract

Read the packaged [OpenAPI contract](openapi.yaml) for paths, authentication, schemas and responses

`/ready` means the runtime store has profile data
It does not mean that all services are running
Use `/status` and `/services` for the final result

Actions are accepted only while the fuku phase is `running`

- Start is valid for `stopped` or `failed`
- Stop is valid for `running`
- Restart is valid for `running`, `stopped` or `failed`

The action response is asynchronous
Poll the service UUID until start or restart reaches `running`, stop reaches `stopped` or the wait bound expires

Every service carries `revision`, a monotonic counter that changes on every lifecycle transition
Compare it against the read taken before the action to tell whether that action took effect
It is the only exact signal: a status can be sampled at the same value on both sides of a transition, a PID can be
reused, and an unchanged `failed` state does not say whether the failure is the old one or the one just caused
Values are opaque and comparable only within one fuku instance

Refresh the service list before every action because UUIDs are created for each fuku run

`discover` loads the token internally and never returns it
It confirms the API serving this project and reports its status and services
`socket_profiles` lists this project's live log socket profiles, `other_socket_profiles` lists the ones
belonging to other directories

## Instance identity

`GET /api/v1/live` needs no token and reports `product`, `instance` and `project`

- `product` is always `fuku`, so another loopback server on the port is not mistaken for one
- `instance` is a UUID created on every start
- `project` is a fingerprint of the directory the instance serves, not the path, because the endpoint is unauthenticated

Use it to pick this project's API out of the port range before sending the token anywhere
`GET /api/v1/status` reports the same `instance` plus the absolute `project` directory, because it is authenticated

`fuku doctor <profile> --json` answers the same question without a token
`config.api` reports whether the API is configured and the port range it may bind, and `runtime.api` reports the
address this project's instance actually bound, or that the answering instances belong to other projects

The same fingerprint is sent in the status banner of the log socket, so a stream can be attributed to its project

## Common errors

| Status | Meaning                                      | Action                                      |
|--------|----------------------------------------------|---------------------------------------------|
| `401`  | Token is missing or wrong                    | Check config selection or token override    |
| `404`  | UUID is not part of the active profile       | Refresh the service list and exact name     |
| `409`  | Phase or service state does not allow action | Refresh status and wait or report the state |

Do not retry an invalid action in a loop

## Buffered logs

`GET /api/v1/logs` and `GET /api/v1/services/{id}/logs` return the relay buffer without following the live stream
`tail` bounds the read and is clamped to the buffer size, `since` accepts a duration such as `30s` or `5m`
`GET /api/v1/logs` also accepts a repeated `service` query parameter carrying service UUIDs

Each line carries `service`, `message` and the `timestamp` the relay buffered it
An invalid `tail` or `since` is rejected with HTTP 400, and an unknown service UUID with HTTP 404

The `logs` helper command wraps both endpoints, resolves names to UUIDs and bounds the response
