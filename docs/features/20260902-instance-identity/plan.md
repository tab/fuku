# Instance identity plan

Feature: feature.md

Status: implemented

Phase: pr review

Current step: wait until commit and push are allowed to prepare PR review

## Approach

Add one small package, `internal/app/instance`, that builds an `Identity` and fingerprints a canonical directory.
Nothing else in the codebase computes either value.

`New` is the only part that touches the filesystem.
It reads the working directory, resolves it with `filepath.EvalSymlinks` and wraps either failure with `ErrFailedToResolveProject`.
`Fingerprint` hashes the path it is given and never canonicalizes, so callers pass the resolved `Project` and the hash stays a pure function.

`cmd/main.go` builds the identity once and hands it to FX with `fx.Supply`.
The API server and the relay server take `instance.Identity` as a constructor parameter, so the value arrives like every other dependency and nothing reads the working directory a second time.

The identity is created after `cli.ChangeToConfigDir`, because the resolved path only means something once the process is in the project directory.
It also comes after the doctor early return, so doctor behavior stays the same.
A resolve failure prints to stderr and returns exit code 1, like the failures around it.

## Steps

- [x] Add `ErrFailedToResolveProject` to `internal/app/errors/errors.go`, next to the other socket and instance errors – AC5
- [x] Add `internal/app/instance/instance.go` with `FingerprintLength = 16`, the `Identity` struct, `New() (Identity, error)` and `Fingerprint(project string) string`, where `New` owns `os.Getwd` and `filepath.EvalSymlinks` and `Fingerprint` hashes what it receives – AC1, AC2, AC3, AC4, AC5
- [x] Call `instance.New()` in `cmd/main.go` after the config loads, return 1 on error, and add the identity to `fx.Supply` next to `cfg` and `topology`
- [x] Take `instance.Identity` in `api.NewServer`, keep it on the struct and pass it to the handler in `Start` – AC6, AC7
- [x] Add the `identity` field to the API handler and a `LiveSerializer` with `status`, `product`, `instance` and `fingerprint` for `handleLive`, where `product` is `config.AppName`, leaving `handleReady` on `ProbeSerializer` – AC6, AC8
- [x] Add `instance` and `project` to `StatusSerializer` and fill both in `handleStatus` – AC7
- [x] Add `Instance` and `Fingerprint` fields with the `instance` and `fingerprint` JSON tags to `StatusMessage` in `internal/app/relay/protocol.go` – AC9
- [x] Take `instance.Identity` in `relay.NewServer`, store the UUID and the fingerprint and set both on the status message sent in `hello` – AC9
- [x] Add a `Live` schema for `/live` and add `instance` and `project` to the `Status` schema in `spec/openapi.yaml`, keeping the shared `Probe` schema for `/ready`; constrain `product` to `fuku`, use the UUID format for `instance`, use the lowercase 16-hex pattern for `fingerprint` and describe `project` as a canonical absolute path – AC10
- [x] Update `liveJson`, `statusJson` and the `get-live` and `get-status` descriptions in `docs/src/pages/docs/api.astro` so the published examples show the new fields
- [x] Update the relay status example and the `/live` and `/status` rows in `ARCHITECTURE.md`, which quote the exact wire format and have to show the new fields

## Tests

Table-driven with the mocks-once pattern, per `add-test`.

- `internal/app/instance/instance_test.go` – `New` fills all three fields; two identities in one directory differ by `ID` and share `Fingerprint`; `Fingerprint` matches fixed SHA-256 prefix test vectors and `^[0-9a-f]{16}$`; `New` from a symlinked spelling of a directory gives the fingerprint of the resolved directory, skipped when the platform refuses to create the symlink; a removed working directory yields a wrapped `ErrFailedToResolveProject`
- `internal/app/api/handler_test.go` – `/live` carries `alive`, `fuku`, the UUID and the fingerprint and no absolute path; `/status` carries the UUID and the canonical path; the `/ready` body is unchanged
- `internal/app/api/openapi_test.go` – reads `spec/openapi.yaml` with `go.yaml.in/yaml/v3` and asserts that the 200 `application/json` response of `/live` and `/status` references `Live` and `Status`; every serialized field is required and has the right type; `product`, `instance`, `fingerprint` and `project` carry the promised OpenAPI constraints
- `internal/app/api/server_test.go` – `NewServer` accepts the identity and reaches the handler
- `internal/app/relay/protocol_test.go` – `StatusMessage` marshals and unmarshals `instance` and `fingerprint`, including empty values
- `internal/app/relay/server_test.go` and `internal/app/relay/module_test.go` – `NewServer` accepts the identity and the `hello` status message carries both fields
- `e2e/api_test.go` – against a live process in `testdata/api`: unauthenticated `/live` returns `product` `fuku`, a parseable UUID and a 16-character hex fingerprint, `/status` returns the same UUID, and the SHA-256 prefix of the path on `/status` equals that fingerprint – AC11

Existing `api.NewServer` and `relay.NewServer` call sites in tests are updated to pass an identity.

The e2e case covers the two HTTP endpoints because they are a public contract that only holds once the identity survives FX wiring and a real process start.
The relay status message stays at unit level.

## Verification

- [x] `make fmt`
- [x] `golangci-lint run --new-from-rev origin/master` – 0 issues
- [x] `make vet`
- [x] `make test`
- [x] `make test:race`
- [x] `make build`
- [x] `make test:e2e` – includes `Test_API_InstanceIdentity`
- [x] `npm run build` in `docs/`, the only check that area has
- [x] `make lint` – 56 findings that predate this branch (50 `goconst`, 6 `nolintlint`). Clearing them is out of scope, so this feature is held to the changed-lines run above, which reports no finding on a line this branch touches

What the tests prove:

- `internal/app/api/openapi_test.go` is the evidence that the responses and `spec/openapi.yaml` agree – AC10
- The CI spec drift check stays a changed-file guard: it fails a handler change that ships without a `spec/openapi.yaml` edit, and it compares file names rather than schemas, so it is not evidence that the two agree
- `/ready` asserting its old body and the relay round-trip asserting the exact JSON together show that nothing existing was renamed or dropped

## Gates

- [x] Plan review – PASS (stress: general + API compatibility)
- [x] Code review – PASS (stress: general + API compatibility)
- [ ] PR review – not run
