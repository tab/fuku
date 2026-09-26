# e2e

End to end tests run the built binary as a subprocess and read what it prints. Nothing here imports `fuku/internal`.

## Running

```sh
make build && make test:e2e
```

`test:e2e` uses `FUKU_BIN=$(PWD)/cmd/fuku`. A stale binary tests the previous change. Build first, every time.
`make test` and `make test:race` skip the suite. A green `make check` says nothing about it.

## Fixtures

A test names a directory under `testdata/`. It holds a `fuku.yaml` and what that config points at.
The services are the stubs in `services/`, one per readiness type: `log.go`, `http.go`, `tcp.go`.

`examples/bookstore` is not a fixture. No test reads it.

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

- `NewRunner` for a fuku that keeps running. `RunOnce` for a command that exits on its own (`doctor`, `--version`). `LogsRunner` for `fuku logs`
- `require` for what the rest of the test depends on. `assert` for the checks
- wait on a log line, never on a sleep: `WaitForLog`, `WaitForRunning`, `WaitForServiceStarted`, `WaitForTierReady`.
  Give each a timeout. Keep the total under the suite's `-timeout 5m`
- assert on event names (`tier_starting`, `service_ready`, `service=postgres`), not on prose
- `ServicePID` reads a child's PID from its `service_starting` event. `WaitForGroupExit` proves no process of its group is left
- a fixture's `.env` is ignored by `.gitignore` and its `fuku.yaml` by a global ignore. Add them to a new fixture with `git add -f`
- never `t.Parallel()`. The fixtures pin real ports. `testdata/api` and `testdata/readiness` bind `127.0.0.1:19876`. Two runners fight over them
