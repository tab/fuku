# Code review

Mode: standard
Gate: code
Status: passed
Round: 15
Target: step 23 fixes `729a590..bc313d8` on `feature/mcp`; full feature target `master...bc313d8`, merge base `bcd0923`
Reviewer: claude
Model: fable
Effort: high

## Findings

None open.

## Resolved

- CODE-1 – Older instances using ip6-localhost cannot be inspected or controlled (medium, round 2, `internal/adapters/rest/client.go:63`, first `mcp/api.go:118`) – resolved: `50afcaa` maps `ip6-localhost` to `::1` before the loopback check; pinned by `Test_NewClient/ip6-localhost_maps_to_::1` and `Test_Client_Live_IP6Localhost`
- CODE-2 – No test pins the unit of `wait` (medium, round 5, `internal/adapters/rest/handler_test.go:39` over `handler.go:199`) – resolved: `fbbea41` asserts the deadline in seconds; pinned by `Test_HandleAction`
- CODE-3 – S8's `project_e2e_passes` can pass without the agent applying the edit (medium, round 7, `plugins/agents/e2e/runner.py` `check_project_e2e`, `S8.json`) – resolved: `40d414b` runs the fixture's `e2e.sh` and adds `service_applied:api`; pinned by `test_project_e2e_check` and `test_shipped_scenario_command_checks [S8 …]`
- CODE-4 – S9 passes when the agent restarts the developer's `api` service (medium, round 10, `S9.json`) – resolved: `209defc` adds `no_api_restart` to S9 and pins the `noteMode`/`noteWatching` phrases; pinned by `test_shipped_scenario_command_checks [S9 restart of api]` and `Test_NewServer`
- CODE-5 – `no_config_read` fails a compliant S10 run that appends the experiment with a `cat` heredoc (major, round 14, `plugins/agents/e2e/checks.py` `simple`) – resolved: `bc313d8` adds `unredirected`, which drops the target of a bare output operator and keeps the file a bare `<` reads; pinned by the five heredoc and stdin rows of `test_recorded_events`
- CODE-6 – S10 passes an agent that restores the override file but never applies the restore (medium, round 14, `S10.json`, e2e `README.md`) – resolved: `bc313d8` adds `run_detached_twice` to S10 and corrects the README; pinned by `test_scenario_checks` and `test_shipped_scenario_command_checks [S10 experiment without the restore apply]`
- CODE-7 – `config_unchanged` passes an agent that wrote the experiment into `fuku.yaml` and restored it byte for byte (medium, round 14, `plugins/agents/e2e/runner.py` `check_config_unchanged`) – resolved: `bc313d8` records `Run.config_mtime` after the setup and fails on a moved mtime or a leftover `fuku.yaml.*`; pinned by `test_config_unchanged_sees_a_rewrite`, the leftover row of `test_config_file_checks` and `FukuBackedTest.test_state_checks [S10 restarted and restored]`

Notes for a future reader:

- Round 13 grader mutants M2 (`search_words` cluster read), M5 (`search_options` `SEARCH_VALUES` guard) and M6 (`rstrip("=")`) survive the tests; each is killed by one `no_config_read` row on `grep -nA 2 . fuku.yaml`, `grep --color=always . fuku.yaml` or `rg --max-count=1 . fuku.yaml`
- `-T` sits in `SEARCH_VALUES` as rg's `--type-not`, so `grep -vT x fuku.yaml` is not flagged although grep reads `-T` as a flag; `grep -e -v fuku.yaml` is a pre-existing false positive. Neither form is one an agent writes
- `run_detached_twice` is exact: a third successful `run` of the profile fails S10, by the developer's choice to mirror `run_detached_once`
- `config_unchanged` compares `st_mtime_ns`, so it relies on the file system's timestamp resolution (APFS: nanoseconds)
- S10 has never been swept; the paid `make test:agents-e2e` sweep needs the user's approval
- `make test:agents-plugin` skips the 7 fuku-backed tests without `FUKU_BIN`; run `make build && FUKU_BIN=$PWD/cmd/fuku make test:agents-plugin`

## Checked

- Round 15: `git diff 729a590 bc313d8` (7 files, +110/−20, no Go) against the three suggested fixes and the recheck rules; the round 14 stand-in agents through `runner.run_scenario(S10, …)` with the built fuku now fail on `run_detached_twice`, `config_unchanged` "written after the setup" and "left behind: fuku.yaml.bak", and the compliant heredoc run passes
- `FUKU_BIN=$PWD/cmd/fuku make test:agents-plugin` 124 tests OK, none skipped; `make docs` "All links resolve."; `git diff --check 729a590 bc313d8` clean; 8 mutants on the three fixes in a `git archive bc313d8` copy, all killed
- Not run: `make check`, `make test:race`, `make test:e2e` (no Go in the range), `npm run build` (no `docs/src` change, 22 pages in round 14), the paid S10 sweep

## Verdict

PASS
