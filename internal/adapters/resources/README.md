# adapters/resources

Samples CPU and memory. The service readings feed the registry. The fuku process reading feeds telemetry.

## How it works

`ProcessMonitor` reads one PID through gopsutil.
CPU is the delta of the process time since the previous read of the same PID, so the first read of a PID is 0.
A PID that died or was reused loses its history. A read that fails clears it too.

`Sampler` is a producer of the run composition. On its own goroutine:

- every 2 seconds it reads every service with a live PID from the registry snapshot, 200 ms per read, and publishes one `ServiceResourcesSampled` batch
- every 5 minutes it reads the fuku process and publishes `ResourceSampled`, only when telemetry is on

Both publishes are non-critical. A full queue drops them. The next tick brings a fresh reading.

## Using it

The composition binds the registry store as `Registry` and projects `Options{Enabled}` from the telemetry options.
It binds the monitor named `sampler` as `Monitor`. The package's `Module` binds nothing.
Two monitors exist: one named `sampler` for the `Sampler`, one for the TUI, which reads the fuku process at its own cadence for the header.
Each keeps its own CPU history, so the two cadences do not skew each other's delta.

## Changing it

- the cadences and the per-read bound are constants here. They are technical rules of the adapter
- `Stats` carries what the registry and the TUI show. A new reading is a field here and a field on the sample event
- a monitor is per consumer. Sharing one across cadences makes the CPU delta wrong
