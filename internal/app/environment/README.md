# app/environment

The merged `.env` entries of every service, cached for the environment tab of the TUI aside.
Nothing injects them into the child process. Reading one file belongs to `adapters/envfiles`.

## How it works

The store is a consumer of the run composition. Its subscription is optional and filtered to two events.

- `ProfileResolved` reloads every service of the profile
- `ServiceStarting` reloads that one service, so a restart picks up an edit
- the files are the service's `env.files`. `adapters/config` fills the defaults
- the files merge in that order. A later file overrides an earlier key. The first occurrence keeps its position
- a file the reader cannot supply is skipped: missing, unreadable or unparsable. The tab shows what loaded
- a service without a directory has no entries

The cache is keyed by service ID.
File reads run outside the lock.

The subscription is optional on purpose. A dropped message leaves a stale tab, never a wrong fact about a service.
The aside must not be able to end the run.

## Using it

`Env(id)` returns a copy of the cached entries, or nil.
The TUI reads it through its `Environment` interface. Only the `view` block in `bootstrap/modules/run.go` wires the store. A headless run has none.

## Changing it

- the entries never reach the child. Launch environment is `adapters/process`
- keep the subscription optional and the reads outside the lock
- a different file format replaces `*envfiles.Reader` behind `Reader` in the composition. The merge rule stays here
