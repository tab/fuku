# Code review

Mode: standard  
Gate: code  
Status: passed  
Round: 4  
Target: `ef8e6b9..cdceca9` on `feature/skill`  
Reviewer: codex  
Model: gpt-6.1-sol  
Effort: high

## Findings

### CODE-4 – Naming the Codex skill file counts as loading it

Severity: major  
Status: fixed  
Location: `plugins/agents/e2e/codex.py:100`

Finding: Codex marks the skill loaded whenever a successful shell command contains its installed `SKILL.md` path. A successful `ls <installed-path>/SKILL.md` or `echo <installed-path>/SKILL.md` produces `skill_loaded=True` without reading any instructions. Both cases reproduced with in-memory events. This defeats AC8 and permits the suite to credit behavior performed without loading the reviewed skill.

Suggested fix: Require evidence of a successful read of the installed skill contents. Add negative cases for listing or echoing the path and a positive case for the normal skill read.

Reply: Fixed by 931a45b. `codex.read_skill` counts the skill as loaded only when a successful command names the installed `SKILL.md` path, its program is a reader (`cat`, `head`, `sed`, `less`, …) and the output carries the `name: fuku` front matter line. `test_codex_skill_read` covers `ls`, `echo`, a `grep` of the name line, a failed `cat`, a `cat` of another file, a `cat` to null and a `sed` past the front matter as not loaded, and `cat`, a `sed` range, a numbered and cut read and `head` as loaded. All 21 recorded Codex runs still count as loaded: each read the file with `cat`.

Recheck: Still open because a successful `head -n 2 <installed-path>/SKILL.md` (or `sed -n '1,2p'`) prints the `name: fuku` front matter but none of the instructions, yet meets `read_skill`’s reader, path and output checks. The current tests do not cover this case, so AC8 can still pass without evidence that Codex read the skill instructions.

Reply: Fixed by 8ead327. `read_skill` now also requires the output to carry the last non-blank line of the working-tree `SKILL.md`, matched with the same optional line-number prefix as the name line, so a `head -n 2` or `sed -n '1,2p'` of the front matter no longer counts. The test table adds both as not-loaded rows and keeps `cat`, a full `sed` range, `nl -ba | head -n 400` and `head -n 400` as loaded; a mutant with an always-true end check fails the two new rows. The Codex sample now carries the full skill as a real `cat` prints it. All 21 recorded Codex runs of the sweep end with that line, so the replay still counts 21 of 21 as loaded. The README names the rule.

Recheck: Still open because the predicate accepts output containing the name and last lines without the intervening instructions. For example, `sed -n '2p;$p' <installed-path>/SKILL.md` is a recognized reader and satisfies both output checks while printing no instructions.

Reply: Fixed by db94c10. `read_skill` now requires the output to hold every non-blank line of the working-tree `SKILL.md`, compared as sets after the optional line-number prefix, indentation and trailing whitespace are stripped, so `cat`, `cat -n`, `nl -ba` and a full `sed` or `head` range pass while `sed -n '2p;$p'` and a read that skips one line do not. Both are not-loaded rows in the test table; a mutant with the content check always true fails exactly the six rows that rely on content. The replay over the 21 recorded Codex runs counts 0 against the current file, because 167b833 edited one skill line after the sweep was recorded, and 21 of 21 against the `SKILL.md` of that recording; a live run installs the working-tree skill, so the two always match there. The README names the rule.

Recheck: Resolved because `read_skill` now requires every distinct non-blank line from the working-tree skill in successful reader output naming the installed skill path. The tests reject both printing only the first and last lines and skipping an instruction line; the reported mutant fails all six content-dependent rows.

## Resolved

- CODE-1 – resolved: the SIGTERM handler raises `SystemExit(143)` and unwinds through run and adapter cleanup; the subprocess cleanup test checks removal of the auth copy, temp project and fixture processes.
- CODE-2 – resolved: both adapters pass raw host stdout to secret detection; intermediate-message regressions for both hosts check exposure, and the supplied replay reports no newly flagged runs.
- CODE-3 – resolved: service program and path patterns are checked for `pkill` and `killall`, piped PID kills are recognized, and the valid S5 restart regression fails as expected. The supplied 42-run replay flags none.
- CODE-5 – resolved: the Claude plugin guard fingerprints relative paths, file contents, links and directories; supplied tests cover metadata edits, cached-file edits and unreadable entries.
- CODE-6 – resolved: both adapters redact complete stderr with applicable secrets before truncating it; boundary regressions cover tokens and opted-in values, plus Codex credentials.

## Checked

- Reviewed commit `db94c10`’s focused changes in `codex.py`, the Codex skill-read tests and README.
- Confirmed `printed_lines` requires every distinct non-blank line from the working-tree `SKILL.md`, after stripping optional line-number prefixes, indentation and trailing whitespace; `read_skill` also requires a successful shell call, an installed skill path and a recognized reader.
- The new test rows reject the first-and-last-line read and a read skipping line 10. The primary-reported always-true content-check mutant fails exactly the six rows that rely on content.
- Primary-reported checks at `db94c10`: `make test:agents-plugin` passed 111 tests with 6 skips without `FUKU_BIN`; with `FUKU_BIN="$PWD/cmd/fuku"`, all 111 tests passed with no skips.
- The supplied offline replay reports 0 of 21 recorded runs against the current skill after its post-recording edit at `167b833`, and 21 of 21 against the skill version used for the recording. A live run installs the working-tree skill, keeping the installed file and checked file aligned.

## Verdict

PASS