# REST API defaults plan

Feature: [feature.md](feature.md)

Status: implemented

Phase: pr review

Current step: the PR

## Done when

- Every acceptance criterion in `feature.md` holds, and each step below owns the ones it names
- `make check`, `make test:race` and `make test:e2e` pass
- `make test:agents-plugin`, `make lint:plugin`, `make docs` and `npm run build` in `docs/` pass
- `golangci-lint run --enable-only depguard ./...` prints `0 issues.`
- A manual `fuku run -d` in a project without a `server` block prints `API 127.0.0.1:<port>` with a port in 3858–3867,
  and `curl` of `/api/v1/status` at that address answers `200` without a token

## Steps

- [x] 1. Config: the default address, the off spellings, the env flag and the optional token
  - Files: `config.go` (pre-fill in `defaultConfig`), `constants.go`, `loader.go` (`initConfig`), `validate.go`, `errors.go`, `project.go`, and their tests with `merge_test.go`.
    `none` and the env flag become an empty `Listen` in `project.go`, after validation. `run.go`, `model` and `adapters/rest` stay untouched
  - Done when: AC1–AC11 pass as unit tests. Checks: `make fmt`, `make vet`, `make lint`, `make test`
- [x] 2. REST: the browser guard and the optional token
  - Files: `middleware.go` (`guardMiddleware` and its loopback-host predicate), `errors.go` (`forbidden`), `server.go`, `middleware_test.go`, `server_test.go`.
    The guard wraps the whole mux inside `telemetryMiddleware`, so it covers the probes and a rejection is still published. An empty `Options.Token` mounts the authed mux without `authMiddleware`. An `Origin` present with an empty value is still an `Origin`
  - Done when: AC13–AC17 pass as unit tests. Checks: `make fmt`, `make vet`, `make lint`, `make test`
- [x] 3. E2E: the default address, the off switch and the Origin rejection
  - Files: a new `testdata/api-default/` with no `server` block, its scenarios in `api_test.go`, and an env-off case in `detached_test.go`.
    Leave the token fixtures `api`, `concurrency`, `detached` and `readiness` as they are, so AC13 keeps its e2e coverage.
    `api-default` is the one fixture that walks from 3858 instead of a pinned port; one line in `e2e/CLAUDE.md` says so
  - Done when: AC12 and AC18 pass, and the Origin scenario backs AC17. Checks: `make build`, `make test:e2e`
- [x] 4. Agent skill: the API on by default, no token needed, `api.sh` without a header
  - Files: `SKILL.md`, `references/control-api.md`, `scripts/api.sh`, `tests/test_plugin.py`.
    Leave `e2e/checks.py` and `tests/test_e2e_checks.py`: their `listen:` greps are look-alikes that must still pass, not guidance
  - Done when: AC21–AC23 hold. Checks: `make test:agents-plugin`
- [x] 5. JetBrains: the default port 3858
  - Files: `Settings.kt`, `ApiClient.kt`, `docs/src/pages/plugins/jetbrains.astro`, `plugins/jetbrains/CLAUDE.md`. Leave `gradle.properties`: the release bumps it
  - Done when: AC24 holds. Checks: `make lint:plugin`, `cd docs && npm run build`
- [x] 6. Docs and the spec
  - Files: `README.md`, `.claude/skills/config/SKILL.md`, `api.astro` (keypad sentence, playground base URL), `configuration.astro`, `plugins/agents.astro`,
    `spec/openapi.yaml` (`servers`, the `403`, an optional bearer), `spec/bruno-collections/environments/localhost.bru` (3858), the `rest` and `config` READMEs. `ARCHITECTURE.md` describes no opt-in API, so it stays
  - Done when: AC19 and AC20 hold. Checks: `make docs`, `cd docs && npm run build`, `.github/scripts/openapi.sh master`

## Gates

- [x] Plan review – PASS WITH FOLLOW-UPS, round 1, standard, codex gpt-6.1-sol, high. PLAN-1 (medium): the manual check accepted only 3858 – fixed, it accepts the walk
- [x] Code review – PASS, round 4, standard, codex gpt-6.1-sol, high. Round 1 PASS WITH FOLLOW-UPS: CODE-1 and CODE-2 (medium) fixed by 7a413e6. Round 3 CHANGES NEEDED: CODE-3 (major, the plugin can control another project) accepted as a risk by the user, tracked as BL-031
