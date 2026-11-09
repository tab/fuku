# adapters/mcp

`fuku mcp`: a stdio MCP server that lets an AI agent control fuku through typed tools.
It is a frontend. It decides no health, readiness or retry rule.
It finds the running instance, calls it and presents the result.

## How it works

The server is the one `Command` participant of `bootstrap/modules/mcp.go`.
`Run` serves the SDK's stdio transport until stdin closes or the run context ends.
Stdout belongs to the protocol. The composition gives the package no log output, so nothing else writes to it.
Stderr is the console a person reads, or the host's MCP log:

- `Run` writes a banner first: the protocol version, the project and its fingerprint, the instance state from the status frame, and the tools
- a receiving middleware around `tools/call` writes one line per call: the tool, the name and action of a `service` call,
  the outcome and the duration
- the outcome of a `service` call is its settled state, or its current state with `· in progress`
- a failed `run`, `stop` or `doctor` shows a fixed label with the exit code, such as `fuku run failed · exit 1`.
  Their errors carry the command's output, the profile and the config path, so the line never shows them.
  A call the SDK rejects for its arguments shows `invalid arguments`, because the SDK's text quotes the rejected value.
  The SDK marks it only by the `validating "arguments"` prefix of the error.
  Every other failed call shows the first line of its error, without control characters
- the writes are serialised, because calls run concurrently
- every write goes through `lipgloss.Fprint`, which strips the styles when stderr is no terminal.
  A plain `fmt.Fprint` of a rendered style keeps the escape codes
- the theme is fixed dark. Detecting the background sends an OSC 11 query to stdout and reads the answer from stdin.
  Both carry the protocol, so the query would corrupt the session

The composition loads no project. The server starts without a running fuku and without a valid `fuku.yaml`.
`Run` enters the root first, so the default config lookup reads the project the agent works in.
Each tool call resolves its project again:

- the root is `CLAUDE_PROJECT_DIR` when set, the working directory otherwise
- the optional `config` argument has the meaning of `--config`. Its directory is the project: a relative path resolves under the root, an absolute path or a `../` path anywhere
- a `config` that names the root file the default lookup picks uses the default lookup, as if omitted.
  An explicit path skips `fuku.override.yaml`, so passing it would drop the override and the token it sets
- the lookup order is fuku's: `fuku.yaml`, then `fuku.yml`. The lookup skips a missing name and stops at the first that exists.
  A named root file maps to the default lookup only when that first file is the one it names and reads without error.
  Otherwise it stays explicit, exactly like `--config`, so a missing `fuku.yaml` fails as the CLI does
- the project directory gives the fingerprint, and the fingerprint finds the log socket of the running instance
- `status` and `service` load the config fresh through `Loader`, because the agent may edit it during the session.
  It reads the status frame first, so no running fuku is reported before an invalid config.
  An invalid config then fails `service` with the config's message.
  `status` still answers: the names from the status frame with state `unknown` and the first line of the config error as `note`
- `logs` and `doctor` need no config, so they work while the agent fixes a broken `fuku.yaml`

A tool that needs the running instance reads its status frame through `Client`.
No instance is a tool error (`isError`) that names the project and says to call `run`. The session keeps serving.
The error loads the config and lists its profile names, sorted, such as `call run with one of: core, default`.
A config that does not load leaves the plain `call run first`.

### Finding the API

`status` and `service` call the REST API through `API`. `APIDialer` returns one for an address and a token:

- the status frame announces the API address. The socket belongs to the project, so the address is trusted
- an empty `api` field means the API is off, or the instance is older and does not send the field.
  The server then tries `server.listen` of the loaded config and accepts it only when `GET /api/v1/live` answers `product: fuku` with this project's fingerprint
- only then is the API off or unreachable. `status` then reports the names from the status frame with state `unknown` and a note.
  `service` fails and says to restart the whole profile with `stop` and `run`
- the `server.listen` candidate is dialed without the token. The token of the loaded config goes only to the address the frame announces,
  or to the candidate after its liveness check, and only when the config sets one
- the client refuses an address that is not loopback, and maps `ip6-localhost` to `::1`
- `401` means the API rejected the token of the loaded config: it is missing or wrong.
  The error says an explicit config skips `fuku.override.yaml`, where the token often lives

### Tool results

Tools take and return service names, never IDs.

- `status`: `{profile, phase, mode, services: [{name, state, pid, uptime, watching}], note?}`. `uptime` is in seconds.
  `mode` is the run mode of the status frame: `tui`, `foreground` or `detached`, and `unknown` for an older fuku that sends none
  `watching` is the API's flag: file watching is active, so a file change restarts the service on its own
  `note` says why the states are `unknown`: no reachable API, or a config that does not load
- `logs(services?, tail?)`: `{lines: [{service, message}], truncated?}`. `tail` is 100 by default and 1000 at most.
  The read stops after 10 seconds. A read cut short returns its lines with `truncated: true`.
  An unknown name is a tool error that lists the names of the running profile.
  The read is the bounded `fuku logs --tail N --no-follow` read through `Client`
- `doctor(profile?)`: the `fuku doctor --json` report, run through `Executor` in the project directory.
  A failing report is a result, not a tool error
- `run(profile)`: `{exitCode, summary, inProgress?, services?}`. It executes `fuku run <profile> --detached` in the project directory.
  `summary` is its output, ending with the `API <address>` line when the API is on.
  After exit 0 the server reads the state once, the way `status` does, so the agent needs no `status` call.
  `services` has the shape of the `status` services. A read that fails leaves it out, and the summary and `exitCode: 0` stand.
  A non-zero exit, such as a fuku that already runs, is a tool error with the reason fuku printed.
  After 50 seconds the server kills the waiting parent and answers `inProgress: true` with a note to call `status`.
  `inProgress` carries the meaning. `exitCode` is then `-1`, because the killed parent has no exit code. No state is read then.
  The detached child is in its own session and ignores `SIGPIPE`, so it keeps starting
- `stop()`: `{exitCode, summary}`. It reads the running profile from the status frame and executes `fuku stop <profile>`.
  On exit 0 the summary is `stopped profile <profile>`. The output is dropped, because it carries the project's log lines.
  A non-zero exit is a tool error with the last 20 lines of the output and the error
- `service(name, action)`: `{name, state, pid, watching, inProgress?, lines?}`. It maps the name to its ID through `GET /api/v1/services`.
  An unknown name is a tool error that lists the names of the running profile.
  It calls `API.Act`, which posts the action with the body `{"wait":45}` and allows the request the wait plus 5 seconds.
  The whole call ends within 55 seconds. The log read of a failed service gets the time that remains.
  A settled `failed` service carries its last 20 log lines. A wait that ended first answers the current state with `inProgress: true`.
  A `404` or `409` is a tool error with the REST text. A `500` says the instance is overloaded

`tools/list` returns the six tools in the SDK's fixed name order: `doctor`, `logs`, `run`, `service`, `status`, `stop`.
`status`, `logs` and `doctor` are read-only. `stop` is destructive. `run` and `service` are neither read-only nor destructive.
The `run` description says it returns every service's state, and to call `status` only when `inProgress` is true or `services` is missing.
The `service` description says it returns the settled state, so no `status` call is needed after.
It also says that `inProgress: true` means the action outlived the call, and to follow it with `status`.
The `status` and `service` descriptions say that a watching service restarts itself after a file change.
After a code edit the agent calls `status` until the watching service whose files it edited shows a new PID or `failed`, then tests it. It restarts only a service that is not watching, so it does not race the watch restart into a `409`.
The `status` description says what `mode` allows: `detached` is a background fuku, so `stop` then `run` applies a config change.
`tui`, `foreground` or `unknown` is the developer's own terminal session: the agent does not stop it to apply a config change and tells the developer the change needs their restart.
An explicit request of the user to stop wins in every mode.
The `stop` description says to leave fuku running after the task and to stop only when the user asks, or to apply a config change under mode `detached`.
The instructions carry the short form: stop only when the user asks.
The `doctor` description says to use it after a failed `run` or a config edit, and that it is not needed before a plain `run`.
The `config` description says to omit it unless the user names a file other than `fuku.yaml`.
The `profile` description of `run` says to use `default` unless the user names another profile.

## Using it

The composition binds `Dialer` to a function that returns `logsocket.NewClient` for an identity,
`APIDialer` to `rest.NewClient` over `rest.NewHTTPClient`, `Loader` to `config.LoadPath` and `Executor` to `Binary`,
which runs the binary the server runs from.
The server dials one client per call, because the project can change from call to call.
`mcp` never imports `adapters/rest`. It matches the API errors with `errors.Is` on the `contracts` sentinels and adds what the agent can do next.

## Changing it

- tool names, parameters and output schemas are the agent contract. The server `instructions` can change freely
- the first 512 characters of `instructions` must stand alone. Some hosts cut the rest.
  They keep the rule never to read `fuku.yaml` or `fuku.override.yaml` whole, because the files may hold secrets.
  They keep the rule to call `doctor` only after a failed `run` or a config edit, because the skill covers config editing only
- an input type embeds `target`, so every tool takes `config`
- an enum or a bound that inference cannot express is a named type with an entry in `schemas`
- never write to stdout outside the transport, and never return the API token in a result, an error or a log
- a call line names no argument other than the `service` name and action, and no result body
- never send the token to an address before it is known to be this project's API
- a schema `default` crashes the SDK on `"arguments": null`, so the handler applies the default instead
- a child the server runs gets no stdin, because stdin carries the protocol
- the run timeout kills the `fuku run -d` parent with `SIGKILL`. A `SIGTERM` would make the parent stop the detached child
- never name a service ID in a result or an error. An API error drops the request URL, which carries the ID
