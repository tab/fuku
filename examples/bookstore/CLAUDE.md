# examples/bookstore

Six fake services the root `fuku.yaml` orchestrates. This is the playground for
running fuku by hand: `fuku` from the repository root starts the whole set, which
is how a new feature gets tried against something that behaves like a real stack.

None of it is real. Each service prints canned log lines and, where the config
asks for it, opens a port. They exist to give tiers, readiness probes, profiles
and log streaming something to act on.

## It is a separate module

`go.mod` here declares `module examples/bookstore`, so nothing under this tree is
in `go list ./...`. `make check`, `make lint` and `make test` at the root all
skip it, and CI never compiles it. A break here surfaces the next time somebody
runs `fuku`, not as a red pull request. Build it yourself after changing it.

The e2e suite does not read this tree either. It has its own stubs in
`e2e/services/` and its own configs in `e2e/testdata/`.

## The wiring

The root `fuku.yaml` names each service by directory and gives it a tier and a
readiness probe:

| service | tier | readiness |
| --- | --- | --- |
| `auth`, `storage` | foundation | log pattern |
| `user`, `worker` | platform | tcp |
| `api` | platform | http |
| `frontend-api` | edge | http |

Each service is a `make run` in its own directory, which builds `src/main.go` and
execs it. The behaviour is all in `pkg/common`: `common.Run(cfg)` takes a name,
an optional `HTTPPort` and `TCPPort`, and the lines to print. It prints
`Service ready` last, which is what the log readiness probe matches on.

`failed-service` has no `src/` on purpose. Its `Makefile` prints an error and
exits 1, which is how the failure path gets exercised.

## Adding a service

Copy a directory, keep the `Makefile` with its `run` target, point `main.go` at
`common.Run`, then add it to the root `fuku.yaml` with a tier and a probe. Ports
are written in both places, so pick one nothing else here uses.
