# adapters/mcp

`fuku mcp` is a stdio MCP server. It lets an AI agent control fuku through typed tools. It is a frontend.
It decides no health, readiness or retry rule. It finds the running instance, calls it and presents the result.

## How it works

The server is the one `Command` participant of `bootstrap/modules/mcp.go`.
`Run` enters the project root, so the env files load from the project the agent works in.
Then it opens the console, shows the banner and serves the SDK's stdio transport until stdin closes or the run context ends.
Stdout belongs to the protocol. The composition gives the package no log output, so nothing else writes to stdout.
Stderr is the console a person reads, or the host's MCP log.

### The console

The server decides what the console says and passes records to `Console`: one banner, then one call record per `tools/call`.
`Plain` and `tui.MCPConsole` render the records.

- a failed `run`, `stop` or `doctor` shows a fixed label, such as `fuku run failed · exit 1`, because its error carries the command's output
- a call the SDK rejects for its arguments shows `invalid arguments`, because the SDK's text quotes the rejected value.
  The SDK marks this failure only by the `validating "arguments"` prefix of the error
- every other failed call shows the first line of its error, without control characters

`Plain` writes when stderr is a pipe, which is the case under Claude Code and Codex, or with `--no-ui`.
It writes the exact lines the host logs keep, with no escape sequences. It serialises the writes, because calls run concurrently.
On a terminal, `tui.MCPConsole` keeps the banner in place and prints each call above it.
It reads no input and sends no terminal query, because stdin and stdout carry the protocol.
The theme is fixed dark: stdin is usually the same terminal, so the reply to the OSC 11 background query would corrupt the session.

### Token redaction

`redactCalls` is a receiving middleware around the call log.
It replaces the API token with `[redacted]`, raw and JSON-escaped, in every result, error and call line.
It covers the token of the config the call names and the token of its project's default lookup. A JSON-RPC error keeps its code.
A structured content that no longer parses after redaction is dropped. The redaction keeps the agent on the tools.
It is not a security boundary, because the agent runs as the user and can read the config.
It misses the token of a config with a YAML syntax error, which yields no token.
It also misses a file the call does not name and the default lookup does not pick, such as the `--config` file of an instance.

### The project of a call

The composition loads no project. The server starts without a running fuku and without a valid `fuku.yaml`. Each tool call resolves its project again:

- the root is `CLAUDE_PROJECT_DIR` when set, the working directory otherwise
- the optional `config` argument has the meaning of `--config`. Its directory is the project. A relative path resolves under the root
- every load goes through `Loader`, bound to `config.LoadDir`, so `CLAUDE_PROJECT_DIR` and the working directory cannot name two projects
- `run`, `stop` and `doctor` pass `--config` only when the default lookup reads another file.
  An explicit path skips `fuku.override.yaml`, so passing it would drop the override and the token it sets
- the project directory gives the fingerprint. The fingerprint finds the log socket of the running instance
- `status`, `service` and `config` load the config fresh, because the agent may edit it during the session
- `logs` and `doctor` need no config, so they work while the agent fixes a broken `fuku.yaml`

`status` and `service` read the status frame before the config, so no running fuku is reported before an invalid config.
For `logs`, `stop` and `service`, no instance is a tool error that names the project and says to call `run`.
The error lists the sorted profile names, such as `call run with one of: core, default`, or says `call run first` when the config does not load.
`status` answers instead, with `running: false`, the profile names and the same hint as `note`.
When the API is off or unreachable, or the config does not load, `status` reports the names from the status frame with state `unknown` and a `note`.

### Finding the API

`status` and `service` call the REST API through `Client`.

- the status frame announces the API address. The socket belongs to the project, so the address is trusted
- an empty `api` field means that the API is off, or that the instance is older.
  The server then tries `server.listen` of the loaded config, without the token.
  It accepts the address only when `GET /api/v1/live` answers `product: fuku` with this project's fingerprint
- the token goes only to the announced address, or to the candidate after its liveness check. The client refuses an address that is not loopback
- `401` means that the token of the loaded config is missing or wrong. The error says that an explicit config skips `fuku.override.yaml`

### Waits and failures

- `logs` stops its read after 10 seconds. A read cut short returns its lines with `truncated: true`
- `run` executes `fuku run <profile> --detached`. After exit 0 it reads the state once, so the agent needs no `status` call.
  After 50 seconds the server kills the waiting parent and answers `inProgress: true` with `exitCode: -1`.
  The detached child is in its own session and ignores `SIGPIPE`, so it keeps starting
- `stop` drops the output on success, because it carries the project's log lines. A failure returns the last 20 lines
- `service` asks the API to wait 45 seconds for the service to settle, and the whole call ends within 55 seconds.
  A wait that ended first answers the current state with `inProgress: true`. A settled `failed` service carries its last 20 log lines

## Using it

The composition binds `Instance` to `logsocket.NewClient`, `Client` to `rest.NewClient`, `Loader` to `config.LoadDir` and `Executor` to `Binary`.
`Binary` runs the binary the server runs from. The server connects to one instance per call, because the project can change from call to call.
`mcp` never imports `adapters/rest`. It matches the API errors with `errors.Is` on the `contracts` sentinels and adds what the agent can do next.

## Changing it

- tool names, parameters and output schemas are the agent contract. The server `instructions` can change freely
- never write to stdout outside the transport. A child the server runs gets no stdin, because stdin carries the protocol
- never return the API token in a result, an error or a call line. `api` in the `config` result never carries it
- never send the token to an address before it is known to be this project's API
- tools take and return service names. Never name a service ID in a result or an error. An API error drops the request URL, which carries the ID
- `instructions` is one raw string literal, one rule per line, so a diff shows one rule. Keep backticks out of the rules
- the first 512 characters of `instructions` must stand alone, as Codex asks. Some hosts cut the rest.
  They hold the rule to use only the tools, the config-read rule and the `doctor` rule. The tool map and the workflow rules come after them.
  Claude Code reads up to 4,096 characters at session start, before it loads any deferred tool description
- the control tool descriptions repeat the only-the-tools rule, because a host may weigh a tool description above the instructions
- a schema `default` crashes the SDK on `"arguments": null`, so the handler applies the default instead
- an enum or a bound that inference cannot express is a named type with an entry in `schemas`. An input type embeds `target`, so every tool takes `config`
- the plain lines are what the host logs keep. Change their text only on purpose, and keep `Plain` and `tui.MCPConsole` in the same columns
- a call line names no argument other than the `service` name and action, and no result body
- the run timeout kills the `fuku run -d` parent with `SIGKILL`. A `SIGTERM` would make the parent stop the detached child
