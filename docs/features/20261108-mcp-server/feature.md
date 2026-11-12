# MCP server

## Goal

An AI agent (Claude Code or Codex) works next to the developer on the local services fuku runs. It sees their state,
starts, stops and restarts them, and edits fuku config. It drives fuku only through typed MCP tools and never guesses.

## Why

The developer runs fuku with the TUI, in the foreground, detached, or not at all. Shell commands and raw API calls made
agents guess ports, tokens, service IDs, polling and whose fuku it is. The tools move that knowledge into fuku.

## Scope

In:

- `fuku mcp`: a stdio MCP server with seven tools, built on the official Go SDK
- An opt-in `wait` on the three REST service actions, and the run mode and API address in the log socket `status` frame
- The agent plugin for Claude Code and Codex: the MCP config, two skills and a sweep that grades real agent runs

Out:

- An HTTP transport, resources, prompts, tasks, sampling, roots and MCP logging
- Config editing tools. The agent edits a config file and reads the effective config through `config`
- Following logs live and logs since a service's current start ([BL-035](../backlog.md))

## How it works

The plugin starts `fuku mcp` with the agent session. The server needs no running fuku and no valid `fuku.yaml`.
Every tool takes an optional `config` path, like `--config`, and loads the config fresh on each call.
The run mode says who owns the running fuku: `tui` and `foreground` are the developer's session, `detached` runs in the
background, and an older fuku reports `unknown`.

The agent's rules:

- Control fuku only through the tools: never the `fuku` CLI, `curl`, the REST API or a script, not even to probe
- When no tool does what the task needs, change nothing in fuku and name the missing tool
- Read config through `config`. Open a config file only to edit one key
- Call `status` before `run` or `stop`. Call `run` only when nothing runs
- After a config edit, call `doctor`. Apply the change with `stop` then `run` only under `detached`; otherwise tell the
  developer their fuku needs a restart
- After a code edit, wait in `status` for a watching service's new PID, then test. Restart an edited service that is not
  watching, and any service the user asks for
- Experiment only in `fuku.override.yaml`, then restore it as it was and apply the restore
- Leave fuku running when the task is done. Call `stop` only when the user asks

## Tools

| Tool                     | Annotation  | Does                                                                        |
|--------------------------|-------------|-----------------------------------------------------------------------------|
| `status`                 | read-only   | mode and each service's state, PID, uptime and watching; `running: false` and the profiles when nothing runs |
| `config(service?)`       | read-only   | the effective config: profiles, and each service's command, dir, tier, readiness, watch and env files |
| `logs(services?, tail?)` | read-only   | the newest lines, 100 by default and 1000 at most; never follows            |
| `doctor(profile?)`       | read-only   | the `fuku doctor --json` report                                             |
| `run(profile)`           | destructive | `fuku run <profile> -d` and each service's state                            |
| `stop()`                 | destructive | `fuku stop` of the running profile                                          |
| `service(name, action)`  | destructive | start, stop or restart one service, returning once it settles or 45s pass   |

## Acceptance criteria

- **AC1** – `fuku mcp` lists the seven tools in a fixed order, in a project with no `fuku.yaml`
- **AC2** – with no fuku running, `status` returns `running: false` and the profiles; `logs`, `stop` and `service` fail
  with the profiles to `run`; the server keeps serving
- **AC3** – `status` returns the mode and each service's state, PID, uptime and watching
- **AC4** – `logs` returns at most `tail` lines and never follows
- **AC5** – `run` starts the profile detached and returns each service's state; a second `run` fails with the reason
- **AC6** – `service` returns the settled state: a restart gives `running` with a new PID or `failed` with the last 20
  log lines, a stop gives `watching: false`, and a wait past 45s gives the current state with `inProgress: true`
- **AC7** – `service` with an unknown name lists the valid names; with the API off it says to use `stop` and `run`
- **AC8** – `config` returns the effective config of every service or one, without a running fuku
- **AC9** – no tool result holds the API token of the config the call names or of the project's default config
- **AC10** – `POST /api/v1/services/{id}/{start,stop,restart}` with `{"wait": <seconds>}` (1 to 60) answers once the
  service settles or the wait ends; without a body it answers `202` as before
- **AC11** – the log socket `status` frame carries the run mode and the API address, empty when the API is off
- **AC12** – `fuku mcp` writes its banner and one line per tool call to stderr, and only protocol messages to stdout
- **AC13** – the sweep passes on both hosts: S1–S9 and S15 make no CLI probe and run no script, S6 reads config only
  through `config`, and S15 names the missing tool and changes nothing

## Contracts

- Tool names, parameters and output schemas are the agent contract. The instructions, descriptions and skills can change
- `wait`, `api` and `mode` are additive. The REST API and the socket wire stay compatible

## Decisions

- Stdio, not localhost HTTP: the server exists before fuku runs, needs no port and starts per project
- `run`, `stop` and `doctor` execute the binary, keeping one `fx.App` per invocation
- Waiting for a service to settle lives in the core and is exposed through REST
- `service` waits at most 45s, inside Codex's 60s tool timeout
- The user's request to `stop` wins in any mode. On its own the agent stops only a detached fuku, to apply a config change
- A missing tool is reported, never worked around, so each gap becomes a request for a new tool
- The first 512 characters of the instructions stand alone, as Codex asks; the tool map follows, within Claude Code's 4,096
- Two focused skills, as both hosts advise: `fuku` for every task (rules and tool map) and `fuku-config` for config edits
- Hiding the API token from the agent keeps it on the tools. It is not a security boundary, because the agent runs as the user and could read the config

## Details

- The server, its results and stderr lines: [`adapters/mcp`](../../../internal/adapters/mcp/README.md)
- The sweep, its scenarios and grader: [`plugins/agents/e2e/README.md`](../../../plugins/agents/e2e/README.md)
- Spec: MCP 2026-07-28, through `github.com/modelcontextprotocol/go-sdk` v1.8.0
