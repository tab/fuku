# Single-instance guard plan

Feature: [feature.md](feature.md)

Status: implemented

Phase: code review

Current step: approve code review before PR preparation

## Approach

Add a focused probe to `internal/app/instance` that checks `GET /api/v1/live` on the configured port and its nine fallbacks.
It returns the first address where `product` is `fuku` and `fingerprint` matches the current project.
Keep generic instance listing out of this feature.

Keep `instance.Identity` and the guard as FX-provided dependencies.
`instance.Guard` and its mock sit beside the implementation, the way `api.Listener` and `dotenv.Loader` do.
Register the hook from `app.RegisterGuard`, next to `RegisterAPI`, because `instance` cannot import `internal/app/cli` for the command gate without the cycle `cli` → `ui/wire` → `api` → `instance`.
Declare `fx.Invoke(RegisterGuard)` ahead of `fx.Invoke(Register)` and `fx.Invoke(RegisterAPI)` in `app.Module`: FX runs invocations in declaration order, so its synchronous `OnStart` hook finishes before the application and the API start. The dotenv, metrics, relay, registry, sampler and tracer hooks that run earlier only subscribe to the bus and wait for events, so none of them touches the project's services.
A confirmed match writes the refusal to an injected stderr stream and returns `ErrInstanceAlreadyRunning`.
`runFxApp` returns exit code 1 when `application.Start` reports the failed `OnStart` hook, so `cmd/main.go` needs no direct guard or identity call.

Probe only `run` commands when `server.listen` is enabled.
Use the existing ten-port range, a 250 ms HTTP timeout and a 4096-byte response limit.
Disable HTTP redirects so a response cannot move the probe outside that range.
Treat every response except a confirmed project match as no match.

## Steps

- [x] Add the bounded same-project liveness probe under `internal/app/instance` – AC1, AC4, AC6 and AC7
- [x] Add the probe limits and `ErrInstanceAlreadyRunning` – AC2 and AC6
- [x] Provide the guard through FX and inject the existing identity instead of constructing or calling dependencies in `cmd/main.go` – AC5 and AC8
- [x] Register the guard before every hook-registering module and return `ErrInstanceAlreadyRunning` on a match – AC1 and AC3
- [x] Inject a named stderr writer and print the matched address and `fuku logs` or stop guidance before returning the error – AC2
- [x] Add unit tests for matching, redirects, bounds, command selection, DI, visible output and complete lifecycle ordering – AC1, AC2, AC4, AC5, AC6, AC7 and AC8
- [x] Add an end-to-end test that keeps the first process and its service PIDs unchanged after a refused second run – AC3
- [x] Add the guard to the README feature list and the architecture startup flow – AC2 and AC3

## Verification

- `make fmt` – clean
- `make vet` – clean
- `golangci-lint run --new-from-rev origin/master` – 0 issues
- `make test` – all packages pass; `internal/app/instance` guard functions cover 92-100%
- `make test:race` – all packages pass, no races
- `make build` – clean
- `make test:e2e` – full suite passes, including `Test_Instance_RefusesSecondRun`
- `npm run build` in `docs/` – not needed, no documentation site files changed
- Manual check: covered by `Test_Instance_RefusesSecondRun`, which drives the built binary through the same scenario (start the API-enabled profile, run it again from the same project, confirm exit code 1, the refusal on stderr, unchanged service PIDs and a still-running first instance)

`make lint` over the whole repository also reports 54 pre-existing issues in files this feature does not touch, all raised by the newer local golangci-lint 2.12.2 rather than the 2.11.3 pinned in CI.

## Gates

- [x] Plan review – PASS (stress: general + startup process safety)
- [x] Code review – PASS (stress: general + startup process safety)
- [ ] PR review – not run
