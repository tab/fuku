# Bounded log replay plan

Feature: [feature.md](feature.md)

Status: ready

Phase: implementation

Current step: prepare the commit, then run PR review

## Approach

Extend the existing Unix socket log path.
Keep `fuku logs` streaming by default and add independent controls for replay size, stream lifetime and banner rendering.

Define `relay.ReplayOptions{Tail *int, NoFollow bool}` once and embed it in `cli.Options`, `logs.Options`,
`relay.SubscribeOptions`, the relay subscribe request, the status response and the hub's `ClientConn`.
Parse `--tail` through a custom flag value that rejects values not greater than zero at parse time and leaves the pointer
nil when the flag is omitted, so an omitted tail and `--tail 0` stay distinct without a `Changed` check.
Keep order-independent parsing but use service-first ordering in examples.

Pass command values to `logs.Screen` as `logs.Options{Profile, Services, NoUI, ReplayOptions}`.
Set `NoUI` from the existing global flag.

Pass wire options as `relay.SubscribeOptions{Services, ReplayOptions}`.
The server validates a provided tail and echoes the accepted values before hub registration.
An invalid raw request logs the error and closes without a status message.

When bounded options are requested, require the first relay message to be a status with the exact echoed values.
A missing, out-of-order or different acknowledgement returns a compatibility error with restart guidance before handler dispatch.
Requests without bounded options keep the current behavior.

Keep history access inside the hub loop.
Filter the buffered history by service before keeping the newest requested messages.
For a no-follow client, enqueue the selected replay, close its send channel and do not retain it as a live subscriber.
Let the write pump drain before closing the socket and treat EOF as normal no-follow completion.

When `--no-ui` is present, skip `RenderBanner` but keep formatted log messages, command diagnostics and errors.

## Steps

- [x] Parse and validate `--tail` and `--no-follow`, keeping flag parsing order-independent – AC2, AC3, AC4, AC5 and AC14
- [x] Pass log options and hide the banner when `--no-ui` is set – AC8, AC12 and AC13
- [x] Add `tail` and `noFollow` to relay options, requests and status messages – AC1, AC11 and AC17
- [x] Validate tail and echo accepted options before hub registration – AC17 and AC19
- [x] Require an exact status acknowledgement before handling bounded log output – AC18
- [x] Apply service filtering before one tail limit across the ordered history – AC2, AC6 and AC9
- [x] Close no-follow clients after the selected replay is drained and keep follow clients registered – AC3, AC4, AC7 and AC10
- [x] Regenerate the relay client and log screen mocks after changing their interface signatures
- [x] Add focused CLI, relay, log-screen and end-to-end tests – AC16
- [x] Update the in-binary CLI help, README, CLI docs and log-streaming docs to describe `--tail`, `--no-follow` and the
  logs use of `--no-ui`, using service-first examples – AC15

## Tests

Use table-driven tests and the repository's existing mocks-once pattern.

- `internal/app/cli/commands_test.go` – preserve omitted `tail`, reject invalid values and cover flag order, including
  `--no-ui` before and after the subcommand
- `internal/app/relay/protocol_test.go` – round-trip the optional request and status fields and preserve omitted `tail`
- `internal/app/relay/client_test.go` – accept exact status echoes and reject invalid acknowledgements before handler dispatch
- `internal/app/relay/hub_test.go` – filter before tail, preserve order, handle a large tail and close only no-follow clients
- `internal/app/relay/server_test.go` – reject invalid tail values, echo accepted options and drain replay before closing
- `internal/app/logs/screen_test.go` – pass stream options, render the banner by default and suppress it with `--no-ui`
- `internal/app/cli/tui_test.go` – pass parsed log options to the log screen
- `e2e/logs_test.go` – verify the existing follow flow and the bounded service-first command through the built binary

Regenerate the changed interface mocks with:

```bash
mockgen -source=internal/app/relay/client.go -destination=internal/app/relay/client_mock.go -package=relay
mockgen -source=internal/app/logs/screen.go -destination=internal/app/logs/screen_mock.go -package=logs
```

The main end-to-end command is:

```bash
fuku logs auth-api --profile default --no-ui --tail 1 --no-follow
```

It must omit the panel, footer and older `Starting service` line, print the newer `Service ready` line and exit successfully.
These fixture-owned messages are used because the `auth-api` filter excludes orchestration events under `fuku`.

## Documentation

Update published documentation only when the implementation exists.
Document `--tail`, `--no-follow` and the logs use of `--no-ui` in the in-binary CLI help, README, CLI docs and log-streaming docs.

Use service-first examples:

```bash
fuku logs api --tail 100 --no-follow
fuku logs api frontend-api --profile core --no-ui --tail 100 --no-follow
```

## Verification

- [x] `make fmt`
- [x] `golangci-lint run --new-from-rev origin/master`
- [ ] `make lint` – local v2.12.2 reports the existing `origin/master` baseline
- [x] `make vet`
- [x] `make test`
- [x] `make test:race`
- [x] `make build`
- [x] `make test:e2e`
- [x] `npm run build` in `docs/`

## Gates

- [x] Plan review – PASS
- [x] Code review – PASS (stress: general + relay protocol compatibility)
- [ ] PR review – not run
