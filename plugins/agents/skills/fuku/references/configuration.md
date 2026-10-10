# fuku configuration for agents

Read this before you create or change fuku config
It holds the rules an agent needs, not the whole schema

## Find the effective config

- fuku reads `fuku.yaml`, or `fuku.yml` when the first is absent, from the working directory
- It merges `fuku.override.yaml`, or `fuku.override.yml`, from beside that file on top
- An override without a base config is ignored
- `--config <path>` reads that exact file and merges no override, even one beside it
- With `--config <path>`, fuku first changes into the directory of that file
- `fuku doctor <profile> --json` names the files it used: `config.file` and `config.override`

Never print a whole config file to find a value. Read the one key you need
Never print `server.auth.token`. Use `scripts/api.sh` from [control-api.md](control-api.md) when a request needs it

## Choose the file

| Goal                     | File                                       |
|--------------------------|--------------------------------------------|
| Shared project behavior  | `fuku.yaml`, committed for the team        |
| Local machine behavior   | `fuku.override.yaml`, untracked and small  |
| A separate setup         | A file the user passes with `--config`     |

Do not put a local debugger command, machine path, port or token in the shared config unless the user wants a shared change
Do not move a requested shared change into the override, or a requested local change into the shared config

Before a local-only edit, check whether Git tracks the override
If it does, say that the change will show in Git and ask before you edit it
Before you store a token or another secret in the override, confirm it is untracked and ignored
If it is not ignored, stop and ask whether to add an ignore rule
Never stage a local-only override change

## Keep the override small

Add only the keys that differ from the base

- A mapping merges by key
- A scalar, or a value of another kind, replaces the base value
- A sequence appends after the base items. It never replaces them
- An explicit `null` deletes the key

Because sequences append, do not repeat a profile list to shrink it
Use `exclude` to drop a service locally, or add a local profile with its own list

## The keys an agent edits

- `services.<name>.dir`: the working directory, default the service name
- `services.<name>.command`: run through `sh -c`, default `make run`
- `services.<name>.tier`: the startup group. Tiers start in the order they first appear in the file
- `services.<name>.readiness`: `type` is `http` with `url`, `tcp` with `address`, or `log` with a regexp `pattern`
- `services.<name>.watch`: `include` globs are required. `ignore`, `shared` and `debounce` are optional
- `services.<name>.logs.output`: `stdout`, `stderr` or both
- `profiles.<name>`: `"*"` for every service, or a list of service names that exist in `services`
- `exclude`: service names dropped from every profile on this machine
- `server.listen`: the API address, `127.0.0.1:3858` when unset
- `server.listen` set to `""` or `none` turns the API off
- `server.listen` otherwise takes `host:port` on a loopback IP such as `127.0.0.1` or `::1`, or on `localhost` or `ip6-localhost`
- `scripts/api.sh` reaches only `127.0.0.1`, `localhost` and `[::1]`, so another loopback IP such as `127.0.0.2` is out of its reach
- `server.auth.token`: optional. When set, every API call except `/live` and `/ready` needs it

A service with an empty body, such as `api:` alone, is invalid
`env.files` only feeds the env tab of the terminal UI. Nothing in it reaches the service process

## Validate and apply

Run `fuku doctor <profile> --json` after every edit
`config.validate` reports a schema error, and `topology.profile` reports an unknown profile
With an explicit file, pass the same `--config <path>` to doctor

fuku reads its config once, when it starts
A running fuku never reloads it, and a service start or restart through the API reuses it
To apply a config change, restart the whole profile: `fuku stop <profile>`, then `fuku run <profile> -d`
Never emulate a profile restart with one API call per service
