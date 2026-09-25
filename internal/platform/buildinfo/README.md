# platform/buildinfo

The application name and version, read everywhere a report or a client needs to say what build this is.

## How it works

`AppName` is `"fuku"`. `Version` is a bare semver string (`X.Y.Z`, no leading `v`).

## Using it

Telemetry, the REST API, the doctor report, the update check and `fuku version` all read the same two constants.

## Changing it

- `Version` is bumped by hand for each release.
  `app/updater` compares it against the latest GitHub release with `golang.org/x/mod/semver`.
  `semver.Compare` ranks an invalid string below every valid one.
  `isNewer` adds a leading `v` to both versions. It reports no update when either one then fails `semver.IsValid`.
  So a bad bump hides the update banner instead of failing loud
