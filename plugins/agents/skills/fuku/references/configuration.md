# fuku configuration for agents

Read this file before you create or change fuku config
It gives the rules that an agent must know. It does not give the full schema

## Find the effective config

- fuku reads `fuku.yaml` from the project root. If that file does not exist, fuku reads `fuku.yml`
- fuku then merges `fuku.override.yaml` or `fuku.override.yml` from the same directory into that config
- fuku ignores an override file when no base config exists
- With a `config` path, fuku reads only that file. It does not merge an override, even if one is in the same directory
- With a `config` path, the directory of that file is the project
- Exception: when `config` names the file that the default lookup reads in the project root, the tools use the default lookup
- The `doctor` report shows the files that fuku used in the checks `config.file` and `config.override`

Never print a full config file to find a value. Read only the key that you need
Never print or read `server.auth.token`. The fuku tools send the token to the API

## Choose the file

| Goal                     | File                                       |
|--------------------------|--------------------------------------------|
| Shared project behavior  | `fuku.yaml`, committed for the team        |
| Local machine behavior   | `fuku.override.yaml`, untracked and small  |
| A separate setup         | A file that the user gives as `config`     |
| A temporary experiment   | `fuku.override.yaml`, restored at the end  |

An experiment never gets its own config file. Copy the override to `fuku.override.yaml.bak` first, without a read
At the end, move the copy back, or delete the override when none existed. Apply the change and the restore like every config change

Keep a local debugger command, a machine path, a port or a token out of the shared config
Put them in the shared config only when the user asks for a shared change
Do not move a shared change that the user asks for into the override
Do not move a local change that the user asks for into the shared config

If Git tracks the override, tell the user that a local-only edit will show in Git
Then ask the user before you edit the override
Before you store a token or another secret in the override, make sure that Git does not track it and ignores it
If Git does not ignore the override, stop. Ask the user if they want an ignore rule
Never stage a local-only override change

## Keep the override small

Add only the keys that are different from the base config

- A mapping merges by key
- A scalar, or a value of another kind, replaces the base value
- A sequence adds its items after the base items. It never replaces them
- An explicit `null` deletes the key

A sequence adds items, so a repeated profile list cannot make the list shorter
To remove a service on this machine, use `exclude`. Or add a local profile with its own list

## The keys an agent edits

- `services.<name>.dir`: the working directory. The default is the service name
- `services.<name>.command`: the command that fuku runs through `sh -c`. The default is `make run`
- `services.<name>.tier`: the startup group. Tiers start in the order of their first appearance in the file
- `services.<name>.readiness`: `type` is `http` with `url`, `tcp` with `address`, or `log` with a regexp `pattern`
- `services.<name>.watch`: you must set `include` globs. `ignore`, `shared` and `debounce` are optional
- `services.<name>.logs.output`: `stdout`, `stderr` or both
- `profiles.<name>`: `"*"` for all services, or a list of service names from `services`
- `exclude`: the service names that fuku removes from every profile on this machine
- `server.listen`: the API address. The default is `127.0.0.1:3858`
- `server.listen` set to `""` or `none`: fuku does not start the API
- Any other `server.listen` value must be `host:port` with a loopback IP such as `127.0.0.1` or `::1`, or `localhost` or `ip6-localhost`
- `server.auth.token`: optional. If you set it, every API call except `/live` and `/ready` must send it

A service without keys, such as `api:` alone, is not valid
`env.files` only supplies the env tab of the terminal UI. fuku does not send these values to the service process

## Validate and apply

Call `doctor` after every edit
`config.validate` reports a schema error, and `topology.profile` reports an unknown profile
With an explicit file, pass the same `config` path to every tool

fuku reads its config one time, when it starts
A running fuku never reads the config again. A `service` start or restart also uses the config from the start
To apply a config change, restart the full profile. Call `status` first and read `mode`
If `mode` is `detached`, call `stop`, then call `run`
If `mode` is `tui`, `foreground` or `unknown`, do not call `stop`. The developer runs fuku in their own terminal, and `stop` ends their session. Tell the developer that the change needs a restart of their fuku
Do not restart a profile with one `service` call for each service
