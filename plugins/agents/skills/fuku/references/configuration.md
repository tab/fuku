# fuku configuration workflow

Read this reference before creating or changing fuku config

## Choose the config file

| Goal                      | File                             | Behavior                                    |
|---------------------------|----------------------------------|---------------------------------------------|
| Shared project behavior   | `fuku.yaml` or `fuku.yml`        | Commit the change for all developers        |
| Local machine behavior    | `fuku.override.yaml` or `.yml`   | Merge a small untracked delta over the base |
| Separate standalone setup | A file passed through `--config` | Load only that file and disable overrides   |

Use `fuku.yaml` before `fuku.yml` when both base names exist
Use `fuku.override.yaml` before `fuku.override.yml` when both override names exist
An override without a base config is ignored

Do not put a local debugger command, machine path, port or token in the shared base config unless the user explicitly wants a shared change

Before any local-only edit, check whether the override is tracked and ignored
If it is tracked, explain that the change will appear in Git and ask before editing it
Never stage a local-only override change

Before writing a token or another secret to an override, confirm that the file is untracked and ignored
If it is not ignored, stop before storing the secret and ask whether to add a repository ignore rule or use another local secret source

## Keep overrides small

Add only the keys that differ from the base config
fuku merges the override with these rules:

| Value type       | Merge behavior                                               |
|------------------|--------------------------------------------------------------|
| Scalar           | Override replaces the base value                             |
| Map              | Keys merge recursively                                       |
| Array            | Override items append after base items without deduplication |
| Null             | Key is removed before runtime defaults are applied           |
| Mismatched types | Override replaces the base value                             |

Because arrays append, do not repeat an existing item and expect replacement
Use `exclude` to remove a service locally or add a new local profile when an exact service list is needed

## Bookstore examples

Add a shared profile to `fuku.yaml`:

```yaml
profiles:
  agent: [api, storage]
```

Extend an existing profile locally in `fuku.override.yaml`:

```yaml
profiles:
  minimal: [worker]
```

Run a smaller local setup without changing the shared profile:

```yaml
exclude:
  - worker
```

Use a local debugger command and API endpoint:

```yaml
services:
  api:
    command: "dlv debug ./cmd/main.go"

server:
  listen: "localhost:1234"
  auth:
    token: "replace-with-a-local-token"
```

Common configuration areas are:

- `services` for directories, commands, tiers, readiness, logs, watch rules and displayed env files
- `profiles` and `defaults.profiles` for service selection
- `exclude` for local service removal
- `concurrency` and `retry` for startup behavior
- `logs` and `logging` for stream history and fuku output
- `server` for the loopback control API

`env.files` lists values for the TUI and does not export them to the child process
Use the service's own environment loading when runtime variables must change

## Validate and apply

Validate default discovery and its override:

```bash
fuku doctor <profile> --json
```

Validate an explicit config without applying an override:

```bash
fuku --config fuku.failed.yaml doctor minimal --json
```

fuku does not reload config in a running profile
API start and restart actions use the config already held by that fuku process

To apply a config change:

1. Gracefully stop the owned `fuku run` process and wait for its services to stop
2. If there is no owned process handle, use `fuku stop <profile>` only when the user requested the restart
3. Pass the same `--config <path>` to stop when the profile used an explicit config
4. Run the same profile again with `--no-ui` and the same explicit `--config` path when one was used
5. Check the final service state and focused logs

Do not use unordered per-service API calls as a substitute for a profile restart
