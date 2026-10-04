# Detached run mode

## Goal

`fuku run --detached` starts fuku in the background and returns once every service is running.
A person or an AI agent then drives it with `fuku logs`, the REST API and `fuku stop`.

## Why

`fuku run` holds the terminal until it is stopped.
A caller that wants fuku running while it does other work must keep the process attached and manage its lifetime by hand.

`fuku stop` never contacts the running fuku.
It kills processes in the service directories, so the orchestrator keeps running and records its services as failed.
A detached fuku would survive `fuku stop` as an orphan that holds the project socket and lock.

## Scope

In:

- `--detached` and `-d` on `fuku run` and on the root command
- A second fuku process, in its own session, with its standard streams detached
- A live Bubble Tea view in the style of `docker compose up -d`, or plain lines when stdout is not a terminal or `--no-ui` is given
- All or nothing: a service that fails after its retries stops the whole detached start
- `fuku stop` stops this project's running fuku gracefully before its existing directory cleanup

Out:

- A PID file or a log file for the detached fuku
- `fuku attach`, `fuku status`, `fuku restart` or JSON output for `fuku run --detached`
- REST API changes
- Reporting a failure that happens after the start succeeded
- Windows support

## How it works

1. After loading the config as today, `fuku run -d` starts `fuku run <profile> --no-ui --config <file>` with a hidden marker
2. The child runs in a new session with an extra pipe, and its stdin, stdout and stderr go to `/dev/null`
3. The marker makes the child write startup events to the pipe.
   A `ServiceFailed` or an unexpected `ServiceStopped` before the run phase fails the start
4. The parent renders each event as it arrives: the live view on a terminal, plain lines otherwise
5. Once every service is running and the log socket is bound, the child writes a success record and closes the pipe.
   The parent prints the summary and exits 0
6. Any failure before that, a guard refusal or a socket bind failure included, goes to the pipe as an error record.
   The child stops through its normal path. The parent prints the reason and exits 1. A closed pipe without success is a failure too
7. `Ctrl-C` in the parent sends `SIGTERM` to the child, waits for it and exits 130
8. `fuku stop` probes the project socket. When fuku answers, it sends `SIGTERM` to the reported PID, waits, then cleans up

The view keeps one line per service, updated in place, like `docker compose up -d`:

```text
[+] fuku run core -d 2/4
 ✔ postgres   Ready                                                 1.2s
 ✔ redis      Ready                                                 0.3s
 ⠋ api        Starting                                              2.8s
 ⠋ worker     Waiting                                               2.8s
```

## Acceptance criteria

- **AC1** – `fuku run -d`, `fuku run --detached`, `fuku -d`, `fuku run core -d` and `fuku -r core -d` parse to a detached run
- **AC2** – `fuku stop -d` and `fuku logs -d` are rejected as unknown flags
- **AC3** – the command exits 0 only after every service is running, and they keep running after the caller's session ends
- **AC4** – the summary names the PID, the service count, the startup time and, when the API server started, its address
- **AC5** – on a terminal without `--no-ui`, a Bubble Tea view shows one line per service with a spinner, state and elapsed time
- **AC6** – the final frame of the view stays on screen after the command returns
- **AC7** – when stdout is not a terminal or `--no-ui` is given, the output has no escape sequences and each line is written as its event arrives
- **AC8** – a service that fails after its retries stops every started service, and the command exits 1
- **AC9** – a start that fails for another reason, an existing run included, prints `Error: <reason>` and exits 1
- **AC10** – `Ctrl-C` during the wait leaves no fuku or service running, and the command exits 130
- **AC11** – `fuku stop` with a running fuku sends `SIGTERM`, waits for it to exit, then runs the directory cleanup
- **AC12** – `fuku stop` sends `SIGKILL` after the stop timeout and says so
- **AC13** – `fuku stop` with no running fuku keeps its current behavior
- **AC14** – `fuku logs` attaches to a detached fuku
- **AC15** – `fuku run` and `fuku run --no-ui` keep their current behavior, including continuing past a failed service

## Assumptions

- The socket and lock are scoped to the project ([project-scoped sockets](../20260913-project-scoped-sockets/feature.md)).
  One fuku runs per project, so `fuku stop` needs no profile to find it
- `SIGTERM` triggers fuku's graceful shutdown. Today Fx traps it only after `app.Start`, so fuku must trap it before services launch
- The child inherits the working directory that `cli.ChangeToConfigDir` set, so `<file>` is the config's base name

## Contracts

- Exit codes of `fuku run --detached`: `0` running, `1` failed or already running, `130` aborted
- The socket status message gains an optional `pid`. An older server omits it, and the client then cannot signal
- Only the parent records telemetry for the command
- The hidden marker and the startup pipe are internal to the binary and may change freely

## Decisions

- A flag, not a `fuku detached` command: every subcommand is a verb and `--no-ui` is already a run mode switch
- A second process, because Go cannot fork and the child must outlive the caller's session
- The child's own output is discarded. `fuku logs` already serves its logs over the socket
- All or nothing, because a half-running fuku without a TUI is hard to diagnose. Retries still apply
- The view follows `docker compose up -d`. A pipe gets plain lines because agents read output that way
- `fuku stop` uses `SIGTERM`, not a socket command, so no unauthenticated shutdown surface is added.
  It now also stops a foreground fuku of the project gracefully, which beats killing its services under it
