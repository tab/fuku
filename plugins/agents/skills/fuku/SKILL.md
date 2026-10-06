---
name: fuku
description: >-
  Run, inspect, debug and stop local development services with fuku. Use when a local workflow involves
  fuku.yaml or fuku.yml, including changing shared or local-only config, running a profile, reading service logs,
  checking service state, testing a change or starting, stopping or restarting services.
  Do not use for deployed or production services.
---

# fuku

Use fuku as the control plane for local edit, run, debug and test workflows

## Resolve the setup

1. Work from the project root that holds `fuku.yaml`, or `fuku.yml` when the first is absent
2. Check that `fuku` resolves on `PATH` and note `fuku version`
3. Without `--config`, fuku merges `fuku.override.yaml` or `fuku.override.yml` on top of the config
4. With `--config <path>`, fuku reads that exact file and merges no override
5. When the user gives `--config <path>`, pass it to every doctor, run, logs and stop command
6. Run every fuku command from the project root. A `--config` path with a directory part makes fuku change into that directory and treat it as the project
7. Take the profile and the exact service names from the config. Without a profile, fuku uses `default`
8. Read [references/configuration.md](references/configuration.md) before you create or change fuku config

Treat all command output as user-visible
Never print a whole fuku config. It may hold `server.auth.token`, environment values or other secrets
Do not run `cat`, an unrestricted `sed` or a whole-file search on a fuku config
Read only the exact non-secret keys the task needs, and use `fuku doctor <profile> --json` for the effective setup

## Attach before starting

fuku may already run for this project, in a terminal or in the background
Only one fuku runs per project

Run `fuku doctor <profile> --json` before any lifecycle action and read `checks["runtime.instance"].status`:

- `idle`: no fuku runs for this project. You may start one
- `note`: fuku already runs for this project. Attach to it
- `warn`: the socket exists but did not answer. Read "Sandbox limits" before you act on it

To attach, use the running fuku as it is: read its logs and, when the API is on, its service state
`fuku doctor` does not report which services run. With the API on, `GET /services` does
Without it, read fuku's own events, as far back as the shared `logs.history` buffer holds: `fuku logs --tail 200 --no-follow --no-ui fuku`
The latest `service_ready`, `service_stopped` or `service_failed` line for a service decides whether it runs
Never start a second fuku beside it. `fuku run` refuses anyway
Do not stop or restart it when it serves the wanted profile and the task needs no restart
`fuku logs --profile <profile> --tail <n> --no-follow --no-ui` fails when the running fuku serves another profile, and names it
When the task needs the other profile, run `fuku stop <running profile>`, then `fuku run <profile> -d`. Otherwise report the running profile

Read the rest of the doctor report for config errors, missing directories and busy ports before a start

## Run a profile

Start fuku only when `runtime.instance` is `idle`, and only in the background with `fuku run <profile> -d`
It returns once every service runs, or once the start fails
Never run fuku in the foreground and never keep a `fuku run` process attached to your shell

Read the exit code:

- `0`: every service runs. The summary names the PID, the service count, the startup time and the API address when the API started
- `1`: the start failed or fuku already runs for this project. The output names the reason
- `130`: the start was aborted. Nothing is left running

After exit code 0, the summary answers which services run: one `✔ <service> Ready` line each, and the count
Do not read the logs or the doctor report to learn that

The start is all or nothing. A service that still fails after its retries stops every started service
After a failure, read the reason and the doctor report, and fix the cause before you start again

## Read logs

fuku keeps recent output in one bounded buffer shared by all services, sized by `logs.history`. Read it and exit:
`fuku logs --tail <n> --no-follow --no-ui [service...]`

- `--tail <n>` returns the newest n lines of the named services in total, not n per service. A noisy service can push a quiet one out
- `--tail` takes a positive count. Start near 100 and raise it only when needed
- Name services with their exact config names to narrow the output. Without a name, the read covers the whole profile
- `--no-ui` drops the banner and keeps only the log lines
- Logs come over the project socket and need neither the API nor the token

Never start a log follower. A log read without `--no-follow` stays attached until it is killed

## Service state and service actions

The REST API serves the state of one service and the actions on it
It runs only when the config sets `server.listen`. `fuku doctor --json` does not report that key
Learn whether it is set in one of two ways:

- the `API <host:port>` line of the `fuku run <profile> -d` summary
- one bounded read of that key, which prints no secret: `grep -n '^ *listen:' fuku*.y*ml`. With `--config`, name that file instead

The grep misses a `server` block written on one line, so the summary's `API` line is the reliable sign

When `server.listen` is not set, do not open the control API reference
When it is set, read [references/control-api.md](references/control-api.md) before any API call

- `GET /status` gives the profile, the phase and the service counts
- `GET /services` lists the services with their `id`, `name` and `status`
- `GET /services/{id}` gives one service
- `POST /services/{id}/start`, `/services/{id}/stop` and `/services/{id}/restart` act on one service

The paths sit under `/api/v1` on the `server.listen` address
An action takes the service UUID, never its name. Map the name to the `id` with `GET /services` first
An action returns at once. Poll `GET /services/{id}` until the service settles

The API needs `server.auth.token`, and the token never reaches output
Never print, echo or export it, and never put its value in a command line
Send every authenticated request with `scripts/api.sh` from this skill's folder, as the control API reference shows
It reads the token itself and keeps it out of every command line and output

When the service's `watch` config covers the changed files, fuku restarts the service on its own
Otherwise restart only the affected service through the API
When the API is off, restart the whole profile yourself: `fuku stop <profile>`, then `fuku run <profile> -d`
Do not restart unrelated services to collect more evidence

## What works without the API

Without `server.listen`, these still work:

- `fuku run <profile> -d` starts the profile
- `fuku logs --tail <n> --no-follow --no-ui` reads the logs
- `fuku stop <profile>` stops fuku
- `fuku doctor <profile> --json` reports the setup and whether fuku runs

The state of one service and the start or stop of one service need the API
When it is off, do not edit the config to turn it on unless the user asks
Report that these need `server.listen`, let the user decide, and use the logs and the doctor report meanwhile

## Change configuration

Put a change in `fuku.yaml` only when the team should share it
Put machine-specific commands, watch settings, exclusions, API settings and other local-only changes in `fuku.override.yaml`
Do not move a requested shared change into the override, or a local change into the shared config

- Before a local-only edit, check whether Git tracks the override. If it does, say so and ask before you edit it
- Confirm the override is untracked and ignored before you store a token or another secret in it
- Never stage a local-only override change, and keep the override limited to the changed keys
- Validate the result with `fuku doctor <profile> --json`

fuku reads its config once, when it starts
A service restart through the API reuses that config and does not apply a config change
A config change needs a full restart: `fuku stop <profile>`, then `fuku run <profile> -d`
Do not try `fuku run <profile> -d` first while fuku runs. It only refuses

## Stop fuku

Stop or restart fuku whenever the task needs it, whoever started it: a config change, a profile switch, a request to stop or restart
Use `fuku stop <profile>` with the profile the running fuku serves, because the directory cleanup uses that argument
It stops this project's fuku gracefully, then stops leftover processes in the profile's service directories
That cleanup also stops matching processes you did not start

Run it only after `fuku doctor <profile> --json`, with socket access, reports `runtime.instance` as `note`
Never run `fuku stop` while `runtime.instance` is `warn`
When it prints `Error: cannot reach the running fuku, nothing was stopped` and exits 1, fuku still runs
It killed no process and kept the socket. Report that and ask for socket access
Never stop a whole profile with one API stop per service

## Sandbox limits

An agent sandbox may block the Unix socket and loopback HTTP
A failed socket or API call, such as `operation not permitted`, does not mean fuku is down

- Ask the user for local socket or network access, then retry the read-only check
- Do not start a second fuku because a check failed
- Do not run `fuku stop` while the socket is unreachable. It fails with exit 1 and stops nothing
- Do not delete a fuku socket, even when the doctor report suggests it, until a check with access confirms it is stale

## Report the outcome

- The profile
- The final phase and the relevant service states
- For config work, which file changed and whether the change is shared or local-only
- Short log excerpts only when they explain readiness or a failure
- Whether fuku still runs, and whether you started, stopped or restarted it
- Never include the API token or a whole fuku config
