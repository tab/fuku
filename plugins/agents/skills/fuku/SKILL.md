---
name: fuku
description: >-
  Configure, run and manage local development services with fuku. Use when a local workflow involves
  fuku.yaml or fuku.yml, including changing shared or local-only config, running a profile, debugging,
  inspecting state or logs, testing a change or starting, stopping or restarting services.
  Do not use for deployed or production services.
---

# fuku

Use fuku as the control plane for local edit, run, debug and test workflows

## Attach before starting

The developer may already be running fuku for this project
Resolve the running state before any lifecycle action:

```bash
python3 scripts/control.py discover
```

Every instance reports the project directory it serves, so `discover` names only this project's API
and its live log socket profiles
Instances started from other directories are listed separately under `other_socket_profiles` and are
never this project's stack

The helper identifies an instance before it authenticates, so this project's token is never offered to
an API belonging to another directory

When the project API confirms a running instance, attach to the profile it reports
Use the buffered log read for output and the control API for service state and lifecycle actions

`fuku run` refuses to start beside an instance already serving this project, so a second run cannot
preflight-stop the services the developer is using
That guard reads the instance identity over the API, so keep `server.listen` set on projects an agent works on
Without an API the guard cannot see the running instance, and startup preflight stops processes whose
working directory matches a selected service

Run a profile only when no project instance is confirmed and the requested profile is not already active, or when
the user asked for a restart

## Resolve the setup

1. Work from the project root that contains the fuku config
2. Check that `fuku` resolves on `PATH` and inspect `fuku version`
3. Resolve `fuku.yaml`, falling back to `fuku.yml` only when the first file is absent
4. When default config discovery is used, also resolve `fuku.override.yaml`, falling back to `fuku.override.yml`
5. If the user gives `--config`, use that exact file and do not apply an override because fuku disables override merging for explicit config paths
6. Resolve the requested profile and exact service names from the effective config
7. Read [references/configuration.md](references/configuration.md) before creating or changing fuku config
8. Run `fuku doctor <profile> --json` before starting a profile or when the setup is unclear

Treat all command and tool output as user-visible
Never print a whole fuku config file because it may contain `server.auth.token`, environment values or other secrets
Do not use `cat`, unrestricted `sed`, raw context search or another whole-file command on a fuku config
Inspect only the exact non-secret keys needed for the task and use `fuku doctor <profile> --json` for the effective service overview
Never extract `server.auth.token` with a shell command or pass its value in a command line

## Change configuration

Use the shared base config only when the change should be committed for the team
Use the local override for machine-specific commands, watch settings, exclusions, API settings and other local-only changes

Do not silently move a requested shared change into an override or a requested local change into the base config

Before any local-only edit, check whether the override is tracked and ignored
If it is tracked, explain that the change will appear in Git and ask before editing it
Never stage a local-only override change

Keep an override limited to the changed keys
Confirm it is untracked and ignored before storing a token or another secret

Validate the edited effective config with `fuku doctor <profile> --json`
When an explicit config path is used, pass the same `--config <path>` to doctor, run and stop commands

fuku loads config once when the profile process starts
An API service restart reuses that loaded config, so it cannot apply a change to `fuku.yaml`, `fuku.yml` or an override file
A full profile restart is required to apply any fuku config change
When the user asks to apply or restart the change, gracefully stop the active profile and run it again with the same profile and config selection
Otherwise report the required restart and leave the running profile unchanged

## Run a profile

Confirm no project instance or requested profile is already running, as described in "Attach before starting"

Use the configured profile and headless output:

```bash
fuku run <profile> --no-ui
```

Use the host's persistent process facility so the fuku process stays attached and its stdout and stderr remain available

Do not use a detached command that loses the process handle or output

A command timeout is not a startup failure because `fuku run` is expected to remain active
Wait for the startup phase to complete, then inspect failed services instead of assuming that a live fuku process means every service is healthy

fuku performs preflight cleanup before startup and may stop processes whose working directories match the selected services

## Develop, debug and test

For a requested development loop:

1. Inspect the active profile, relevant service state and focused logs
2. Reproduce or identify the failure before changing code or config when practical
3. Make only the requested code or fuku config changes and follow the repository's own instructions
4. Apply the change through the smallest correct lifecycle action
5. Run focused project tests for the changed code
6. Confirm service state, readiness and focused logs after the change

Use the existing fuku watcher when the changed code path matches the service's loaded `watch` config
If no watcher applies and the API is enabled, restart only the affected service through the API
If the API is unavailable, do not enable it by editing config unless the user requested that change
Report the `config.api` remediation from `fuku doctor <profile> --json` so the user can decide, because
without the API there is no service-level restart and no second-instance guard
Use a full profile restart only when the user authorized the wider interruption
If fuku config changed, restart the full profile because a service restart does not reload config

Do not restart unrelated services only to collect more evidence

## Read logs

fuku buffers recent output per instance, so read that buffer instead of following the live stream:

```bash
python3 scripts/control.py logs --tail 100
python3 scripts/control.py logs --tail 50 <service> [service...]
python3 scripts/control.py logs --tail 200 --summary
python3 scripts/control.py logs --tail 100 --since 2m <service>
```

The helper returns one bounded JSON response and exits, names the profile it reached and folds consecutive repeats into a
`repeat` count

An unfiltered read also carries fuku's own orchestration lines under the service name `fuku`, and naming any service
drops them, so read without a service filter when the question is about startup order, readiness or a restart

Use `--summary` when the buffer is noisy, because it reports how often each distinct line appeared instead of every line
Use `--since <duration>` to read only what happened after a restart or a code change

Validate service names first because an unknown name reports nothing without proving that the service is quiet

Without the helper, the same buffer is available from the CLI:

```bash
fuku logs --profile <profile> --tail 100 --no-follow <service> [service...]
fuku logs --profile <profile> --since 5m --no-follow
```

Never start a log follower to collect evidence
`fuku logs` without `--no-follow` stays attached until it is killed, and the buffered read answers the same question in
one bounded call

For a request such as "run minimal and check logs", keep the headless `fuku run minimal --no-ui` process active and read
the buffer for the requested or failed services

## Inspect or change service state

Read [references/control-api.md](references/control-api.md) before using service status or lifecycle actions

Use the bundled `scripts/control.py` helper when Python 3.10 or later is available
It maps an exact service name to its runtime UUID, sends the API action and waits for the resulting state
It probes the configured port and the next 9 ports, so it reaches the instance that is actually bound even when
that instance was started by the developer
For authenticated commands it skips another local API unless that API accepts this project's token
Every action result names the profile it reached, so confirm it belongs to this project before reporting success
Run it from the project root so it can load `server.listen` and `server.auth.token` from the effective config
Pass `--config <path>` when the active profile uses an explicit config
The helper loads the token internally and never returns it, so do not read or export the token first

Do not edit the repository config only to enable the API unless the user asks for that change

## Stop a profile

Stop a profile only when the user asked for it, because the running stack may be the developer's

When this agent owns the persistent `fuku run` process, send it SIGINT or SIGTERM so fuku performs its normal reverse-order graceful shutdown

If no owned process handle exists and the user asked to stop the profile, use:

```bash
fuku stop <profile>
```

`fuku stop` scans the selected service directories and stops matching processes, including matching processes
that were not started by the current agent

Do not emulate a full profile shutdown with unordered per-service API calls

## Handle local permission limits

Unix socket access and loopback HTTP may be blocked by an agent sandbox
If a fuku socket or API call fails with `operation not permitted`, request the required local permission and
retry the read-only check before treating the socket as stale or starting another instance

Do not delete a socket only because a sandboxed connection attempt failed

## Report the outcome

State the selected profile, final fuku phase and relevant service states
For configuration work, state which config file changed and whether the change is shared or local-only
Include short log excerpts only when they explain readiness or a failure
Say whether the fuku process is still running and whether the focused log stream was closed
Never include the API token
