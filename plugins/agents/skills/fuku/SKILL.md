---
name: fuku
description: >-
  Use for every task about fuku or the local services it runs, before the first action: start, stop or restart
  services, read their state, logs or config, or diagnose a failure. It names the fuku MCP tool for each job. To
  create or change fuku config, also use the fuku-config skill.
---

# fuku

Control fuku only through the fuku MCP tools.

- Never run `fuku`, `curl`, the REST API or a script from the shell, not even to probe
- Never read the API token
- If no tool does what the task needs, change nothing in fuku and tell the user which tool is missing
- If the fuku tools are not available, tell the user to install fuku, add it to `PATH` and enable the fuku plugin.
  Then stop, and do not use the shell instead

## Pick the tool

- State of fuku and each service: `status`. It also answers when nothing runs
- The effective config of a service or profile: `config`. Read it instead of `fuku.yaml`
- Recent log lines: `logs`. It never follows
- Start a profile: `run`. Call `status` first, and call `run` only when nothing runs
- Start, stop or restart one service: `service`
- Stop fuku: `stop`. Call `status` first
- Diagnose a failed `run` or a config edit: `doctor`
- Create or change config: the fuku-config skill

## Finish the task

Leave fuku running. Call `stop` only when the user asks for it.
