# Code review

Mode: standard  
Gate: code  
Status: passed  
Round: 4  
Target: `8c9446e..cdceca9` on `feature/skill`  
Reviewer: codex  
Model: gpt-6.1-sol  
Effort: high

## Findings

## Resolved

### CODE-2 – Restrict the socket before it starts listening

Severity: major  
Status: resolved  
Location: `internal/adapters/logsocket/server.go:167`

Finding: `net.Listen` binds and starts listening before `os.Chmod` restricts the socket to `0600`. With a permissive umask, such as `000`, another local user can connect during that interval. The connection remains queued after chmod, and `handleConnection` accepts its `stop` frame without checking peer identity. That user can therefore stop the legitimate fuku instance and all its services, bypassing the owner-only protection required by step 10. The existing mode test sets umask `000` but checks permissions only after startup completes.

This is distinct from BL-025: the attacker connects to the legitimate server rather than impersonating it.

Suggested fix: Bind the socket, restrict it to `0600`, and only then start listening; alternatively, authenticate the peer UID before accepting a stop request. Add regression coverage for the startup interval, beyond checking final permissions.

Reply: Fixed by e716c12. The server binds on a `0600` socket inside a fresh `0700` directory `<socket>.d`, then renames it to the project path, so the listening socket is never reachable by another user. `Test_bind_PrivateWindow` runs under umask `000` and checks the directory holds only the `0700` private directory and that the socket is `0600` before the rename; a mutant that restricts after the rename fails it. A two-user run on Linux refused the second user during the window and on the final socket. On macOS only the owner side was run.

Recheck: Resolved. In `e716c12`, `bind` creates the `0700` private directory, listens inside it, and applies `0600` before `listen` renames the socket to the public path. The private directory is removed on every outcome. The startup-window test and reported mutants and two-user Linux run support the fix.

## Checked

- Read the focused recheck rules and inspected `e716c12`'s changes to `server.go`, `server_test.go` and the logsocket README.
- Confirmed the private directory is removed after the bind attempt, rename failures close the listener, and the existing shutdown path closes the listener and removes the public socket. The private directory does not match the stale socket sweep pattern.
- Accepted the primary agent’s reported checks at `e716c12`: `make fmt`, `make lint` (0 issues), `make test`, `make test:e2e`, and `make docs` passed; `go test -count=1 -race` passed for logsocket, detach and instance.
- Accepted the reported step reviewer mutants as killed: no private directory, `mkdir 0755`, no leftover cleanup, no deferred `RemoveAll`, and chmod after rename.
- Accepted the reported two-user Linux run: the second user was refused during the private window and at the final socket. On macOS, only the owner side was run.
- The single-instance guard’s flock is taken before the server starts; `Coordinator.Start` performs the guard check before producers.
- Excluded uncommitted changes under `plugins/agents/`.

## Verdict

PASS