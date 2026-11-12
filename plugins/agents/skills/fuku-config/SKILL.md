---
name: fuku-config
description: >-
  Create or change fuku config: add or edit a service, change a service command or port, add a profile, make a
  local-only change or store a secret in fuku.yaml or fuku.override.yaml, apply a config change that was already
  made, or run an experiment with a temporary config change that must be undone at the end. Do not use to start,
  stop, restart or inspect services when no config changes; the fuku skill covers that.
---

# fuku config

Do every fuku action through the fuku MCP tools, as the fuku skill says.
[references/configuration.md](references/configuration.md) gives the file lookup, the merge rules and the keys.
Read it before you create or change fuku config.

## Choose the file

- Put a change in `fuku.yaml` only when the team must share it
- Put a change for this machine only in `fuku.override.yaml`: local commands, machine paths, ports, watch settings,
  exclusions and API settings
- Keep a change where the user asks for it. Do not move a shared change into the override, or a local change into `fuku.yaml`
- Use another config file only when the user names one. Then pass the same `config` path to every tool

## Read and edit safely

- Open a config file only to edit it, and read only the key you change. Read the current config with `config`, as the fuku skill says
- Never print a full config or a secret
- If Git tracks the override, tell the user that a local edit will show in Git, and ask before you edit it
- Store a token or another secret only in an override that Git does not track and ignores.
  If Git does not ignore it, stop and ask the user if they want an ignore rule
- Never stage the override

## Apply a config change

fuku reads its config once, when it starts. A `service` start or restart does not apply a config change.

1. After the edit, call `doctor`
2. If `doctor` reports a config error, fix it and call `doctor` again. Do not call `stop`: the running fuku keeps running
3. When `doctor` reports no config error, call `status`
4. If `running` is false, nothing needs a restart. The next `run` uses the new config
5. If `mode` is `detached`, call `stop`, then call `run`
6. If `mode` is `tui`, `foreground` or `unknown`, do not call `stop`.
   The developer runs fuku in their own terminal, and `stop` ends their session.
   Tell the developer that the change needs a restart of their fuku, and end the task

Never restart a profile with one `service` call for each service.
`stop` also ends the remaining processes in the profile's service directories, including ones you did not start.

## Run an experiment

An experiment is a temporary config change to try something.
Make it only in the project's `fuku.override.yaml`. Never create a config file anywhere else, and do not pass `config`.

1. Call `status`. If `mode` is `tui`, `foreground` or `unknown`, do not start.
   Tell the developer that the experiment needs a restart of their fuku, and end the task
2. Copy the override to `fuku.override.yaml.bak` in the project. Do not read it. If no override exists, note that
3. Make the change in the override
4. Apply it like any config change. With nothing running, call `run` to try it
5. At the end, move the copy back over the override, or delete the override if none existed before
6. Apply the restore the same way. fuku keeps running

## Report the outcome

- The file that changed, and if the change is shared or for this machine only
- For an experiment, the keys it changed and that the override is restored
- The state of each related service after the change
- That fuku still runs, or that you stopped or started it
