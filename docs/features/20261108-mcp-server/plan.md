# MCP server plan

Feature: [feature.md](feature.md)

Status: implemented

Phase: pr review

Current step: the PR

## Done when

- Every acceptance criterion in `feature.md` holds, and each step below owns the ones it names
- `make check`, `make test:race`, `make test:e2e`, `make test:agents-plugin`, `make docs`, `npm run build` in `docs/` and the
  depguard audit pass
- A Claude Code session and a Codex session with the plugin installed list the six fuku tools

## Steps

- [x] 1. Settle wait in the core, exposed as `wait` on the three REST actions (AC10)
- [x] 2. The API address in the log socket `status` frame and its client (AC11)
- [x] 3. `fuku mcp`: the command, its composition and `adapters/mcp` serving `tools/list` over stdio (AC1, AC2)
- [x] 4. Read tools `status`, `logs` and `doctor`, with the per-call config load (AC3, AC4, AC9)
- [x] 5. Write tools `run`, `stop` and `service` (AC5–AC8, AC6a)
- [x] 6. stderr: the startup banner and one line per tool call (AC13, AC14)
- [x] 7. Agent plugin: the MCP config for Claude Code and Codex, the reduced skill, `control-api.md` and `api.sh` removed
- [x] 8. E2E: the Go e2e of `fuku mcp`; the agent scenarios graded on MCP tool calls, the shell parser removed (AC12)
- [x] 9. Docs: `README.md`, `ARCHITECTURE.md`, the `adapters/mcp` README and the agents plugin page
- [x] 10. Sweep 1 fixes: the profiles in the no-instance error, `unknown` on a broken config, no whole-config reads (AC5, AC6a, AC9)
- [x] 11. Harness: Codex approves the fuku tools, `skill_loaded` on S5 only, the guard ignores the host's usage files (AC12)
- [x] 12. The admission time stamped in `Guard.admit` is the REST wait's `since`, keeping `Control.Wait` (AC10)
- [x] 13. Second full live sweep, its results sent for an independent review (AC12)
- [x] 14. Sweep leftovers: the skill's `config` line, the marketplace refresh files ignored, the `projectDir` README sentence (AC12)
- [x] 15. Fewer round trips, CODE-1: `run` returns the service states, `ip6-localhost` maps to `::1`, `doctor` only after a
  failed `run` or a config edit; a third sweep (AC5, AC12)
- [x] 16. The REST `wait` moves from the query to the JSON body `{"wait": <seconds>}`; an STE pass on the skill (AC10)
- [x] 17. `watching` in the `status` and `service` results, the watch rule in the descriptions; scenario S8 (AC3, AC6, AC12)
- [x] 18. The REST client moves into `adapters/rest`; `mcp` calls it through its own `API` interface (AC3, AC5, AC6)
- [x] 19. The sweep refuses a host older than the oldest version its adapters were verified on (AC12)
- [x] 20. The agent works next to the developer's own fuku without guessing (AC3, AC6, AC11, AC12)
  - `mode` in the status frame and the `status` result; a config change restarts only a detached fuku, otherwise the agent asks
  - After a code edit, the agent waits in `status` for a new PID or a failure of the watching service before it tests
  - fuku keeps running at the end of a task, `stop` only when the user asks; S9: a config change under a foreground fuku
- [x] 21. The grader reads a config name in `grep` pattern position as a pattern, not a file; found by the fourth sweep (AC12)
- [x] 22. The grader reads attached and clustered `grep` pattern options (`-efoo`, `--regexp=foo`, `-vefoo`) (AC12)
- [x] 23. Experiments live in `fuku.override.yaml` and are restored at the end; S10 under a detached fuku (AC12)

## Gates

- [x] Plan review – PASS (round 2, codex, gpt-6.1-sol, high)
- [x] Code review – round 15 PASS (claude, fable, high)
