---
name: fuku
description: >-
  Create or change fuku config: add or edit a service, change a service command or port, add a profile, make a
  local-only change or store a secret in fuku.yaml or fuku.override.yaml, apply a config change that was already
  made, or run an experiment with a temporary config change that must be undone at the end. Do not use to run,
  inspect, restart or stop services when no config changes.
---

# fuku

Control fuku only through the fuku MCP tools. Never run `fuku`, `curl` or the REST API from the shell, and never read the API token

If the fuku tools are not available, tell the user to install fuku, add it to `PATH` and enable the fuku plugin. Then stop
Do not use the shell instead

## Change configuration

Read [references/configuration.md](references/configuration.md) before you create or change fuku config

- Put a change in `fuku.yaml` only when the team must share it
- Put a change for this machine only in `fuku.override.yaml`: local commands, watch settings, exclusions and API settings
- If Git tracks the override, tell the user and ask before you edit it
- Store a token or another secret only in an override that Git does not track and ignores
- Never stage the override
- Never print a full config or a secret. Read only the keys that the task needs
- Pass `config` only when the user names a config file other than `fuku.yaml`. Then pass the same path to every tool

fuku reads its config one time, when it starts. A `service` restart does not apply a config change
After an edit, call `doctor` first
If `doctor` reports a config error, do not call `stop`. A running fuku continues to run while you fix the config
When `doctor` reports no config error, call `status` and read `mode`
If `mode` is `detached`, call `stop`, then call `run`
If `mode` is `tui`, `foreground` or `unknown`, do not call `stop`. The developer runs fuku in their own terminal, and `stop` ends their session. Tell the developer that the change needs a restart of their fuku, and end the task
`stop` also stops the remaining processes in the service directories of the profile. This includes processes that you did not start
When the task is done, leave fuku running. Call `stop` only when the user asks for it

## Run an experiment

An experiment is a temporary config change to try something. It goes only in `fuku.override.yaml` of the project
Never create a config file in another place, and do not pass `config` for an experiment
Before the change, copy the override to `fuku.override.yaml.bak` in the project. Do not read it. Note it when no override exists
Apply the experiment like a config change: `doctor`, then `status`, then `stop` and `run` only when `mode` is `detached`
At the end, move the copy back over the override. Delete the override when none existed before
Apply the restore the same way. fuku keeps running
Report the keys that the experiment changed, and that the override is restored

## Report the outcome

- The file that changed, and if the change is shared or for this machine only
- The state of each related service after the change
- That fuku still runs, or that you stopped or started it
