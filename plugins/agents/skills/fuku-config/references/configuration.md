# fuku configuration reference

The facts an agent needs to edit fuku config: where fuku finds it, how the override merges and which keys exist.
It is not the full schema. The rules for choosing a file, editing safely and applying a change are in [SKILL.md](../SKILL.md).

## Find the effective config

- fuku reads `fuku.yaml` from the project root. If that file does not exist, fuku reads `fuku.yml`
- fuku then merges `fuku.override.yaml` or `fuku.override.yml` from the same directory into that config
- fuku ignores an override file when no base config exists
- With a `config` path, fuku reads only that file and merges no override. The directory of that file is the project
- When `config` names the file the default lookup reads in the project root, the tools use the default lookup
- The `config` tool returns the merged result, and `doctor` names the files used in `config.file` and `config.override`

## Choose the file

| Goal                    | File                                      |
|-------------------------|-------------------------------------------|
| Shared project behavior | `fuku.yaml`, committed for the team       |
| Local machine behavior  | `fuku.override.yaml`, untracked and small |
| A separate setup        | A file that the user gives as `config`    |
| A temporary experiment  | `fuku.override.yaml`, restored at the end |

## Keep the override small

Add only the keys that differ from the base config.

- A mapping merges by key
- A scalar, or a value of another kind, replaces the base value
- A sequence adds its items after the base items. It never replaces them
- An explicit `null` deletes the key

A repeated profile list cannot make the list shorter. To remove a service on this machine, use `exclude`, or add a
local profile with its own list.

## The keys an agent edits

- `services.<name>.dir`: the working directory. The default is the service name
- `services.<name>.command`: the command that fuku runs through `sh -c`. The default is `make run`
- `services.<name>.tier`: the startup group. Tiers start in the order of their first appearance in the file
- `services.<name>.readiness`: `type` is `http` with `url`, `tcp` with `address`, or `log` with a regexp `pattern`
- `services.<name>.watch`: `include` globs are required. `ignore`, `shared` and `debounce` are optional
- `services.<name>.logs.output`: `stdout`, `stderr` or both
- `profiles.<name>`: `"*"` for all services, or a list of service names from `services`
- `exclude`: the service names that fuku removes from every profile on this machine
- `server.listen`: the API address. The default is `127.0.0.1:3858`. `""` or `none` turns the API off.
  Any other value is `host:port` with a loopback IP such as `127.0.0.1` or `::1`, or `localhost` or `ip6-localhost`
- `server.auth.token`: optional. When set, every API call except `/live` and `/ready` must send it

A service with no keys, such as `api:` alone, is not valid.
`env.files` only supplies the env tab of the terminal UI. fuku does not send these values to the service process.

## What doctor reports

- `config.validate`: a schema error
- `topology.profile`: an unknown profile
