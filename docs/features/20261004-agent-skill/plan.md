# Agent skill plan

Feature: [feature.md](feature.md)

Status: in progress

Phase: code review

Current step: step 8, the human's manual install and session in Claude Code and Codex; then prepare the PR

## Done when

- Every acceptance criterion in `feature.md` holds
- `make check`, the race tests and the e2e suite pass
- `make test:agents-plugin` passes locally and as a CI job
- `npm run build` in `docs/` passes
- A manual install in Claude Code and in Codex lists the `fuku` skill
- A manual session in each host, on `examples/bookstore`, starts a profile with `-d`, reads the logs of one service and stops fuku
- `git diff master --stat` shows no Go file outside the two stop changes and their tests

## Steps

- [x] 1. Verify the Codex plugin and marketplace paths against the current Codex documentation
- [x] 2. Make `fuku stop` fail without cleanup when it cannot reach the running fuku
  - Exit 1, no directory cleanup and no socket removal. Unit tests, and an e2e scenario when the suite can block the socket
- [x] 3. Write `plugins/agents/skills/fuku/SKILL.md` against master's `fuku help` and `spec/openapi.yaml`
  - Order: resolve the setup, attach before starting, run with `-d`, read logs, service state and actions, config changes, stop, sandbox limits, report
  - Take the still-valid guidance from `origin/feature/skill-work-in-progress`, and drop the helper, the follower and the API-only guard text
- [x] 4. Write the two references and `scripts/api.sh`
- [x] 5. Add the Claude Code and Codex plugin manifests, the logo and the two marketplace files
- [x] 6. Add the structure check with the script fixtures, `make test:agents-plugin` and the CI job in `checks.yaml` and `master.yaml`
- [x] 7. Add the docs page, the plugins index link, the nav entry and the `README.md` section
- [ ] 8. Run the manual install and the manual session in both hosts
- [x] 9. Let the agent stop or restart fuku whoever started it, approved by the human after the code gate
  - Restart the whole profile without asking when the API is off, and treat a request to apply a config change as the ask
  - Checks: `make test:agents-plugin`
- [x] 10. Make `fuku stop` ask the running fuku to stop over its socket, with the socket at mode `0600`, approved by the human after the code gate
  - A fuku that does not echo the stop frame gets `SIGTERM`, as before
  - Checks: the `detach` and `logsocket` unit tests, the e2e `Test_Stop_StopsTheRunningInstance`, and Codex S5 of the agent e2e sweep, 3 of 3

## Gates

- [x] Plan review – PASS, round 1, 2026-10-04
- [x] Code review – PASS, round 4, 2026-10-06; CODE-2 fixed by the private-directory bind; the round 4 recheck ran on gpt-6-luna
