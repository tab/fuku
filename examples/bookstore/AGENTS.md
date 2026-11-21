# examples/bookstore

Six fake services the root `fuku.yaml` runs. `fuku` from the repository root starts them all.
This is how a new feature gets tried by hand.

None of it is real. Each service prints canned log lines and opens a port where the config asks for one.

## It is a separate module

`go.mod` here declares `module examples/bookstore`. Nothing under this tree is in `go list ./...`.
`make check`, `make lint`, `make test` and CI skip it. A break here shows up the next time somebody runs `fuku`.
Build it yourself after a change.

The e2e suite has its own stubs in `e2e/services/` and its own configs in `e2e/testdata/`.

## The wiring

The root `fuku.yaml` names each service by directory. It gives each a tier and a readiness probe:

| service           | tier       | readiness   |
| ----------------- | ---------- | ----------- |
| `auth`, `storage` | foundation | log pattern |
| `user`, `worker`  | platform   | tcp         |
| `api`             | platform   | http        |
| `frontend-api`    | edge       | http        |

Each service is a `make run` in its own directory. It builds `src/main.go` and runs it.
The behaviour is in `pkg/common`: `common.Run(cfg)` takes a name, an optional `HTTPPort` and `TCPPort`, and the lines to print.
It prints `Service ready` last. The log readiness probe matches on that.

`failed-service` has no `src/` on purpose. Its `Makefile` exits 1. That exercises the failure path.

## Adding a service

Copy a directory. Keep the `Makefile` with its `run` target. Point `main.go` at `common.Run`.
Add it to the root `fuku.yaml` with a tier and a probe. Ports are written in both places, so pick a free one.
