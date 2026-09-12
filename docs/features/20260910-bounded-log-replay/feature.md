# Bounded log replay

## Goal

Let callers limit buffered log replay, omit the interactive log banner and exit after the replay completes.
This gives scripts and AI agents a bounded way to inspect logs without the interactive banner or changes to the default
streaming workflow.

## Context

`fuku logs [service...]` connects to the selected profile socket, replays the configured log history and then follows new
output until the client stops.
This works for an interactive terminal but not for a bounded agent workflow.

The global `--no-ui` flag is accepted by the logs command today, but the command still renders the logs panel and the
`ctrl+c exit` footer.
An agent does not need this presentation around a bounded log read.

An agent that needs to check logs must currently start a follower, wait for enough output and stop the client itself.
A timeout cannot prove that the full buffered replay arrived and piping the stream through another command does not make
the source process finish cleanly.

The relay already owns a bounded history buffer and applies service subscriptions before replay.
The missing controls are a replay line limit and a server-driven way to close the connection after replay.

The client can connect to a fuku process that was started with an older binary.
Older servers ignore unknown subscription fields, so the server must confirm that it accepted the bounded-read options.

## Scope

### In

- Add `--tail <n>` to `fuku logs`
- Add `--no-follow` to `fuku logs`
- Make the existing global `--no-ui` flag hide the logs panel and footer
- Carry `--tail` and `--no-follow` in the relay subscription request
- Echo accepted `tail` and `noFollow` values in the relay status response
- Fail clearly when the running server does not confirm requested bounded-read options
- Validate provided tail values in both the CLI and relay server
- Apply the tail limit to matching buffered messages before they are sent to the client
- Close a no-follow client after its buffered replay has been written
- Keep the existing profile and service filters
- Accept supported flags before, between or after service names
- Add focused CLI, relay, log-screen and end-to-end tests
- Document the new flags and the logs use of `--no-ui` in the in-binary CLI help, README, CLI docs and log-streaming docs

### Out

- REST API log endpoints
- OpenAPI changes
- `--since` or timestamp-based filtering
- New log storage or persistence
- Log search, summaries or aggregation
- Agent plugin, skill or helper changes
- Changes to log history configuration
- Application-version comparison or general relay capability negotiation
- A separate replay-complete relay message
- Short flag aliases or bare `tail` and `no-follow` command forms
- A new quiet mode that suppresses log lines, command diagnostics or errors

## Expected behavior

### Default flow

Existing commands keep their current behavior:

```bash
fuku logs
fuku logs api
fuku logs api --profile core
```

They replay the configured buffered history and continue following new output until the client stops.
They keep rendering the logs panel and footer unless `--no-ui` is provided.

Flag position does not change the command behavior.
User-facing examples put service names immediately after `fuku logs` so service names and flags are easy to distinguish.

### Limited replay

`--tail <n>` limits only the buffered replay:

```bash
fuku logs api --tail 100
```

The command replays at most the newest 100 matching buffered messages, then continues following new output.
The limit is applied after service filtering, across the selected log stream.

When fewer than `n` matching messages are buffered, every matching message is replayed.
When `--tail` is omitted, fuku replays the full configured history as it does today.

An explicitly provided tail value must be greater than zero.

### Replay and exit

`--no-follow` closes the log connection after buffered replay:

```bash
fuku logs api --no-follow
```

The command prints the matching buffered messages and exits with code 0.
It also exits successfully when no matching messages are buffered.

The server decides when replay is complete and closes the client stream.
The client must not use a fixed delay or inactivity timeout to guess when replay has finished.

### Banner-free output

`--no-ui` hides the interactive logs panel and footer:

```bash
fuku logs api --no-ui
```

The command still prints matching log lines and continues following them.
It keeps the configured log formatting and normal error reporting.
The flag changes presentation only and does not imply a tail limit or disable following.

### Bounded agent read

Scripts and AI agents combine the new flags with `--no-ui`:

```bash
fuku logs api --profile core --no-ui --tail 100 --no-follow
```

The command omits the logs panel and footer, prints at most 100 matching buffered messages and exits after replay.
No service argument means all buffered messages, including fuku orchestration messages.

### Running server compatibility

The server echoes the accepted `tail` and `noFollow` values in its status message.
When bounded options are requested, the client requires this status acknowledgement before it handles any relay log
message.
It rejects a missing, out-of-order or different acknowledgement.

An older server omits these fields because it does not support them.
When either bounded-read option was requested, the new client closes the connection, prints a clear compatibility error and
exits with code 1 instead of starting an unbounded follower.
The error tells the caller to restart the running profile with the current fuku version.
The rejected status is not passed to the log handler, so the command does not render its banner.

Commands without `--tail` or `--no-follow` continue to work with older servers.
Compatibility is based on the echoed values, not the fuku application version.

### Invalid input

These commands fail before connecting to a socket:

```bash
fuku logs --tail 0
fuku logs --tail -1
```

The error states that `--tail` must be greater than zero.

The relay server also rejects a raw subscription request with an explicit zero or negative `tail` before hub registration.
This protects the wire contract even when the request does not come from the fuku CLI.
The server logs the validation error and closes the raw connection without sending a status message.

Bare words remain service names.
For example, `fuku logs tail` requests logs for a service named `tail`; it is not an alias for `--tail`.

### Acceptance criteria

- **AC1** – `fuku logs` without the new flags keeps replaying the configured history and following new messages
- **AC2** – `fuku logs --tail <n>` replays at most the newest `n` messages after service filtering and then follows new messages
- **AC3** – `fuku logs --no-follow` replays all matching buffered messages and exits without waiting for new output
- **AC4** – combining `--tail <n>` and `--no-follow` prints at most `n` matching buffered messages and exits
- **AC5** – `--tail 0` and negative tail values return a non-zero exit code before any socket connection is attempted
- **AC6** – a tail value larger than the matching history returns all available matching messages without error
- **AC7** – a no-follow read with no matching history exits successfully without printing a log message
- **AC8** – existing `--profile` and service filtering work with both new flags
- **AC9** – no service filter includes service output and fuku orchestration messages in their buffered order
- **AC10** – during normal no-follow completion, the server closes the stream only after every selected replay message has been written
- **AC11** – existing relay clients that omit the new subscription fields keep their current follow behavior
- **AC12** – `fuku logs --no-ui` omits the logs panel and footer but keeps printing matching log lines
- **AC13** – `--no-ui` does not change replay limits or follow behavior
- **AC14** – supported flags before, between or after service names produce the same parsed log options
- **AC15** – the in-binary CLI help, README, CLI docs and log-streaming docs describe `--tail`, `--no-follow` and the
  logs use of `--no-ui`, with service names immediately after `fuku logs` and options after the service names
- **AC16** – unit tests cover each new behavior and compatibility failure, while end-to-end tests cover the existing
  follow flow and the combined bounded read
- **AC17** – the status message echoes the exact accepted `tail` and `noFollow` values
- **AC18** – a client that requests either bounded-read option fails with exit code 1 before printing relay logs when the
  acknowledgement is missing, out of order or different from the request
- **AC19** – the relay server rejects an explicit zero or negative `tail` before hub registration

## Assumptions

- The existing relay history buffer remains the source of replayed messages
- Service names remain subscription filters only and are not changed by this feature
- History order remains oldest to newest after the tail limit is applied
- The configured history size remains the upper bound when `--tail` is omitted
- The existing global `--no-ui` value is available to the logs screen
- The running server remains available until a normal no-follow replay completes
- This first version does not distinguish a normal no-follow EOF from an unexpected server shutdown during replay

## Contracts

`tail` is an optional integer in the relay subscription request and status response.
An omitted value keeps the current full-history replay.
A provided value is a positive whole number and limits only buffered messages.
It does not limit messages received later while following.
The wire representation must preserve the difference between an omitted value and an explicit zero.

`noFollow` is optional in the relay subscription request and status response.
An omitted or false value keeps the current follow behavior.
A true value asks the server to close the stream after replay.

The server copies the accepted values into the status response before it registers the client with the hub.
When the client requested `tail` or `noFollow`, it requires the status response to echo the exact requested values.
A missing, out-of-order or different acknowledgement means the running server did not honor the request.
The client returns a compatibility error before handling relay log messages.

Existing clients can ignore the new status fields.
The JetBrains log client already parses relay messages with unknown fields enabled.

The two options are independent.
`--tail` does not imply `--no-follow` and `--no-follow` does not apply an implicit tail limit.

`--no-ui` controls log-screen presentation only.
It hides the connection panel and footer without changing which log messages are selected or how long the client follows.

Command parsing keeps Cobra's existing interspersed flag behavior.
The documented argument order is a readability convention, not a parser restriction.

The server applies service filtering before the tail limit.
For example, `fuku logs api worker --tail 2` returns the newest two buffered messages across `api` and `worker`, not two
messages per service.

During normal operation, the server owns replay completion.
Closing the stream after its send channel drains is the completion signal, so no timeout or new relay message type is
required for this first version.

## Decisions

- Add both flags because they control separate behavior: replay size and connection lifetime
- Keep the existing streaming default to avoid breaking interactive use
- Reuse the global `--no-ui` flag instead of adding a logs-specific quiet flag
- Hide only the logs panel and footer because log messages and errors are still useful in plain-output mode
- Use only long flag names because bare words are service names and short aliases are not needed for the first version
- Accept flags in any supported position but use service-first ordering in documentation
- Apply one tail limit across the filtered stream because the relay preserves one ordered history
- Let the server close no-follow clients because only the server knows when replay is complete
- Echo accepted options in the existing status message so older servers fail fast without version comparison
- Preserve tail presence on the wire and validate it again at the server boundary
- Defer `--since` because tail already bounds agent output and time filtering would require timestamps and more protocol logic
- Keep logs on the Unix socket because a REST log endpoint would duplicate the relay without adding value to this feature
