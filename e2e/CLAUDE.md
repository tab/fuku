# e2e

End to end tests drive the built binary as a subprocess and read what it prints.
Nothing here imports `fuku/internal`.

## Running

```sh
make build && make test:e2e
```

`test:e2e` passes `FUKU_BIN=$(PWD)/cmd/fuku`, so a stale binary quietly tests the
previous change. Build first, every time. The suite is excluded from `make test`
and `make test:race` (`go list ./... | grep -v /e2e`), which means a green
`make check` says nothing about it.

## Fixtures

A test names a directory under `testdata/`, which holds a `fuku.yaml` and
whatever that config points at. The services those configs run are the stubs in
`services/`, one per readiness type: `log.go`, `http.go`, `tcp.go`.

`examples/bookstore` is the playground the root `fuku.yaml` drives by hand. It is
not an e2e fixture, and no test reads it.

## Writing a test

`Test_<Area>_<Behaviour>`, one runner per test, stopped on the way out:

```go
func Test_Tier_StartsInOrder(t *testing.T) {
	runner := NewRunner(t, "testdata/tier")
	defer runner.Stop()

	require.NoError(t, runner.Start("default"))
	require.NoError(t, runner.WaitForRunning(30*time.Second))

	output := runner.Output()
	assert.Contains(t, output, "service_ready")
}
```

- `NewRunner` for a fuku that keeps running, `RunOnce` for a command that exits
  on its own (`doctor`, `--version`), `LogsRunner` for `fuku logs`
- `require` for anything the rest of the test depends on, `assert` for the
  checks themselves
- wait on a log line, never on a sleep: `WaitForLog`, `WaitForRunning`,
  `WaitForServiceStarted`, `WaitForTierReady`. Give each an explicit timeout,
  and keep the total under the suite's `-timeout 5m`
- assert on the structured event names (`tier_starting`, `service_ready`,
  `service=postgres`), not on prose a wording change would break
- never `t.Parallel()`. The fixtures pin real ports, `testdata/api` binds
  `127.0.0.1:19876`, and two runners at once fight over them
