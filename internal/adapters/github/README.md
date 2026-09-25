# adapters/github

The release source of the update check: one GET against the GitHub releases API behind a daily file cache.
`*github.Client` is bound to `updater.ReleaseSource` in `bootstrap/modules/runtime.go`. The version comparison stays in `app/updater`.

## How it works

`Latest(ctx)` returns the cached tag while the cache is fresh. Otherwise it fetches and caches:

- endpoint `https://api.github.com/repos/tab/fuku/releases/latest`, 3s timeout
- headers `Accept: application/vnd.github+json` and `User-Agent: fuku/<version>`
- reads `tag_name`
- rejects an empty tag and a non-2xx status. Whether the tag is a newer semver release is `app/updater`'s rule

The cache is `$UserConfigDir/fuku/version.json` with the keys `tag` and `fetched_at`:

- an entry older than 24 hours, an empty tag or a zero time is a miss
- a missing file is a miss without an error
- an unreadable file is logged at debug and treated as a miss
- the file is written with `0600` in a `0700` directory
- an empty `CachePath` disables the cache

## Changing it

- every IO failure stays a returned error. The caller decides the check is best effort
- the request goes through `HTTPDoer`, so a test never reaches the network. Keep the timeout on the client
