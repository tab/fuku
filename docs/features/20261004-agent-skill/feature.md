# Agent skill

## Goal

An AI agent in Claude Code or Codex can run, inspect, debug and stop a project's fuku services by installing one plugin.
The plugin ships one shared skill that drives fuku through its CLI and its REST API.

## Why

An agent that meets a `fuku.yaml` today guesses: it holds its shell with a foreground `fuku run`, follows `fuku logs` forever or kills services by hand.

Master now has what an agent needs:

- detached run `fuku run -d`
- a graceful `fuku stop`
- `fuku logs --tail <n> --no-follow`
- `fuku doctor --json`
- the single-instance guard and the project-scoped socket

Nothing tells the agent to use them.

An earlier attempt on `origin/feature/skill-work-in-progress` predates all of the above.
It carries an 860-line Python helper, a session hook and Go changes that worked around the missing pieces.
This feature starts over on master and keeps only the packaging and the still-valid guidance.

## Scope

In:

- `plugins/agents/` with one skill, `skills/fuku/SKILL.md`, in the shared Agent Skills format
- Two short references beside it: fuku config for agents, and the control API
- One small script, `scripts/api.sh`, that sends an API request with the token from the effective config
- `fuku stop` fails without cleanup when it cannot reach the running fuku ([BL-018](../backlog.md))
- `fuku stop` asks the running fuku to stop over its project socket. The socket is mode `0600`
- A plugin manifest and a marketplace file for Claude Code, and the same pair for Codex
- A structure check for the manifests and the skill, as a Make target and a CI job
- A docs page for the plugin, a link from the plugins index and a short `README.md` section

Out:

- Any other Go change. Apart from the two stop changes, the skill uses fuku as master ships it
- The Python helper `control.py`, the `SessionStart` hook and the `/status`, `/logs`, `/restart` slash commands of the earlier attempt
- A packaged copy of `spec/openapi.yaml`. The reference links the published API docs
- The earlier attempt's Go work that master lacks: the API log endpoints, the service `revision`, `fuku logs --since`, two doctor checks
- CLI commands for one service ([BL-008](../backlog.md)) and a `fuku status` command
- Publishing to a public marketplace or registry
- The repository's own developer skills in `.claude/skills/`

## How it works

1. The user adds the fuku marketplace in Claude Code or Codex and installs the `fuku` plugin
2. The skill loads when a task involves `fuku.yaml` or the project's local services
3. The agent runs `fuku doctor <profile> --json`. `runtime.instance` says whether fuku already runs for this project
4. When fuku runs, the agent attaches to it. It does not start a second one
5. When fuku does not run, the agent starts it with `fuku run <profile> -d` and reads the exit code and the summary
6. The agent reads output with `fuku logs --tail <n> --no-follow [service...]`. It never starts a follower
7. For the state of one service, or to start, stop or restart one, the agent calls the REST API when `server.listen` is set
8. A change to `fuku.yaml` or the override needs a full restart: `fuku stop`, then `fuku run <profile> -d`
9. The agent stops or restarts fuku when the task needs it, whoever started fuku: a config change, a profile switch or a request
10. `fuku stop` sends a stop frame over the project socket, and the running fuku stops its services as the TUI's `q` does.
    A fuku that does not echo the frame, such as an older one, gets `SIGTERM`

## Acceptance criteria

- **AC1** – Claude Code installs the plugin from the repository marketplace and lists the `fuku` skill
- **AC2** – Codex installs the plugin from the repository marketplace and lists the `fuku` skill
- **AC3** – both hosts load the same `SKILL.md`. No skill text exists twice
- **AC4** – every command and flag the skill names exists in `fuku help` on master, and every API path in `spec/openapi.yaml`
- **AC5** – the skill starts fuku only with `-d` and never tells the agent to keep a foreground `fuku run` alive
- **AC6** – the skill reads logs only with `--no-follow`
- **AC7** – the skill tells the agent to check `runtime.instance` before a start and to attach to a running fuku
- **AC8** – the skill never prints a whole fuku config or the API token, and says so
- **AC9** – the skill states that a config change needs a full restart and that a service restart does not reload config
- **AC10** – the skill states what works without the API (run, logs, stop, doctor) and what needs it (service state and service actions)
- **AC11** – the structure check fails on invalid manifest JSON, a missing frontmatter field or a broken relative link, and runs `scripts/api.sh` on fixture configs
- **AC12** – CI runs the structure check on a pull request
- **AC13** – the docs site has an agents plugin page with the install steps for both hosts, and it builds
- **AC14** – `scripts/api.sh` sends the token of the effective config, the override first, or of the one `--config` file.
  The token reaches no command line, environment variable or output. Without a readable token it sends no request and exits non-zero
- **AC15** – a `fuku stop` that cannot reach the running fuku prints the reason, exits 1, kills no process and keeps the socket
- **AC16** – `fuku stop` stops a running fuku over its socket without signalling it, and the socket is mode `0600`

## Assumptions

- Claude Code reads `.claude-plugin/marketplace.json` at the repository root and `.claude-plugin/plugin.json` in the plugin folder.
  Codex reads `.agents/plugins/marketplace.json` and `.codex-plugin/plugin.json`, as its documentation and its installed plugins show
- The agent's sandbox may block the Unix socket and loopback HTTP. The skill tells it to ask for access, not to treat fuku as down

## Contracts

- The plugin and the skill are both named `fuku`
- The plugin version is its own and starts at `0.1.0`. It does not follow the binary version
- The skill ships one executable, `scripts/api.sh`: POSIX `sh` with `awk` and `curl`. The structure check lives in `plugins/agents/tests/`

## Decisions

- One skill in one plugin folder for both hosts, because the Agent Skills format is shared and two copies would drift
- CLI first, because `-d`, `stop`, `logs` and `doctor` need no token and no API
- No discovery helper. The old helper's discovery, port probing and second-instance logic are now the guard, the project socket and `doctor`
- The agent manages fuku whoever started it, because the developer and the agent work on one task together
- No hook and no slash commands in the first version. The skill covers the same ground, and each one is another surface to keep true
- Two Go changes, both in `fuku stop`, because a sandboxed agent is the caller that hits them. Each missing API or CLI piece stays its own feature
- `fuku stop` asks over the socket before it signals, because a sandbox denies a signal to a fuku started outside it.
  The socket is `0600`, so only its owner can send the stop frame
- The API token stays: the server checks neither `Host` nor `Origin`, so it is the only defence against a browser page ([BL-017](../backlog.md)).
  A script, not a shell line, reads it: parsing YAML in one line is unreadable. The script pipes the header into `curl` on stdin, so `ps` never shows it
- The structure check is one standard-library Python `unittest` file, because it parses JSON and frontmatter and resolves links
