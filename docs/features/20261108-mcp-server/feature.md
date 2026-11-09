# MCP server

## Goal

An AI agent (Claude Code or Codex) works next to the developer on the local services fuku runs: it sees the service state,
starts, stops and restarts services, and maintains `fuku.yaml`. It drives fuku only through typed MCP tools. It never guesses.

## Why

The developer runs fuku with the TUI, with `--no-ui` in the foreground, with `-d` detached, or not at all. Shell commands
and raw API calls made agents guess ports, tokens, service IDs, polling and whose fuku it is. The tools move that into fuku.

## Scope

In:

- `fuku mcp`: a stdio MCP server with six tools, built on the official Go SDK
- An opt-in `wait` on the three REST service actions
- The API address and the run mode in the log socket `status` frame
- The agent plugin: MCP config for Claude Code and Codex, a skill for config editing, e2e scenarios graded on tool calls
- A startup banner and one line per tool call on stderr

Out:

- An HTTP MCP transport, resources, prompts, tasks, sampling, roots and logging
- Config editing tools. The agent edits `fuku.yaml` as a file
- A live TUI for `fuku mcp`, and tool arguments or results on stderr
- Logs since a service's current start ([BL-035](../backlog.md)), a `status` result when nothing runs
  ([BL-032](../backlog.md)) and one command order across the CLI parser ([BL-034](../backlog.md))

## How it works

The plugin starts `fuku mcp` with the agent session. `fuku` is on `PATH`. The server needs no running fuku and no valid
`fuku.yaml`. Every tool takes an optional `config` path, like `--config`, and loads the config fresh on each call.
The run mode says who owns the running fuku: `tui` and `foreground` are the developer's terminal session, `detached` is
a background fuku. An older fuku sends no `mode` and no `api`: the agent sees `unknown`, and the server probes `server.listen`.

The server instructions, the tool descriptions and the skill carry the agent's rules:

- Call `status` first. Call `run` only when nothing runs
- After a config edit, call `doctor` first. A config error never stops a working fuku
- Apply a config change with `stop` then `run` only under `detached`. Otherwise tell the developer their fuku needs a restart
- Experiment only in `fuku.override.yaml`. At the end restore it as it was, apply that like a config change, report the keys
- After a code edit, wait in `status` until the watching service the edit touches shows a new PID or `failed`, then test.
  Restart only a service that is not watching
- Leave fuku running when the task is done. Call `stop` only when the user asks, and report that fuku still runs

## Tools

| Tool                     | Annotation  | Does                                                                          |
|--------------------------|-------------|-------------------------------------------------------------------------------|
| `status`                 | read-only   | profile, phase, mode and each service's name, state, PID, uptime and watching |
| `logs(services?, tail?)` | read-only   | the newest lines, 100 by default and 1000 at most, never following            |
| `doctor(profile?)`       | read-only   | the `fuku doctor --json` report                                               |
| `run(profile)`           | write       | `fuku run <profile> -d`, its exit code, summary and each service's state      |
| `stop()`                 | destructive | `fuku stop` of the running profile                                            |
| `service(name, action)`  | write       | start, stop or restart, returning once the service settles or 45 seconds pass |

## Acceptance criteria

- **AC1** – `fuku mcp` answers `tools/list` with the six tools in a fixed order, in a project with no `fuku.yaml`
- **AC2** – `status`, `logs`, `stop` or `service` without a running fuku errors with the profiles to `run`; the server keeps serving
- **AC3** – `status` returns the mode and the name, state, PID, uptime and watching of every service in the profile
- **AC4** – `logs` returns at most `tail` lines and never follows
- **AC5** – `run` starts the profile detached and returns each service's state. A second `run` fails with the reason
- **AC6** – `service(name, "restart")` returns `running` with a new PID, or `failed` with its last 20 log lines
- **AC6a** – when the 45s wait ends first, `service` returns the current state with `inProgress: true`
- **AC7** – `service` with an unknown name returns an error that lists the valid names
- **AC8** – `service` with the API off returns an error that says to use `stop` and `run`
- **AC9** – the agent never sees the API token
- **AC10** – `POST /api/v1/services/{id}/{start,stop,restart}` with the body `{"wait": <seconds>}` (1 to 60) answers
  once the service settles or the wait ends. Without a body it answers `202` as before
- **AC11** – the log socket `status` frame carries the run mode and the API address, empty when the API is off
- **AC12** – the agent scenarios pass through the MCP tools alone, with no shell call to `fuku`, `api.sh` or the API.
  S5 restarts a detached fuku, S8 leaves fuku running, S9 leaves a foreground fuku running and asks the developer,
  S10 runs an experiment in the override and restores it
- **AC13** – `fuku mcp` writes its startup banner to stderr and nothing but protocol messages to stdout
- **AC14** – each tool call writes one stderr line with the tool, its outcome and its duration

## Contracts

- Tool names, parameters and output schemas are the agent contract. The instructions, descriptions and skill can change
- `wait`, `api` and `mode` are additive. The REST API and the socket wire stay compatible for current fuku users

## Decisions

- Stdio, not localhost HTTP: the server exists before fuku runs, needs no port and starts per project
- `run`, `stop` and `doctor` execute the binary, keeping one `fx.App` per invocation
- Waiting for a service to settle lives in the core and is exposed through REST, since frontends decide no readiness
- REST over new socket frames for state and actions: REST is already the public control contract
- `service` waits at most 45s, inside Codex's 60s default tool timeout
- A user's request to `stop` wins in any mode. On its own, the agent stops only a detached fuku, and only to apply a config
  change. A TUI or foreground fuku is the developer's session, so the agent asks them to restart it

## Details

- The ring 3 frontend, its tool results, API lookup and stderr lines: [`adapters/mcp`](../../../internal/adapters/mcp/README.md)
- Spec: MCP 2026-07-28, through `github.com/modelcontextprotocol/go-sdk` v1.8.0
- The agent sweep, its scenarios and host versions: [`plugins/agents/e2e/README.md`](../../../plugins/agents/e2e/README.md)
