# Agent skill end-to-end tests plan

Feature: [feature.md](feature.md)

Status: implemented

Phase: pr review

Current step: the PR. The coverage and docs passes are done and the `Done when` checks passed on 2026-10-05.
A first sweep with `REPEATS=1` passed 13 of 14 runs. Codex S5 failed because `fuku stop` could not signal fuku from the sandbox.
The agent skill's socket stop fixed it, and Codex S5 then passed 3 of 3.
The full sweep on 2026-10-05 passed 42 of 42 runs, and each scenario's budgets are now about twice its worst run.
After the code gate fixes, one S1 run per host passed on 2026-10-06 with the transcript scan and the plugin fingerprint in place

## Done when

- Every acceptance criterion in `feature.md` holds
- A full manual sweep with `make build && make test:agents-e2e` meets the threshold in every scenario on Claude Code and on Codex
- Recorded host events with each forbidden command of AC6 (a foreground `fuku run`, a log follower, a whole-file config read, a kill of fuku or a service, a socket removal) and with a token each fail their check, offline, for both hosts
- `make test:agents-plugin` passes
- `npm run build` in `docs/` passes
- `git diff master --stat` shows no change under `internal/`, `cmd/` or `e2e/` outside the agent skill's socket stop and its private-directory bind

## Steps

- [x] 1. Verify the headless flags, the JSON events and a prompt-free Codex plugin install on the recorded host versions
- [x] 2. Add the fixture and the seven scenario files with their `Must`, `Never` and `End` checks, and the scenario validation
- [x] 3. Add the runner core: the fuku from `make build` on `PATH`, the temp project, the setup steps, the end-state checks and the teardown
- [x] 4. Add the Claude Code adapter
- [x] 5. Add the Codex adapter
  - A private `CODEX_HOME` with a fresh plugin install, compared with the working-tree skill
  - The copied `auth.json` removed on every exit path
- [x] 6. Add the command checks, the budgets, the hard time limit and the guards on the real `~/.claude` and `~/.codex`
- [x] 7. Add the sweep report, the pass rates and the exit codes
- [x] 8. Add `make test:agents-e2e` with `HOST`, `SCENARIO` and `REPEATS`, the offline self-checks in `make test:agents-plugin` and the folder README
- [x] 9. Make the skill text changes K1 to K3, with the docs page's log command and the structure check kept in step
- [x] 10. Run a first sweep and set each scenario's budgets from its medians
  - The full sweep on 2026-10-05 passed 42 of 42 runs on both hosts
  - Each budget is about twice the worst run of that sweep, not twice the median, so a slow run still fits
- [x] 11. Run the full sweep on both hosts
  - The 42-run sweep of step 10 ran under the placeholder budgets. Every one of its runs is inside the new budgets, so it was not rerun

## Gates

- [x] Plan review – PASS WITH FOLLOW-UPS, round 2, 2026-10-04; PLAN-3 fixed by covering every AC6 command in the offline proof
- [x] Code review – PASS, round 4, 2026-10-06; CODE-1 to CODE-3, CODE-5 and CODE-6 fixed in one round, CODE-4 in three; the rechecks ran on gpt-6-luna
