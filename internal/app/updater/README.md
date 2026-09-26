# app/updater

The one-shot release check of `fuku run`.
It decides whether the check runs and whether the latest release is newer than the running build. It publishes `UpdateAvailable`.
Fetching and caching belong to `adapters/github`.

## How it works

The checker is the last producer of `fuku run` under the TUI. `Start` runs the check on its own goroutine.
`Stop` cancels an unfinished check and waits for it. A run with `--no-ui` never starts it.

- `Options.Enabled` is false (`FUKU_UPDATER_DISABLED=1`): the check returns at once
- the source fails: logged at debug and swallowed. The check is never fatal
- the tag is not newer, or not valid semver on either side: nothing is published

The `v` prefix is normalized before the comparison.

## Using it

Nothing calls this package. The TUI bridge forwards `UpdateAvailable` to the view. The view shows the version in the header.
A message published before the view attaches waits in the bridge's queue.

The running version arrives as `Options.Version`, projected from `buildinfo.Version` in `bootstrap/modules/runtime.go`.

## Changing it

- the check stays best effort: no retry, no user-visible error, no delay of the run
- a different source replaces `*github.Client` behind `ReleaseSource` in the composition. The comparison stays here
