# Detached run mode plan

Feature: [feature.md](feature.md)

Status: implemented

Phase: pr review

Current step: open the PR

## Done when

- Every acceptance criterion in `feature.md` holds
- `make check`, the race tests and the e2e suite pass
- E2e covers a detached start then `fuku logs` then `fuku stop`, a failed service, an existing run, a relative `--config`, `Ctrl-C` during startup and a pipe with no escape sequences
- A manual `fuku run -d` on a terminal shows the compose-style view, and `fuku stop` ends it
- The in-binary help, `README.md`, the CLI docs page and the package READMEs describe `-d` and the new stop behavior

## Steps

- [x] 1. Parse `--detached`/`-d` on the root command and `run`, and the hidden child marker
- [x] 2. Trap `SIGINT` and `SIGTERM` before `app.Start`, so a signal during startup still stops the services
- [x] 3. Report the server PID in the socket status message and read it into `contracts.LogStatus`
- [x] 4. Child: under the marker, startup events and errors go to the inherited pipe
  - Success needs the run phase and a bound log socket. A failed or unexpectedly stopped service, or a bind failure, fails the start
  - Errors before the participants start, the guard refusal included, reach the pipe as error records. No command telemetry
- [x] 5. Parent: a detached composition starts `run <profile> --no-ui --config <file>` in a new session and waits on the pipe
  - A closed pipe without a success record is a failure. Exit codes 0, 1 and 130. `Ctrl-C` sends `SIGTERM` to the child and waits for it
- [x] 6. Render the wait: the compose-style Bubble Tea view on a terminal, plain lines otherwise
- [x] 7. `fuku stop`: signal the running fuku by its reported PID, escalate after the stop timeout, then clean up
- [x] 8. E2e scenarios for the detached lifecycle
- [x] 9. Help, README, CLI docs and package READMEs

## Gates

- [x] Plan review – PASS, round 2, 2026-10-01
- [x] Code review – PASS, round 5, 2026-10-02
